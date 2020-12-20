package spjs

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
)

type SPJSData struct {
	Version     string
	Commands    []string
	Hostname    string
	SerialPorts []SerialPort

	P         string
	D         string
	Cmd       string
	ID        string
	ErrorCode string

	Port string
	QCnt int
}

func (c *Client) PortByName(name string) *Port {
	ports := <-c.ports
	c.ports <- ports
	for _, p := range ports {
		portName, _ := p.Name()
		if portName != name {
			continue
		}
		return p
	}

	return nil
}

func (c *Client) updatePorts(serialPorts []SerialPort) {
	<-c.serialPorts
	c.serialPorts <- serialPorts

	ports := <-c.ports
	c.ports <- ports

	// open matching ports
	for _, sp := range serialPorts {
		if sp.IsOpen {
			continue
		}
		for _, port := range ports {
			if !port.match(sp) {
				continue
			}
			err := port.open(sp.Name)
			if err != nil {
				log.Println("ERROR:", err)
			}
			break
		}
	}
}

func (c *Client) readLoop() {
	for dataStr := range c.dataCh {
		log.Println("READ:", dataStr)
		if !strings.HasPrefix(dataStr, "{") {
			continue
		}
		var data SPJSData
		err := json.Unmarshal([]byte(dataStr), &data)
		if err != nil {
			log.Printf("ERROR: parse SPJS payload (%s): %v", dataStr, err)
			continue
		}

		if data.SerialPorts != nil {
			c.updatePorts(data.SerialPorts)
			continue
		}

		if data.P != "" && data.Cmd == "" && data.D != "" {
			port := c.PortByName(data.P)
			if port == nil {
				continue
			}
			err := port.drv.HandleData(data.D)
			if err != nil {
				log.Printf(`ERROR: handle serial data "%s" (%s): %v`, data.D, port.drv.Name(), err)
			}
			continue
		}
		switch data.Cmd {
		case "Complete":
			cb := <-c.callbacks
			ch := cb[data.ID]
			delete(cb, data.ID)
			c.callbacks <- cb
			if ch != nil {
				ch <- nil
			}
		case "Error":
			cb := <-c.callbacks
			ch := cb[data.ID]
			delete(cb, data.ID)
			c.callbacks <- cb
			if ch != nil {
				ch <- errors.New(data.ErrorCode)
			}
		case "WipedQueue", "Close":
			cb := <-c.callbacks
			err := errors.New("RESET")
			for id, ch := range cb {
				if !strings.HasSuffix(id, ":"+data.Port) {
					continue
				}
				delete(cb, id)
				ch <- err
			}
			c.callbacks <- cb
		}
	}
}
