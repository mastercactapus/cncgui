package spjs

import (
	"context"
	"fmt"
	"strings"
)

type GRBL struct {
	port *Port

	firstStatus bool
	statCh      chan GRBLStatus
	statExtCh   chan ControllerStatus
}

var _ Driver = &GRBL{}

func NewGRBL() *GRBL {
	return &GRBL{
		statCh:    make(chan GRBLStatus, 1),
		statExtCh: make(chan ControllerStatus),
	}
}

func (g *GRBL) WrapGCode(data []string) string { return strings.Join(data, "\n") + "\n" }

// SetPort will set the control port.
func (g *GRBL) SetPort(p *Port) { g.port = p }

// Name will always return the string `GRBL`.
func (g *GRBL) Name() string { return "GRBL" }

// BufferAlgorithm returns the string `grbl`.
func (g *GRBL) BufferAlgorithm() string { return "grbl" }

// BaudRate is always set to 115200.
func (g *GRBL) BaudRate() int { return 115200 }

func (g *GRBL) FeedHold() string   { return "!" }
func (g *GRBL) CycleStart() string { return "~" }
func (g *GRBL) Home() string       { return "$H\n" }
func (g *GRBL) EStop() string      { return "\x18" }
func (g *GRBL) Reset() string      { return "\x18" }
func (g *GRBL) Jog(axis rune, mm float64) string {
	return fmt.Sprintf("$J=G21G91F10000%c%0.4g\n", axis, mm)
}
func (g *GRBL) WPos(axis rune, mm float64) string { return fmt.Sprintf("G10L20P1%c%0.4g\n?", axis, mm) }

// LastStatus will return the last available status. It will block until the first status message is processed.
func (g *GRBL) LastStatus() ControllerStatus {
	stat := <-g.statCh
	g.statCh <- stat
	return stat
}

// Status will return a channel that will get updates each time status data is updated. It always returns the same channel.
func (g *GRBL) Status() <-chan ControllerStatus { return g.statExtCh }

// HandleData will process data coming from GRBL. It is only intended to be used by the SPJS client code.
func (g *GRBL) HandleData(ctx context.Context, data string) error {
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
