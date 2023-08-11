package gimage

import (
	"image"
	"image/color"
)

// H264 does nothing but holds bytes. Methods return dummy values below to fulfill image.Image interface
type H264 struct {
	Bytes []byte
}

func (h H264) ColorModel() color.Model {
	return color.Alpha16Model
}

func (h H264) Bounds() image.Rectangle {
	return image.Rect(0, 0, 0, 0)
}

func (h H264) At(x, y int) color.Color {
	return color.Black
}

func NewH264Image(b []byte) image.Image {
	return H264{b}
}
