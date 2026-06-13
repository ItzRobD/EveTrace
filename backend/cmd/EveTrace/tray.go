//go:build tray && !linux

package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"

	"github.com/getlantern/systray"
)

func runTray(addr string, cancel context.CancelFunc) {
	systray.Run(func() {
		systray.SetIcon(buildIcon())
		systray.SetTitle("EveTrace")
		systray.SetTooltip("EveTrace — EVE Online tracker")

		mOpen := systray.AddMenuItem("Open EveTrace", "Open dashboard in browser")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop EveTrace")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openBrowser("http://" + addr)
				case <-mQuit.ClickedCh:
					cancel()
					systray.Quit()
					return
				}
			}
		}()
	}, nil)
}

// buildIcon generates a simple 32×32 EVE-themed tray icon at runtime.
func buildIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	bg := color.RGBA{R: 10, G: 10, B: 20, A: 255}
	accent := color.RGBA{R: 100, G: 180, B: 255, A: 255}

	// Fill background.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}

	// Draw a simple diamond/rhombus in the accent colour.
	cx, cy := size/2, size/2
	half := size/2 - 3
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := x - cx
			dy := y - cy
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if dx+dy <= half {
				img.Set(x, y, accent)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
