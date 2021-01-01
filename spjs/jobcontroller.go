package spjs

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

const (

	// spjsJobLines is the min number of commands to keep in the SPJS queue at a time.
	spjsJobLines = 100

	// spjsJobLineBatch is the number of commands to send of a job to SPJS at a time.
	spjsJobLineBatch = 45

	// spjsLoadJobChunks is the max number of lines of a job to load at a time.
	spjsJobLinesBuffer = 100000
)

type jobController struct {
	*Controller

	ctx      context.Context
	cancelFn func()

	lines chan string

	statusCh chan JobStatus

	wg sync.WaitGroup
}

func newJobController(ctx context.Context, ctrl *Controller, name string, r io.Reader) *jobController {
	jc := &jobController{
		Controller: ctrl,
		statusCh:   make(chan JobStatus, 1),
		lines:      make(chan string, spjsJobLinesBuffer),
	}
	jc.statusCh <- JobStatus{Valid: true, Name: name}
	jc.ctx, jc.cancelFn = context.WithCancel(ctx)

	jc.wg.Add(1)
	closer, _ := r.(io.Closer)
	go jc.readLoop(bufio.NewScanner(r), closer)

	return jc
}

func (jc *jobController) updateStatus(update func(s *JobStatus)) JobStatus {
	stat := <-jc.statusCh
	if stat.Err == nil {
		update(&stat)
		select {
		case jc.Controller.jobStatus <- stat:
		default:
		}
	}

	jc.statusCh <- stat
	return stat
}
func (jc *jobController) failWith(err error) {
	jc.updateStatus(func(s *JobStatus) { s.Err = err })
	jc.cancelFn()
}

func (jc *jobController) readLoop(scan *bufio.Scanner, c io.Closer) {
	defer jc.wg.Done()
	defer close(jc.lines)
	if c != nil {
		defer c.Close()
	}

	for scan.Scan() {
		text := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(text, ";") || text == "" {
			continue
		}

		jc.updateStatus(func(s *JobStatus) { s.Read++ })
		select {
		case jc.lines <- scan.Text():
		case <-jc.ctx.Done():
			return
		}
	}

	jc.updateStatus(func(s *JobStatus) { s.ReadComplete = true })

	if scan.Err() != nil {
		jc.failWith(scan.Err())
	}
}

func (jc *jobController) Start() error {
	var wasStarted bool
	stat := jc.updateStatus(func(s *JobStatus) {
		wasStarted = s.Active
		s.Active = true
	})
	if stat.Err != nil {
		return stat.Err
	}
	if wasStarted {
		return errors.New("already started")
	}
	jc.wg.Add(2)

	ch := make(chan *commandCallback, spjsJobLines)

	// send commands
	go func() {
		defer jc.wg.Done()
		defer close(ch)
		defer jc.flush()

		var line string
		var ok bool
		for {
			select {
			case line, ok = <-jc.lines:
			case <-jc.ctx.Done():
				return
			}
			if !ok {
				// done sending
				return
			}

			cb, err := jc.sendCommand(jc.wrapGCode([]string{line}), spjsJobLineBatch)
			if err != nil {
				jc.failWith(err)
				// abort on failure
				return
			}

			select {
			case <-jc.ctx.Done():
				return
			case ch <- cb:
			}
		}
	}()

	// process responses
	go func() {
		defer jc.wg.Done()

		var cb *commandCallback
		var ok bool
		for {
			select {
			case cb, ok = <-ch:
			case <-jc.ctx.Done():
				return
			}
			if !ok {
				return
			}

			select {
			case <-cb.WriteCh:
				jc.updateStatus(func(s *JobStatus) { s.Sent++ })
			case <-cb.DoneCh:
				if cb.Err != nil {
					jc.failWith(cb.Err)
					return
				}
				jc.updateStatus(func(s *JobStatus) {
					s.Sent++
					s.Completed++
				})
				continue
			case <-jc.ctx.Done():
				return
			}

			select {
			case <-cb.DoneCh:
				if cb.Err != nil {
					jc.failWith(cb.Err)
					return
				}
				jc.updateStatus(func(s *JobStatus) {
					s.Sent++
					s.Completed++
				})
			case <-jc.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (jc *jobController) Err() error {
	stat := <-jc.statusCh
	jc.statusCh <- stat
	return stat.Err
}
func (jc *jobController) Close() error {
	jc.failWith(nil)
	jc.wg.Wait()
	return nil
}
