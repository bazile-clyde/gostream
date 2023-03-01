//go:build cgo

package h264

import "C"
import (
	"context"
	"github.com/edaniels/golog"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avutil"
	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pkg/errors"
	"image"
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

func (h *encoder) encode(ctx avcodec.Context, codec avcodec.Codec, img image.Image) ([]byte, error) {
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

	h.img = img
	var r video.ReaderFunc = h.Read
	yuvImg, _, err := video.ToI420(r).Read()
	if err != nil {
		return nil, errors.New("could not convert image to I420")
	}

	vFrame := avutil.AvFrameAlloc()
	avutil.SetPicture(vFrame, yuvImg.(*image.YCbCr))

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

	return h.encode(_context, _codec, img)
}
