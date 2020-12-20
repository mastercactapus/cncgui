package spjs

import (
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
}

func (p *Port) SendCommand(command string, wait bool) error {
	ch, err := p.sendCommand(command)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	return <-ch
}

func (p *Port) sendCommand(command string) (done <-chan error, err error) {
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

	id := fmt.Sprintf("%d:%s", atomic.AddUint32(&p.cli.id, 1), portName)
	data, err := json.Marshal(SendJSON{
		Port: portName,
		Data: []SendJSONData{{
			ID:   id,
			Data: command,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}

	cb := <-p.cli.callbacks
	ch := make(chan error, 1)
	cb[id] = ch
	p.cli.callbacks <- cb

	_, err = io.WriteString(p.cli, "sendjson "+string(data))
	if err != nil {
		return nil, fmt.Errorf("write to SPJS: %w", err)
	}

	return ch, nil
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
