package spjs

import (
	"fmt"
	"strings"
)

type GRBL struct {
	port *Port

	firstStatus bool
	statCh      chan GRBLStatus
	statExtCh   chan ControllerStatus
}

var _ Controller = &GRBL{}
var _ Driver = &GRBL{}

func NewGRBL() *GRBL {
	return &GRBL{
		statCh:    make(chan GRBLStatus, 1),
		statExtCh: make(chan ControllerStatus),
	}
}

// Connected returns true if the serial port is available and open.
func (g *GRBL) Connected() bool {
	_, isOpen := g.port.Name()
	return isOpen
}

// SetPort will set the control port.
func (g *GRBL) SetPort(p *Port) { g.port = p }

// Name will always return the string `GRBL`.
func (g *GRBL) Name() string { return "GRBL" }

// BufferAlgorithm returns the string `grbl`.
func (g *GRBL) BufferAlgorithm() string { return "grbl" }

// BaudRate is always set to 115200.
func (g *GRBL) BaudRate() int { return 115200 }

// CommandHome runs the `$H` GRBL command and optionally waits for it to complete.
func (g *GRBL) CommandHome(wait bool) error { return g.port.SendCommand("$H\n", wait) }

// CommandEStop issues a soft-reset of the controller.
func (g *GRBL) CommandEStop() error { return g.port.SendCommand("\x18", false) }

// CommandJog issues a jog command and waits for it to finish.
func (g *GRBL) CommandJog(axis rune, mm float64, wait bool) error {
	return g.port.SendCommand(fmt.Sprintf("$J=G21G91F10000%c%0.4g\n", axis, mm), wait)
}

// SetWPos will set the work coordinate to the proveded value.
func (g *GRBL) SetWPos(axis rune, mm float64) error {
	return g.port.SendCommand(fmt.Sprintf("G10L20P1%c%0.4g\n", axis, mm), true)
}

// LastStatus will return the last available status. It will block until the first status message is processed.
func (g *GRBL) LastStatus() ControllerStatus {
	stat := <-g.statCh
	g.statCh <- stat
	return stat
}

// Status will return a channel that will get updates each time status data is updated. It always returns the same channel.
func (g *GRBL) Status() <-chan ControllerStatus { return g.statExtCh }

// HandleData will process data coming from GRBL. It is only intended to be used by the SPJS client code.
func (g *GRBL) HandleData(data string) error {
	if !strings.HasPrefix(data, "<") {
		return nil
	}

	var stat GRBLStatus
	if g.firstStatus {
		stat = <-g.statCh
	}

	newStat := stat
	err := newStat.Parse(data)
	if err != nil {
		if g.firstStatus {
			g.statCh <- stat
		}
		return err
	}

	g.firstStatus = true
	g.statCh <- newStat

	select {
	case g.statExtCh <- newStat:
	default:
	}

	return nil
}
