package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/widget"

	"github.com/mastercactapus/cncgui/spjs"
)

type paddedTheme struct {
	fyne.Theme
}

func (p paddedTheme) Padding() int { return 8 }

func main() {
	spjsURL := flag.String("spjs", "ws://localhost:8989/ws", "Set the SPJS connection URL.")
	full := flag.Bool("fullscreen", false, "Run in fullscreen.")
	flag.Parse()
	log.SetFlags(log.Lshortfile)

	log.Println("START")
	cli := spjs.NewClient(*spjsURL)
	grbl := cli.NewPort(spjs.NewVIDPIDMatcher("2a03", "0043"), spjs.NewGRBL()).NewController()
	pendant := spjs.NewArduinoPendant(grbl)
	cli.NewPort(spjs.NewVIDPIDMatcher("1a86", "7523"), pendant)

	a := app.New()
	ctx := context.Background()

	var st spjs.ControllerStatus
	var jobSt spjs.JobStatus
	var refreshFns []func()

	go func() {
		for {
			select {
			case st = <-grbl.Status():
			case jobSt = <-grbl.JobStatus():
			}
			if st == nil {
				return
			}
			for _, fn := range refreshFns {
				fn()
			}
		}
	}()

	w := a.NewWindow("CNC GUI")
	w.Resize(fyne.NewSize(800, 480))
	w.SetFixedSize(true)
	if *full {
		w.SetFullScreen(true)
	}
	readyCh := make(chan struct{})
	go func() {
		<-readyCh

		var grblWait *dialog.ProgressInfiniteDialog
		t := time.NewTicker(100 * time.Millisecond)
		for range t.C {
			if !grbl.Connected() && grblWait == nil {
				grblWait = dialog.NewProgressInfinite("Connecting to GRBL", "The CNC controller board (GRBL) is not connected...", w)
			} else if grbl.Connected() && grblWait != nil {
				grblWait.Hide()
				grblWait = nil
			}
		}
	}()

	home := widget.NewButtonWithIcon("", theme.HomeIcon(), func() {
		go dialog.ShowConfirm("Home Machine?", "This will cause the machine to move to it's home position and lose it's work coordinates.", func(proceed bool) {
			if proceed {
				prog := dialog.NewProgressInfinite("Homing Machine", "The machine is now calibrating it's home position, please wait...", w)
				go func() {
					err := grbl.CommandHome(ctx, true)
					prog.Hide()
					if err != nil {
						go dialog.ShowError(err, w)
					}
				}()
			}
		}, w)
	})
	load := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		lister, err := storage.ListerForURI(storage.NewURI("file:///home/nathan/cnc/cncgui"))
		if err != nil {
			log.Println("ERROR:", err)
			return
		}

		open := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil {
				log.Println("ERROR:", err)
				return
			}
			if rc == nil {
				return
			}
			err = grbl.SetJob(rc.Name(), rc)
			if err != nil {
				rc.Close()
				dialog.ShowError(err, w)
				return
			}
		}, w)

		open.SetLocation(lister)
		open.SetFilter(storage.NewExtensionFileFilter([]string{".nc"}))
		open.Show()
	})

	runJob := widget.NewButtonWithIcon("", theme.ContentRedoIcon(), func() {
		err := grbl.StartJob(ctx)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})
	runJob.Disable()
	refreshFns = append(refreshFns, func() {
		if jobSt.Valid && !jobSt.Active {
			runJob.Enable()
		} else {
			runJob.Disable()
		}
	})
	cycleStart := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		err := grbl.CommandCycleStart(ctx)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})
	feedHold := widget.NewButtonWithIcon("", theme.MediaPauseIcon(), func() {
		err := grbl.CommandFeedHold(ctx)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})
	resetCancel := widget.NewButtonWithIcon("", theme.MediaReplayIcon(), func() {
		err := grbl.CommandReset(ctx)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})

	status := widget.NewLabel("GRBL Status: ...")
	pendStatus := widget.NewLabel("Pendant: Not Connected")

	refreshFns = append(refreshFns, func() {
		status.SetText("GRBL Status: " + st.StatusText())
		pend := "Connected"
		if !pendant.Connected() {
			pend = "Not Connected"
		}
		pendStatus.SetText("Pendant: " + pend)
	})

	actions := fyne.NewContainerWithLayout(layout.NewHBoxLayout(),
		fyne.NewContainerWithLayout(NewSquareHBoxLayout(64),
			home, load, runJob, cycleStart, feedHold, resetCancel,
		),
		fyne.NewContainerWithLayout(layout.NewVBoxLayout(), status, pendStatus),
	)

	NewPos := func() *widget.Label {
		l := widget.NewLabel("     0.000")
		l.Alignment = fyne.TextAlignTrailing
		l.TextStyle.Monospace = true
		return l
	}

	wPosX := NewPos()
	wPosY := NewPos()
	wPosZ := NewPos()
	mPosX := NewPos()
	mPosY := NewPos()
	mPosZ := NewPos()
	refreshFns = append(refreshFns, func() {
		set := func(l *widget.Label, v float64) { l.SetText(fmt.Sprintf("%10.3f", v)) }
		wpos := st.WorkPosition()
		mpos := st.MachinePosition()
		set(wPosX, wpos.X)
		set(wPosY, wpos.Y)
		set(wPosZ, wpos.Z)
		set(mPosX, mpos.X)
		set(mPosY, mpos.Y)
		set(mPosZ, mpos.Z)
	})

	wpos := widget.NewLabel("WPos")
	wpos.Alignment = fyne.TextAlignTrailing
	mpos := widget.NewLabel("MPos")
	mpos.Alignment = fyne.TextAlignTrailing

	posRead := fyne.NewContainerWithLayout(layout.NewGridLayout(4),
		wpos, wPosX, wPosY, wPosZ,

		mpos, mPosX, mPosY, mPosZ,

		widget.NewLabel(""),
		widget.NewButton("X=0", func() { grbl.SetWPos(ctx, 'X', 0) }),
		widget.NewButton("Y=0", func() { grbl.SetWPos(ctx, 'Y', 0) }),
		widget.NewButton("Z=0", func() { grbl.SetWPos(ctx, 'Z', 0) }),
	)

	centerLabel := func(text string) fyne.CanvasObject {
		label := widget.NewLabel(text)
		label.Alignment = fyne.TextAlignCenter
		return widget.NewVBox(layout.NewSpacer(), label, layout.NewSpacer())
	}

	zUp := widget.NewButtonWithIcon("", theme.MoveUpIcon(), nil)
	zDn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), nil)

	mult := "10"
	sel := widget.NewRadioGroup([]string{
		"100", "10", "1", "0.1", "0.01", "0.001",
	}, nil)
	sel.OnChanged = func(val string) {
		if val == "" {
			val = mult
			sel.SetSelected(mult)
			return
		}
		mult = val
		if val == "100" {
			zUp.Disable()
			zDn.Disable()
		} else {
			zUp.Enable()
			zDn.Enable()
		}
	}
	sel.SetSelected("10")

	makeMove := func(axis rune, invert bool) func() {
		return func() {
			val, err := strconv.ParseFloat(mult, 64)
			if err != nil {
				panic(err)
			}
			if invert {
				val = -val
			}
			err = grbl.CommandJog(ctx, axis, val, false)
			if err != nil {
				dialog.ShowError(err, w)
			}
		}
	}
	zUp.OnTapped = makeMove('Z', false)
	zDn.OnTapped = makeMove('Z', true)

	touchPendant := fyne.NewContainerWithLayout(NewSquareGridLayout(5, 64),
		zUp, layout.NewSpacer(), layout.NewSpacer(), widget.NewButtonWithIcon("", theme.MoveUpIcon(), makeMove('Y', false)), layout.NewSpacer(),
		centerLabel("Z"), layout.NewSpacer(), widget.NewButtonWithIcon("<", nil, makeMove('X', true)), centerLabel("XY"), widget.NewButtonWithIcon(">", nil, makeMove('X', false)),
		zDn, layout.NewSpacer(), layout.NewSpacer(), widget.NewButtonWithIcon("", theme.MoveDownIcon(), makeMove('Y', true)), layout.NewSpacer(),
	)

	pos := fyne.NewContainerWithLayout(
		layout.NewHBoxLayout(),
		sel, touchPendant, layout.NewSpacer(), posRead,
	)

	jobStatus := widget.NewLabel("No active job.")
	jobProgress := widget.NewProgressBar()
	jobProgress.TextFormatter = func() string {
		if !jobSt.Active {
			return "No job active."
		}
		var pct float64
		if jobSt.Read > 0 {
			pct = float64(jobSt.Completed) / float64(jobSt.Read)
		}
		if jobSt.ReadComplete {
			return fmt.Sprintf("%.f%% (%d of %d)", pct*100, jobSt.Completed, jobSt.Read)
		}

		return fmt.Sprintf("%.f%% (%d of %d+)", pct*100, jobSt.Completed, jobSt.Read)
	}
	refreshFns = append(refreshFns, func() {
		if !jobSt.Valid {
			jobStatus.SetText("No active job.")
			jobProgress.SetValue(0)
			return
		}

		msg := fmt.Sprintf("Job: %s", jobSt.Name)
		if jobSt.Err != nil {
			msg += " (error: " + jobSt.Err.Error() + ")"
		} else if !jobSt.Active {
			msg += " (paused)"
		}
		jobStatus.SetText(msg)

		if jobSt.Read > 0 {
			jobProgress.SetValue(float64(jobSt.Completed) / float64(jobSt.Read))
		}
	})

	grp := widget.NewGroup("Job",
		fyne.NewContainerWithLayout(layout.NewHBoxLayout(), jobStatus),
		jobProgress,
	)
	w.SetContent(fyne.NewContainerWithLayout(
		layout.NewVBoxLayout(), actions, pos,
		layout.NewSpacer(),
		grp,
	))

	fmt.Println("Launch")
	w.Show()
	close(readyCh)
	a.Run()
}
