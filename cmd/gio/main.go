package main

import (
	"image"
	"image/color"
	"image/gif"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	l "gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	m "gioui.org/widget/material"
	c "gioui.org/x/component"
	"github.com/jackmordaunt/giffer"
)

func main() {
	go func() {
		ui := UI{
			Window: app.NewWindow(
				app.Title("Giffer"),
				app.MinSize(unit.Dp(800), unit.Dp(425)),
			),
			Th: material.NewTheme(gofont.Collection()),
			Giffer: Giffer{
				Store: &gifdb{
					Dir: filepath.Join(os.TempDir(), "giffer"),
				},
			},
		}
		if err := ui.Loop(); err != nil {
			log.Fatalf("error: %v", err)
		}
		os.Exit(0)
	}()
	app.Main()
}

type (
	C = layout.Context
	D = layout.Dimensions
)

type UI struct {
	*app.Window
	Giffer Giffer
	Th     *m.Theme
	Form   Form
	Video  Gif
}

// Loop runs the event loop until terminated.
func (ui *UI) Loop() error {
	var (
		ops    op.Ops
		events = ui.Window.Events()
	)
	for event := range events {
		switch event := (event).(type) {
		case system.DestroyEvent:
			return event.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, event)
			ui.Update(gtx)
			ui.Layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
	return nil
}

func (ui *UI) Update(gtx C) {
	if ui.Form.SubmitBtn.Clicked() {
		var (
			url     = ui.Form.URL.Text()
			fuzz    = 0
			quality = giffer.Low
		)
		start, err := strconv.ParseFloat(ui.Form.Start.Text(), 64)
		if err != nil {
			log.Printf("error: start must be a floating point number")
		}
		end, err := strconv.ParseFloat(ui.Form.End.Text(), 64)
		if err != nil {
			log.Printf("error: end must be a floating point number")
		}
		fps, err := strconv.ParseFloat(ui.Form.FPS.Text(), 64)
		if err != nil {
			log.Printf("error: fps must be a floating point number")
		}
		width, err := strconv.Atoi(ui.Form.Width.Text())
		if err != nil {
			log.Printf("error: width must be an integer number")
		}
		height, err := strconv.Atoi(ui.Form.Height.Text())
		if err != nil {
			log.Printf("error: height must be an integer number")
		}
		g, err := ui.Giffer.GififyURL(
			url,
			start,
			end,
			fps,
			width,
			height,
			fuzz,
			quality,
		)
		if err != nil {
			log.Printf("error: fetching gif: %v", err)
		}
		img, err := gif.DecodeAll(g)
		if err != nil {
			log.Printf("error: decoding gif: %v", err)
		}
		ui.Video.Src = img
	}
}

func (ui *UI) Layout(gtx C) {
	l.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx C) D {
		return l.Flex{
			Axis: l.Horizontal,
		}.Layout(
			gtx,
			l.Flexed(1, func(gtx C) D {
				return l.Center.Layout(gtx, func(gtx C) D {
					var (
						cs  = &gtx.Constraints
						max = gtx.Px(unit.Dp(400))
					)
					if cs.Max.X > max {
						cs.Max.X = max
					}
					return ui.Form.Layout(gtx, ui.Th)
				})
			}),
			l.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					var (
						cs     = &gtx.Constraints
						width  = gtx.Px(unit.Dp(350))
						height = gtx.Px(unit.Dp(250))
					)
					cs.Max.X /= 2
					cs.Max.Y /= 2
					if cs.Max.X > width {
						cs.Max.X = width
					}
					if cs.Max.Y > height {
						cs.Max.Y = height
					}
					return ui.Video.Layout(gtx)
				})
			}),
		)
	})
}

// Form holds state for form inputs.
type Form struct {
	URL       c.TextField
	Start     c.TextField
	End       c.TextField
	Quality   c.TextField
	Width     c.TextField
	Height    c.TextField
	FPS       c.TextField
	Loading   bool
	SubmitBtn widget.Clickable
}

func (f *Form) Layout(gtx C, th *m.Theme) D {
	return l.Stack{}.Layout(
		gtx,
		l.Stacked(func(gtx C) D {
			return l.Flex{
				Axis: l.Vertical,
			}.Layout(
				gtx,
				l.Rigid(func(gtx C) D {
					return f.URL.Layout(gtx, th, "url")
				}),
				l.Rigid(func(gtx C) D {
					return f.Start.Layout(gtx, th, "start")
				}),
				l.Rigid(func(gtx C) D {
					return f.End.Layout(gtx, th, "end")
				}),
				l.Rigid(func(gtx C) D {
					return f.Quality.Layout(gtx, th, "quality")
				}),
				l.Rigid(func(gtx C) D {
					return f.Width.Layout(gtx, th, "width")
				}),
				l.Rigid(func(gtx C) D {
					return f.Height.Layout(gtx, th, "height")
				}),
				l.Rigid(func(gtx C) D {
					return f.FPS.Layout(gtx, th, "fps")
				}),
				l.Rigid(func(gtx C) D {
					return D{Size: image.Point{Y: gtx.Px(unit.Dp(10))}}
				}),
				l.Rigid(func(gtx C) D {
					return m.Button(th, &f.SubmitBtn, "Create").Layout(gtx)
				}),
			)
		}),
		l.Expanded(func(gtx C) D {
			if !f.Loading {
				return D{}
			}
			return c.Rect{
				Color: color.NRGBA{A: 100},
				Size: image.Point{
					X: gtx.Constraints.Max.X,
					Y: gtx.Constraints.Max.Y,
				},
			}.Layout(gtx)
		}),
	)
}

// Gif animates through a series of frames.
type Gif struct {
	Src    *gif.GIF
	Cursor int

	since time.Time
	img   widget.Image
}

// Ready if the next frame is ready to be displayed.
func (g *Gif) Ready(gtx C) bool {
	var (
		now     = gtx.Now
		since   = g.since
		latency = time.Duration(g.Src.Delay[g.Cursor]) * time.Second / 100
	)
	return now.Sub(since).Milliseconds() >= latency.Milliseconds()
}

// Next returns the next frame in the series.
func (g *Gif) Next(gtx C) {
	defer func() {
		g.Cursor++
		if g.Cursor > len(g.Src.Image)-1 {
			g.Cursor = 0
		}
		g.since = gtx.Now
	}()
	op.InvalidateOp{
		At: gtx.Now.Add(time.Duration(g.Src.Delay[g.Cursor]) * time.Second / 100),
	}.Add(
		gtx.Ops,
	)
	g.img.Src = paint.NewImageOp(g.Src.Image[g.Cursor])
}

// Current returns the current frame.
func (g *Gif) Current() *image.Paletted {
	return g.Src.Image[g.Cursor]
}

func (g *Gif) Layout(gtx C) D {
	if g.Src == nil || len(g.Src.Image) == 0 {
		return c.Rect{
			Size: image.Point{
				X: gtx.Constraints.Max.X,
				Y: gtx.Constraints.Max.Y,
			},
			Color: color.NRGBA{A: 100},
		}.Layout(gtx)
	}
	if g.Ready(gtx) {
		g.Next(gtx)
	}
	return g.img.Layout(gtx)
}
