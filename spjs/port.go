package spjs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
)

type Port struct {
	cli   *Client
	match func(SerialPort) bool
	drv   Driver

	sendCh chan *sendReq
}

// Connected returns true if the serial port is available and open.
func (p *Port) Connected() bool {
	_, isOpen := p.Name()
	return isOpen
}

func (p *Port) SendCommand(ctx context.Context, command string, wait bool) error {
	cb, err := p.sendCommand(command)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	<-cb.DoneCh
	return cb.Err
}

func (id commandID) Format(baseID string) string { return fmt.Sprintf("%s-%d", baseID, id.ID) }

type sendReq struct {
	commandID
	data  string
	batch int

	cb *commandCallback
}

func (p *Port) sendLoop() {
	for req := range p.sendCh {
		data, err := json.Marshal(SendJSON{
			Port: req.Port,
			Data: []SendJSONData{{ID: req.Format(p.cli.baseID), Data: req.data}},
		})
		if err != nil {
			panic(err)
		}
		_, err = io.WriteString(p.cli, "sendjson "+string(data))
		if err != nil {
			req.cb.finish(err)
			continue
		}
	}
}
func (p *Port) sendJSON(id commandID, serialData string) *commandCallback {
	cb := &commandCallback{
		DoneCh: make(chan struct{}), WriteCh: make(chan struct{}),
	}
	p.cli.withCallbacks(func(m callbackMap) {
		m[id] = cb
	})

	req := &sendReq{commandID: id, data: serialData, cb: cb}
	p.sendCh <- req
	return cb
}
func (p *Port) sendCommand(command string) (*commandCallback, error) {
	portName, isOpen := p.Name()
	if portName == "" {
		return nil, errors.New("port not available")
	}

	if !isOpen {
		err := p.open(portName)
		if err != nil {
			return nil, err
		}
	}
	id := commandID{Port: portName, ID: atomic.AddUint32(&p.cli.id, 1)}

	return p.sendJSON(id, command), nil
}

func (p *Port) open(name string) error {
	_, err := fmt.Fprintf(p.cli, "open %s %d %s", name, p.drv.BaudRate(), p.drv.BufferAlgorithm())
	if err != nil {
		return fmt.Errorf("open %s (%s): %w", name, p.drv.Name(), err)
	}

	return nil
}

func (p *Port) Name() (string, bool) {
	ports := <-p.cli.serialPorts
	p.cli.serialPorts <- ports

	for _, port := range ports {
		if !p.match(port) {
			continue
		}
		return port.Name, port.IsOpen
	}

	return "", false
}
