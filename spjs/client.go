package spjs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	baseID string

	id uint32

	url string
	ws  *websocket.Conn
	mx  sync.Mutex

	ports chan []*Port

	serialPorts chan []SerialPort
	dataCh      chan string

	callbacks chan callbackMap
}
type callbackMap map[commandID]*commandCallback

type commandID struct {
	Port string
	ID   uint32
}
type commandCallback struct {
	Err     error
	WriteCh chan struct{}
	DoneCh  chan struct{}
	once    sync.Once
}

func (cb *commandCallback) written() {
	if cb == nil {
		return
	}
	close(cb.WriteCh)
}
func (cb *commandCallback) finish(err error) {
	if cb == nil {
		return
	}
	cb.Err = err
	cb.once.Do(func() {
		if cb.Err == nil {
			cb.Err = err
		}
		close(cb.DoneCh)
	})
}

type SerialPortMatcher func(SerialPort) bool

type SendJSON struct {
	Port string `json:"P"`
	Data []SendJSONData
}
type SendJSONData struct {
	Data string `json:"D"`
	ID   string `json:"Id"`
}
type SPJSCmd struct {
	Port string `json:"P"`
	Data []SPJSCmdData
}
type SPJSCmdData struct {
	Data string `json:"D"`
	ID   string `json:"Id"`
	Buf  bool   `json:"Buf,omitempty"`
}

func NewClient(url string) *Client {
	buf := make([]byte, 8)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		panic(err)
	}

	cli := &Client{
		baseID:      base64.StdEncoding.EncodeToString(buf),
		url:         url,
		serialPorts: make(chan []SerialPort, 1),
		callbacks:   make(chan callbackMap, 1),
		dataCh:      make(chan string),
		ports:       make(chan []*Port, 1),
	}

	cli.serialPorts <- nil
	cli.ports <- nil
	cli.callbacks <- make(callbackMap)

	// update port list
	go func() {
		for range time.NewTicker(10 * time.Second).C {
			io.WriteString(cli, "list")
		}
	}()

	go func() {
		for range time.NewTicker(time.Second).C {
			cli.Check()
		}
	}()

	// process messages
	go cli.readLoop(context.TODO())

	return cli
}

func (c *Client) NewPort(match SerialPortMatcher, drv Driver) *Port {
	p := &Port{match: match, cli: c, drv: drv, sendCh: make(chan *sendReq, 1000)}
	go p.sendLoop()
	c.ports <- append(<-c.ports, p)
	io.WriteString(c, "list")
	log.Println("Registered new driver", drv.Name())
	return p
}
func (c *Client) withOneCallback(id commandID, handle func(*commandCallback) bool) {
	c.withCallbacks(func(m callbackMap) {
		cb := m[id]
		if cb == nil {
			return
		}
		if handle(cb) {
			delete(m, id)
		}
	})
}
func (c *Client) withCallbacks(update func(callbackMap)) {
	cbMap := <-c.callbacks
	update(cbMap)
	c.callbacks <- cbMap
}

func (c *Client) reconnect() error {
	log.Println("Connecting to:", c.url)
	cleanup := func() {
		<-c.serialPorts
		c.serialPorts <- nil
		c.ws.Close()
		c.ws = nil

		err := errors.New("NETWORK ERROR")
		c.withCallbacks(func(m callbackMap) {
			for id, cb := range m {
				cb.finish(err)
				delete(m, id)
			}
		})
	}
	if c.ws != nil {
		cleanup()
	}

	var err error

	ws, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("dial SPJS: %w", err)
	}

	err = ws.WriteMessage(websocket.TextMessage, []byte("list"))
	if err != nil {
		ws.Close()
		return fmt.Errorf("write SPJS (list): %w", err)
	}

	c.ws = ws

	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				log.Printf("ERROR: read SPJS: %v", err)
				break
			}

			c.dataCh <- string(data)
		}

		c.mx.Lock()
		if c.ws != nil {
			cleanup()
		}
		c.mx.Unlock()
	}()

	return nil
}

func (c *Client) Check() error {
	c.mx.Lock()
	defer c.mx.Unlock()

	var err error
	if c.ws == nil {
		err = c.reconnect()
		if err != nil {
			return err
		}
	}

	return nil
}

// Write will write to the active ws stream, reconnecting on error.
func (c *Client) Write(p []byte) (int, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	var err error
	if c.ws == nil {
		err = c.reconnect()
		if err != nil {
			return 0, err
		}
	}

	// log.Println("WRITE:", string(p))
	err = c.ws.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		log.Println("ERROR: write SPJS (will reconnect): %w", err)
		err = c.reconnect()
		if err != nil {
			return 0, err
		}
		return c.Write(p)
	}

	return len(p), nil
}
