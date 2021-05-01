package main

import (
	"bytes"
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
	"github.com/ncruces/zenity"
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
	Giffer     Giffer
	Th         *m.Theme
	Form       Form
	GifPlayer  GifPlayer
	Processing bool
	cache      *gif.GIF
	done       chan *gif.GIF
}

// Loop runs the event loop until terminated.
func (ui *UI) Loop() error {
	var (
		ops    op.Ops
		events = ui.Window.Events()
	)
	ui.Init()
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

func (ui *UI) Init() {
	ui.Form.FPS.SetText("12")
	ui.Form.Width.SetText("400")
	ui.Form.Height.SetText("350")
	ui.Form.Start.SetText("0")
	ui.Form.End.SetText("3")
	ui.done = make(chan *gif.GIF)
}

func (ui *UI) Update(gtx C) {
	if ui.Form.SaveBtn.Clicked() && ui.cache != nil {
		path, err := zenity.SelectFileSave(zenity.Title("Save Gif"), zenity.ConfirmOverwrite())
		if err != nil {
			log.Printf("error: selecting save file: %v", err)
		}
		savef, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0664)
		if err != nil {
			log.Printf("error: opening save file: %v", err)
		}
		defer func() { go savef.Close() }()
		if err := gif.EncodeAll(savef, ui.cache); err != nil {
			log.Printf("error: saving gif to %q: %v", path, err)
		}
		ui.cache = nil
	}
	if ui.Form.SubmitBtn.Clicked() {
		ui.Processing = true
		var (
			url  = ui.Form.URL.Text()
			fuzz = 0
		)
		// TODO(jfm): show as validation errors.
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
		go func() {
			g, err := ui.Giffer.GififyURL(
				url,
				start,
				end,
				fps,
				width,
				height,
				fuzz,
			)
			if err != nil {
				log.Printf("error: fetching gif: %v", err)
			}
			img, err := gif.DecodeAll(bytes.NewReader(g.Data))
			if err != nil {
				log.Printf("error: decoding gif: %v", err)
			}
			ui.done <- img
		}()
	}
	select {
	case g := <-ui.done:
		ui.cache = g
		ui.Processing = false
		ui.GifPlayer.Load(g)
	default:
	}
}

func (ui *UI) Layout(gtx C) D {
	return layout.Stack{}.Layout(
		gtx,
		layout.Stacked(func(gtx C) D {
			return l.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx C) D {
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
							if ui.Processing {
								gtx.Queue = nil
							}
							return l.Flex{
								Axis: l.Vertical,
							}.Layout(
								gtx,
								l.Rigid(func(gtx C) D {
									return ui.Form.LayoutFields(gtx, ui.Th)
								}),
								l.Rigid(func(gtx C) D {
									return l.Flex{
										Axis: l.Horizontal,
									}.Layout(
										gtx,
										l.Rigid(func(gtx C) D {
											return m.Button(ui.Th, &ui.Form.SubmitBtn, "Create").
												Layout(gtx)
										}),
										l.Rigid(func(gtx C) D {
											return D{Size: image.Point{X: gtx.Px(unit.Dp(10))}}
										}),
										l.Rigid(func(gtx C) D {
											if ui.cache == nil {
												gtx.Queue = nil
											}
											return m.Button(ui.Th, &ui.Form.SaveBtn, "Save").
												Layout(gtx)
										}),
									)
								}),
							)
						})
					}),
					l.Flexed(1, func(gtx C) D {
						return l.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, func(gtx C) D {
								var (
									cs     = &gtx.Constraints
									width  = gtx.Px(unit.Dp(350))
									height = gtx.Px(unit.Dp(250))
								)
								if ui.GifPlayer.Dimensions.X > 0 && ui.GifPlayer.Dimensions.Y > 0 {
									cs.Min.Y = ui.GifPlayer.Dimensions.Y
									cs.Min.X = ui.GifPlayer.Dimensions.X
								} else {
									cs.Max.X /= 2
									cs.Max.Y /= 2
									if cs.Max.X > width {
										cs.Max.X = width
									}
									if cs.Max.Y > height {
										cs.Max.Y = height
									}
								}
								return ui.GifPlayer.Layout(gtx)
							})
						})
					}),
				)
			})

		}),
		layout.Expanded(func(gtx C) D {
			if !ui.Processing {
				return D{}
			}
			return c.Rect{
				Color: color.NRGBA{A: 100},
				Size:  gtx.Constraints.Max,
			}.Layout(gtx)
		}),
		layout.Expanded(func(gtx C) D {
			if !ui.Processing {
				return D{}
			}
			cs := &gtx.Constraints
			cs.Max.X = gtx.Px(unit.Dp(50))
			cs.Max.Y = gtx.Px(unit.Dp(50))
			return l.Center.Layout(gtx, func(gtx C) D {
				return m.Loader(ui.Th).Layout(gtx)
			})
		}),
	)
}

// Form holds state for form inputs.
type Form struct {
	URL       c.TextField
	Start     c.TextField
	End       c.TextField
	Width     c.TextField
	Height    c.TextField
	FPS       c.TextField
	SubmitBtn widget.Clickable
	SaveBtn   widget.Clickable
}

func (f *Form) LayoutFields(gtx C, th *m.Theme) D {
	return l.Flex{
		Axis: l.Vertical,
	}.Layout(
		gtx,
		l.Rigid(func(gtx C) D {
			return f.URL.Layout(gtx, th, "url")
		}),
		l.Rigid(func(gtx C) D {
			return f.Start.Layout(gtx, th, "start (seconds)")
		}),
		l.Rigid(func(gtx C) D {
			return f.End.Layout(gtx, th, "end (seconds)")
		}),
		l.Rigid(func(gtx C) D {
			return f.Width.Layout(gtx, th, "width (pixels)")
		}),
		l.Rigid(func(gtx C) D {
			return f.Height.Layout(gtx, th, "height (pixels)")
		}),
		l.Rigid(func(gtx C) D {
			return f.FPS.Layout(gtx, th, "fps (integer)")
		}),
		l.Rigid(func(gtx C) D {
			return D{Size: image.Point{Y: gtx.Px(unit.Dp(10))}}
		}),
	)
}

func (f *Form) LayoutActions(gtx C, th *m.Theme) D {
	return l.Flex{
		Axis: l.Horizontal,
	}.Layout(
		gtx,
		l.Rigid(func(gtx C) D {
			return m.Button(th, &f.SubmitBtn, "Create").Layout(gtx)
		}),
		l.Rigid(func(gtx C) D {
			return D{Size: image.Point{X: gtx.Px(unit.Dp(10))}}
		}),
		l.Rigid(func(gtx C) D {
			return m.Button(th, &f.SaveBtn, "Save").Layout(gtx)
		}),
	)
}

// GifPlayer animates through a series of frames.
//
// TODO(jfm): fix! Displays garbled images.
// - playing way too fast
// - stretching image too much
type GifPlayer struct {
	Frames     []paint.ImageOp
	Delays     []time.Duration
	Cursor     int
	Dimensions image.Point
	since      time.Time
	img        widget.Image
}

// Load a Gif image to render.
func (g *GifPlayer) Load(src *gif.GIF) {
	g.Frames = make([]paint.ImageOp, len(src.Image))
	for ii := range src.Image {
		g.Frames[ii] = paint.NewImageOp(src.Image[ii])
	}
	g.Delays = make([]time.Duration, len(src.Delay))
	for ii := range g.Delays {
		// Gif delays are in 100th of a second.
		g.Delays[ii] = time.Duration(src.Delay[ii]) / 10 * time.Millisecond
	}
	g.Dimensions.X = src.Config.Width
	g.Dimensions.Y = src.Config.Height
}

// Ready if the next frame is ready to be displayed.
func (g *GifPlayer) Ready(gtx C) bool {
	if len(g.Delays) == 0 {
		return false
	}
	var (
		now     = gtx.Now
		since   = g.since
		latency = g.Delays[g.Cursor]
	)
	return now.Sub(since).Milliseconds() >= latency.Milliseconds()
}

// Next loads the next frame in the series.
func (g *GifPlayer) Next(gtx C) {
	defer func() {
		g.Cursor++
		if g.Cursor > len(g.Frames)-1 {
			g.Cursor = 0
		}
		g.since = gtx.Now
	}()
	g.img.Src = g.Frames[g.Cursor]
}

// Current returns the current frame.
func (g *GifPlayer) Current() paint.ImageOp {
	return g.Frames[g.Cursor]
}

func (g *GifPlayer) Layout(gtx C) D {
	if g == nil || len(g.Frames) == 0 {
		return c.Rect{
			Size: image.Point{
				X: gtx.Constraints.Max.X,
				Y: gtx.Constraints.Max.Y,
			},
			Color: color.NRGBA{A: 100},
		}.Layout(gtx)
	}
	op.InvalidateOp{}.Add(gtx.Ops)
	if g.Ready(gtx) {
		g.Next(gtx)
	}
	return g.img.Layout(gtx)
}
