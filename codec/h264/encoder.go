//go:build cgo

package h264

import "C"
import (
	"context"
	"github.com/edaniels/golog"
	ourcodec "github.com/edaniels/gostream/codec"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avutil"
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
//
// INSTALLING FFMPEG FROM SOURCE:
//
//	use v4.X https://github.com/FFmpeg/FFmpeg/tree/release/4.4
type encoder struct {
	img    image.Image
	width  int
	height int
}

func (h *encoder) Read() (img image.Image, release func(), err error) {
	return h.img, nil, nil
}

func NewEncoder(width, height, _ int, _ golog.Logger) (ourcodec.VideoEncoder, error) {
	h := &encoder{width: width, height: height}
	return h, nil
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
	defer _context.AvcodecFreeContext()

	pkt := avcodec.AvPacketAlloc()
	if pkt == nil {
		return nil, errors.Errorf("cannot allocate packet")
	}
	defer pkt.AvFreePacket()

	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	pixFmt := avcodec.PixelFormat(avcodec.AV_PIX_FMT_YUV)
	_context.SetEncodeParams2(
		width,
		height,
		pixFmt,
		true,
		10,
	)
	_context.SetTimebase(1, 25)

	if _context.AvcodecOpen2(_codec, nil) < 0 {
		return nil, errors.New("cannot open codec")
	}

	frame := avutil.AvFrameAlloc()
	defer avutil.AvFrameFree(frame)
	if err := avutil.AvSetFrame(frame, h.width, h.height, pixFmt); err != nil {
		return nil, errors.New("cannot allocate the video frame data")
	}

	frame.AvSetFrameFromImg(img)

	if ret := _context.AvCodecSendFrame((*avcodec.Frame)(unsafe.Pointer(frame))); ret < 0 {
		return nil, errors.New("error sending a frame for encoding")
	}

	var bytes []byte
	for {
		ret := _context.AvCodecReceivePacket(pkt)
		if ret == avutil.AvErrorEOF || ret == avutil.AvErrorEAGAIN {
			break
		} else if ret < 0 {
			return nil, errors.New("error during encoding")
		}

		payload := C.GoBytes(unsafe.Pointer(pkt.Data()), C.int(pkt.Size()))
		bytes = append(bytes, payload...)

		pkt.AvPacketUnref()
	}

	return bytes, nil
}
