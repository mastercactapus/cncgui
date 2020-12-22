package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
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
	grbl := spjs.NewGRBL()
	cli := spjs.NewClient(*spjsURL)
	cli.RegisterDriver(spjs.NewVIDPIDMatcher("2a03", "0043"), grbl)
	pendant := spjs.NewArduinoPendant(grbl)
	cli.RegisterDriver(spjs.NewVIDPIDMatcher("1a86", "7523"), pendant)

	a := app.New()
	// a.Settings().SetTheme(paddedTheme{a.Settings().Theme()})

	i := 0
	i++

	var st spjs.ControllerStatus
	var refreshFns []func()

	go func() {
		for newState := range grbl.Status() {
			st = newState
			for _, fn := range refreshFns {
				fn()
			}
		}
	}()

	w := a.NewWindow("CNC GUI")
	if *full {
		w.SetFullScreen(true)
	}

	go func() {
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
		dialog.ShowConfirm("Home Machine?", "This will cause the machine to move to it's home position and lose it's work coordinates.", func(proceed bool) {
			if proceed {
				prog := dialog.NewProgressInfinite("Homing Machine", "The machine is now calibrating it's home position, please wait...", w)
				err := grbl.CommandHome(true)
				prog.Hide()
				if err != nil {
					dialog.ShowError(err, w)
				}
			}
		}, w)
	})
	load := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), nil)

	run := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
	pause := widget.NewButtonWithIcon("", theme.MediaPauseIcon(), nil)
	stop := widget.NewButtonWithIcon("", theme.MediaReplayIcon(), nil)

	actions := fyne.NewContainerWithLayout(layout.NewHBoxLayout(),
		home, load, run, pause, stop,
	)

	NewPos := func() *widget.Label {
		l := widget.NewLabel("     0.0000")
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
		set := func(l *widget.Label, v float64) { l.SetText(fmt.Sprintf("%11.4f", v)) }
		wpos := st.WorkPosition()
		mpos := st.MachinePosition()
		set(wPosX, wpos.X)
		set(wPosY, wpos.Y)
		set(wPosZ, wpos.Z)
		set(mPosX, mpos.X)
		set(mPosY, mpos.Y)
		set(mPosZ, mpos.Z)
	})

	posRead := fyne.NewContainerWithLayout(layout.NewGridLayout(4),
		widget.NewLabel("WPos"), wPosX, wPosY, wPosZ,

		widget.NewLabel("MPos"), mPosX, mPosY, mPosZ,

		widget.NewLabel(""),
		widget.NewButton("X=0", func() { grbl.SetWPos('X', 0) }),
		widget.NewButton("Y=0", func() { grbl.SetWPos('Y', 0) }),
		widget.NewButton("Z=0", func() { grbl.SetWPos('Z', 0) }),
	)

	centerLabel := func(text string) *widget.Label {
		label := widget.NewLabel(text)
		label.Alignment = fyne.TextAlignCenter
		return label
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
			err = grbl.CommandJog(axis, val, false)
			if err != nil {
				dialog.ShowError(err, w)
			}
		}
	}
	zUp.OnTapped = makeMove('Z', false)
	zDn.OnTapped = makeMove('Z', true)

	touchPendant := fyne.NewContainerWithLayout(layout.NewGridLayout(5),
		zUp, widget.NewLabel(""), widget.NewLabel(""), widget.NewButtonWithIcon("", theme.MoveUpIcon(), makeMove('Y', false)), widget.NewLabel(""),
		centerLabel("Z"), widget.NewLabel(""), widget.NewButtonWithIcon("<", nil, makeMove('X', true)), centerLabel("XY"), widget.NewButtonWithIcon(">", nil, makeMove('X', false)),
		zDn, widget.NewLabel(""), widget.NewLabel(""), widget.NewButtonWithIcon("", theme.MoveDownIcon(), makeMove('Y', true)), widget.NewLabel(""),
	)

	pos := widget.NewGroup("Position", fyne.NewContainerWithLayout(
		layout.NewHBoxLayout(),
		sel, touchPendant, posRead,
	))

	w.SetContent(fyne.NewContainerWithLayout(
		layout.NewVBoxLayout(), actions, pos,
	))

	fmt.Println("Launch")
	w.ShowAndRun()
}
