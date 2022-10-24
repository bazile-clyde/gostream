package gostream

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
)

type imageSource struct {
	Images []image.Image
	idx    int
}

// Returns the next image or nil if there are no more images left. This should never error.
func (is *imageSource) Read(_ context.Context) (image.Image, func(), error) {
	if is.idx >= len(is.Images) {
		return nil, func() {}, nil
	}
	img := is.Images[is.idx]
	is.idx++
	return img, func() {}, nil
}

func (is *imageSource) Close(_ context.Context) error {
	return nil
}

func pngToImage(t *testing.T, path string) image.Image {
	t.Helper()
	openBytes, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	img, err := png.Decode(bytes.NewReader(openBytes))
	test.That(t, err, test.ShouldBeNil)
	return img
}

func TestReadMedia(t *testing.T) {
	colors := []image.Image{
		pngToImage(t, "data/red.png"),
		pngToImage(t, "data/blue.png"),
		pngToImage(t, "data/green.png"),
		pngToImage(t, "data/yellow.png"),
		pngToImage(t, "data/fuchsia.png"),
		pngToImage(t, "data/cyan.png"),
	}

	imgSource := imageSource{Images: colors}
	videoSrc := NewVideoSource(&imgSource, prop.Video{})
	// Test all images are returned in order.
	for _, expected := range colors {
		actual, _, err := ReadMedia(context.Background(), videoSrc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageToColor(t, actual), test.ShouldEqual, imageToColor(t, expected))
	}

	// Test image comparison can fail if two images are not the same
	imgSource.Images = []image.Image{pngToImage(t, "data/red.png")}
	videoSrc = NewVideoSource(&imgSource, prop.Video{})

	blue := pngToImage(t, "data/blue.png")
	red, _, err := ReadMedia(context.Background(), videoSrc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, red, test.ShouldNotEqual, blue)
}

func imageToColor(t *testing.T, img image.Image) string {
	t.Helper()
	r, g, b, a := img.At(1, 1).RGBA()
	rgba := color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}
	switch rgba {
	case color.RGBA{R: 255, G: 0, B: 0, A: 255}:
		return "red"
	case color.RGBA{R: 0, G: 255, B: 0, A: 255}:
		return "green"
	case color.RGBA{R: 0, G: 0, B: 255, A: 255}:
		return "blue"
	case color.RGBA{R: 255, G: 255, B: 0, A: 255}:
		return "yellow"
	case color.RGBA{R: 255, G: 0, B: 255, A: 255}:
		return "fuchsia"
	case color.RGBA{R: 0, G: 255, B: 255, A: 255}:
		return "cyan"
	default:
		t.Errorf("rgba=%v undefined", rgba)
		return ""
	}
}

func TestStreamNext(t *testing.T) {
	colors := []image.Image{
		pngToImage(t, "data/red.png"),
		pngToImage(t, "data/blue.png"),
		pngToImage(t, "data/green.png"),
		pngToImage(t, "data/yellow.png"),
		pngToImage(t, "data/fuchsia.png"),
		pngToImage(t, "data/cyan.png"),
	}

	imgSource := imageSource{Images: colors}
	videoSrc := NewVideoSource(&imgSource, prop.Video{})
	stream, err := videoSrc.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// Test all images are returned in order.
	for _, expected := range colors {
		actual, _, err := stream.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imageToColor(t, actual), test.ShouldEqual, imageToColor(t, expected))
	}
}
