package spjs

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type Client struct {
	id uint32

	url string
	ws  *websocket.Conn
	mx  sync.Mutex

	ports chan []*Port

	serialPorts chan []SerialPort
	dataCh      chan string

	callbacks chan map[string]chan error
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
	cli := &Client{
		url:         url,
		serialPorts: make(chan []SerialPort, 1),
		callbacks:   make(chan map[string]chan error, 1),
		dataCh:      make(chan string),
		ports:       make(chan []*Port, 1),
	}

	cli.serialPorts <- nil
	cli.ports <- nil
	cli.callbacks <- make(map[string]chan error)

	// update port list
	go func() {
		for range time.NewTicker(10 * time.Second).C {
			io.WriteString(cli, "list")
		}
	}()

	// process messages
	go cli.readLoop()

	return cli
}

func (c *Client) RegisterDriver(match SerialPortMatcher, drv Driver) {
	p := &Port{match: match, cli: c, drv: drv}
	drv.SetPort(p)
	c.ports <- append(<-c.ports, p)
	io.WriteString(c, "list")
	log.Println("Registered new driver", drv.Name())
}

func (c *Client) reconnect() error {
	log.Println("Connecting to:", c.url)
	cleanup := func() {
		<-c.serialPorts
		c.serialPorts <- nil
		c.ws.Close()
		c.ws = nil
		cb := <-c.callbacks
		err := errors.New("NETWORK ERROR")
		for id, ch := range cb {
			ch <- err
			delete(cb, id)
		}
		c.callbacks <- cb
	}
	if c.ws != nil {
		cleanup()
	}

	var err error
	ws, err := websocket.Dial(c.url, "ws", "http://localhost")
	if err != nil {
		return fmt.Errorf("dial SPJS: %w", err)
	}

	_, err = io.WriteString(ws, "list")
	if err != nil {
		ws.Close()
		return fmt.Errorf("write SPJS (list): %w", err)
	}

	c.ws = ws

	go func() {
		buf := make([]byte, 65536)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				log.Printf("ERROR: read SPJS: %v", err)
				break
			}

			c.dataCh <- string(buf[:n])
		}

		c.mx.Lock()
		if c.ws != nil {
			cleanup()
		}
		c.mx.Unlock()
	}()

	return nil
}

// Write will write to the active ws stream, reconnecting on error.
func (c *Client) Write(p []byte) (int, error) {
	c.mx.Lock()
	defer c.mx.Unlock()

	var err error
	if c.ws == nil {
		err := c.reconnect()
		if err != nil {
			return 0, err
		}
	}

	log.Println("WRITE:", string(p))
	n, err := c.ws.Write(p)
	if err != nil {
		log.Println("ERROR: write SPJS (will reconnect): %w", err)
		err = c.reconnect()
		if err != nil {
			return 0, err
		}
		return c.Write(p)
	}

	return n, nil
}
