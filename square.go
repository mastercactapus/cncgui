package main

import (
	"fyne.io/fyne"
)

type squareHBox struct{ minSize int }

func NewSquareHBoxLayout(minSize int) fyne.Layout {
	return squareHBox{minSize: minSize}
}
func (b squareHBox) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var min int
	for _, obj := range objects {
		size := obj.MinSize()
		if size.Height > min {
			min = size.Height
		}
		if size.Width > min {
			min = size.Width
		}
	}
	if min < b.minSize {
		min = b.minSize
	}
	return fyne.NewSize(min*len(objects), min)
}
func (b squareHBox) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	cols := len(objects)
	rows := 1

	orig := size
	size.Width = size.Width / cols

	if size.Height < size.Width {
		size.Width = size.Height
	} else {
		size.Height = size.Width
	}
	yOffset := (orig.Height - size.Height*rows) / 2

	for i, obj := range objects {
		x := i % cols * size.Width
		y := i / cols * size.Height
		obj.Move(fyne.NewPos(x, yOffset+y))
		obj.Resize(size)
	}
}

type squareGrid struct {
	cols, minSize int
}

func NewSquareGridLayout(cols, minSize int) fyne.Layout {
	return squareGrid{cols: cols, minSize: minSize}
}

func (g squareGrid) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var min int
	for _, obj := range objects {
		size := obj.MinSize()
		if size.Height > min {
			min = size.Height
		}
		if size.Width > min {
			min = size.Width
		}
	}
	if min < g.minSize {
		min = g.minSize
	}
	return fyne.NewSize(min*g.cols, len(objects)/g.cols*min)
}

func (g squareGrid) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	cols := g.cols
	rows := len(objects) / cols

	orig := size
	size.Width = size.Width / cols
	size.Height = size.Height / rows

	if size.Height < size.Width {
		size.Width = size.Height
	} else {
		size.Height = size.Width
	}
	xOffset := (orig.Width - size.Width*cols) / 2
	yOffset := (orig.Height - size.Height*rows) / 2

	for i, obj := range objects {
		x := i % cols * size.Width
		y := i / cols * size.Height
		obj.Move(fyne.NewPos(xOffset+x, yOffset+y))
		obj.Resize(size)
	}
}
