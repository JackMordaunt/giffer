package main

import (
	"image"
	"image/color"
)

var _ image.Image = ImageStack{}

// ImageStack implements image.Image as a stack of images that returns the first non-zero color
// encountered.
type ImageStack struct {
	Stack  []image.Image
	Config image.Config
}

func (stack ImageStack) At(x, y int) color.Color {
	for _, img := range stack.Stack {
		c := img.At(x, y)
		if r, g, b, a := c.RGBA(); r > 0 || g > 0 || b > 0 || a > 0 {
			return c
		}
	}
	return color.NRGBA{}
}

func (stack ImageStack) Bounds() image.Rectangle {
	return image.Rectangle{
		Max: image.Point{
			X: stack.Config.Width,
			Y: stack.Config.Height,
		},
	}
}

func (stack ImageStack) ColorModel() color.Model {
	return stack.Config.ColorModel
}
