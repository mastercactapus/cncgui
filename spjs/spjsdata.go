package spjs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func (c *Client) readLoop(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for dataStr := range c.dataCh {
		if strings.Contains(dataStr, "SerialPorts") {
			log.Println("READ:", "...serial port data omitted...")
		} else {
			log.Println("READ:", dataStr)
		}
		if !strings.HasPrefix(dataStr, "{") || !strings.HasSuffix(dataStr, "}") {
			continue
		}
		var data SPJSData
		err := json.Unmarshal([]byte(dataStr), &data)
		if err != nil {
			log.Fatalf("ERROR: parse SPJS payload (%s): %v", dataStr, err)
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
			err := port.drv.HandleData(ctx, data.D)
			if err != nil {
				log.Printf(`ERROR: handle serial data "%s" (%s): %v`, data.D, port.drv.Name(), err)
			}
			continue
		}

		var baseID string
		cmdID := commandID{Port: data.P}
		if cmdID.Port == "" {
			cmdID.Port = data.Port
		}
		idStr := strings.ReplaceAll(data.ID, "-", " ")
		if idStr == "" {
			continue
		}
		_, err = fmt.Sscanf(idStr, "%s %d", &baseID, &cmdID.ID)
		if err != nil {
			log.Printf(`ERROR: unknown ID format "%s"`, data.ID)
			continue
		}
		if baseID != c.baseID {
			continue
		}

		switch data.Cmd {
		case "Open":
			io.WriteString(c, "list")
		case "Write":
			c.withOneCallback(cmdID, func(cb *commandCallback) bool {
				cb.written()
				return false
			})
		case "Complete":
			c.withOneCallback(cmdID, func(cb *commandCallback) bool {
				cb.finish(nil)

				return false
			})
		case "Error":
			c.withOneCallback(cmdID, func(cb *commandCallback) bool {
				cb.finish(errors.New(data.ErrorCode))
				return true
			})
		case "WipedQueue", "Close":
			err := errors.New("RESET")
			c.withCallbacks(func(m callbackMap) {
				for id, cb := range m {
					if id.Port != data.P {
						continue
					}
					cb.finish(err)
					delete(m, id)
				}
			})
		}
	}
}
