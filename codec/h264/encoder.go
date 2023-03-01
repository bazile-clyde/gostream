//go:build cgo

package h264

// #include "convert_cgo.h"
// #cgo CFLAGS: -std=c11
import "C"

import (
	"context"
	"github.com/edaniels/golog"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avutil"
	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pkg/errors"
	"image"
	"image/color"
	"unsafe"
)

// ffmpeg -f v4l2 -i /dev/video0 -codec:v h264_v4l2m2m webcam.mkv
//
// Main options
// -f fmt (input/output)
//
//	Force input or output file format. The format is normally auto-detected for input files and guessed from the file extension for output files, so this
//	option is not needed in most cases.
//
// -i url (input)
//
//	input file url
//
// -c[:stream_specifier] codec (input/output,per-stream)
// -codec[:stream_specifier] codec (input/output,per-stream)
//
//	Select an encoder (when used before an output file) or a decoder (when used before an input file) for one or more streams. codec is the name of a
//	decoder/encoder or a special value "copy" (output only) to indicate that the stream is not to be re-encoded.
//
//	For example
//
//			ffmpeg -i INPUT -map 0 -c:v libx264 -c:a copy OUTPUT
//
//		encodes all video streams with libx264 and copies all audio streams.
//
//		For each stream, the last matching "c" option is applied, so
//
//			ffmpeg -i INPUT -map 0 -c copy -c:v:1 libx264 -c:a:137 libvorbis OUTPUT
//
//		will copy all the streams except the second video, which will be encoded with libx264, and the 138th audio, which will be encoded with libvorbis.
type encoder struct {
	codec  codec.ReadCloser
	img    image.Image
	logger golog.Logger
}

func (h *encoder) Read() (img image.Image, release func(), err error) {
	return h.img, nil, nil
}

func rgbaToI444(dst *image.YCbCr, src *image.RGBA) {
	C.rgbaToI444(
		(*C.uchar)(&dst.Y[0]),
		(*C.uchar)(&dst.Cb[0]),
		(*C.uchar)(&dst.Cr[0]),
		(*C.uchar)(&src.Pix[0]),
		C.int(src.Rect.Dx()),
		C.int(src.Rect.Dy()),
	)
}

// imageToYCbCr converts src to *image.YCbCr and store it to dst
// Note: conversion can be lossy
func imageToYCbCr(dst *image.YCbCr, src image.Image) {
	if dst == nil {
		panic("dst can't be nil")
	}

	yuvImg, ok := src.(*image.YCbCr)
	if ok {
		*dst = *yuvImg
		return
	}

	bounds := src.Bounds()
	dy := bounds.Dy()
	dx := bounds.Dx()
	flat := dy * dx

	if len(dst.Y)+len(dst.Cb)+len(dst.Cr) < 3*flat {
		i0 := 1 * flat
		i1 := 2 * flat
		i2 := 3 * flat
		if cap(dst.Y) < i2 {
			dst.Y = make([]uint8, i2)
		}
		dst.Y = dst.Y[:i0]
		dst.Cb = dst.Y[i0:i1]
		dst.Cr = dst.Y[i1:i2]
	}
	dst.SubsampleRatio = image.YCbCrSubsampleRatio444
	dst.YStride = dx
	dst.CStride = dx
	dst.Rect = bounds

	switch s := src.(type) {
	case *image.RGBA:
		rgbaToI444(dst, s)
	default:
		i := 0
		for yi := 0; yi < dy; yi++ {
			for xi := 0; xi < dx; xi++ {
				// TODO: probably try to get the alpha value with something like
				// https://en.wikipedia.org/wiki/Alpha_compositing
				r, g, b, _ := src.At(xi, yi).RGBA()
				yy, cb, cr := color.RGBToYCbCr(uint8(r/256), uint8(g/256), uint8(b/256))
				dst.Y[i] = yy
				dst.Cb[i] = cb
				dst.Cr[i] = cr
				i++
			}
		}
	}
}

func encode(ctx avcodec.Context, codec avcodec.Codec, img image.Image) ([]byte, error) {
	if ctx.AvcodecIsOpen() == 0 {
		return nil, errors.New("codec context not open")
	}

	if codec.AvCodecIsEncoder() == 0 {
		return nil, errors.New("codec is not an encoder")
	}

	pkt := avcodec.AvPacketAlloc()
	if pkt == nil {
		return nil, errors.Errorf("cannot allocate packet")
	}

	var yuvImg image.YCbCr
	imageToYCbCr(&yuvImg, img)

	vFrame := avutil.AvFrameAlloc()
	avutil.SetPicture(vFrame, &yuvImg)

	var gp int
	if ctx.AvcodecEncodeVideo2(pkt, vFrame, &gp); gp < 0 {
		return nil, errors.New("cannot encode video frame")
	}
	defer avutil.AvFrameFree(vFrame)

	payload := C.GoBytes(unsafe.Pointer(pkt.Data()), C.int(pkt.Size()))
	return payload, nil
}

func (h *encoder) Encode(_ context.Context, img image.Image) ([]byte, error) {
	encName := "h264_v4l2m2m"
	_codec := avcodec.AvcodecFindEncoderByName(encName)
	if _codec == nil {
		return nil, errors.Errorf("cannot find encoder '%s'", encName)
	}

	_context := _codec.AvcodecAllocContext3()
	if _context == nil {
		return nil, errors.Errorf("cannot allocate video codec context")
	}

	width := 1280
	height := 720
	pixFmt := avcodec.AV_PIX_FMT_YUV420P16
	_context.SetEncodeParams(width, height, pixFmt)

	if _context.CodecId() != avcodec.AV_CODEC_ID_H264 {
		return nil, errors.New("not H264 encoder")
	}

	if _context.AvcodecOpen2(_codec, nil) < 0 {
		return nil, errors.New("cannot open codec")
	}

	return encode(_context, _codec, img)
}
