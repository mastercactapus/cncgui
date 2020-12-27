package spjs

import (
	"context"
	"errors"
	"io"
	"sync"
)

type jobAction int

const (
	jobActionUnknown jobAction = iota
	jobActionStart
	jobActionPause
	jobActionCancel
)

type Controller struct {
	*Port

	mx        sync.Mutex
	job       *jobController
	jobStatus chan JobStatus

	wrapGCode func([]string) string
}

func (p *Port) NewController() *Controller {
	w, ok := p.drv.(GCodeWrapper)
	c := &Controller{Port: p, jobStatus: make(chan JobStatus, 1)}
	if ok {
		c.wrapGCode = w.WrapGCode
	}
	return c
}

func (c *Controller) SetJob(name string, r io.Reader) error {
	if c.wrapGCode == nil {
		return ErrUnsupportedByDriver
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	if c.job != nil {
		c.job.Close()
	}

	c.job = newJobController(context.Background(), c, name, r)

	return nil
}
func (c *Controller) JobStatus() <-chan JobStatus { return c.jobStatus }

func (c *Controller) Status() <-chan ControllerStatus {
	s, ok := c.drv.(Statusable)
	if !ok {
		return nil
	}

	return s.Status()
}

func (c *Controller) StartJob(ctx context.Context) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.job == nil {
		return errors.New("no loaded job")
	}

	return c.job.Start()
}

func (c *Controller) CommandCycleStart(ctx context.Context) error {
	s, ok := c.drv.(CycleStarter)
	if !ok {
		return ErrUnsupportedByDriver
	}

	return c.SendCommand(ctx, s.CycleStart(), false)
}
func (c *Controller) CommandReset(ctx context.Context) error {
	f, ok := c.drv.(Resetter)
	if !ok {
		return ErrUnsupportedByDriver
	}

	c.mx.Lock()
	defer c.mx.Unlock()
	if c.job != nil {
		c.job.Close()
		c.job = nil
		select {
		case c.jobStatus <- JobStatus{}:
		default:
		}
	}

	return c.SendCommand(ctx, f.Reset(), false)
}
func (c *Controller) CommandFeedHold(ctx context.Context) error {
	f, ok := c.drv.(FeedHolder)
	if !ok {
		return ErrUnsupportedByDriver
	}

	return c.SendCommand(ctx, f.FeedHold(), false)
}

func (c *Controller) CommandHome(ctx context.Context, wait bool) error {
	h, ok := c.drv.(Homeable)
	if !ok {
		return ErrUnsupportedByDriver
	}
	return c.SendCommand(ctx, h.Home(), wait)
}

func (c *Controller) CommandEStop(ctx context.Context) error {
	s, ok := c.drv.(EStopable)
	if !ok {
		return ErrUnsupportedByDriver
	}
	return c.SendCommand(ctx, s.EStop(), false)
}

// CommandJog issues a jog command and waits for it to finish.
func (c *Controller) CommandJog(ctx context.Context, axis rune, mm float64, wait bool) error {
	j, ok := c.drv.(Joggable)
	if !ok {
		return ErrUnsupportedByDriver
	}
	return c.SendCommand(ctx, j.Jog(axis, mm), wait)
}

// SetWPos will set the work coordinate to the proveded value.
func (c *Controller) SetWPos(ctx context.Context, axis rune, mm float64) error {
	w, ok := c.drv.(WPosable)
	if !ok {
		return ErrUnsupportedByDriver
	}
	return c.SendCommand(ctx, w.WPos(axis, mm), true)
}

type ControllerStatus interface {
	MachinePosition() Position
	WorkPosition() Position

	StatusText() string

	IsReady() bool
	IsAlarm() bool
}

type Position struct{ X, Y, Z float64 }
