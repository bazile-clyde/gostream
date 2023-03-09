//go:build cgo

package h264

import "C"
import (
	"context"
	"fmt"
	"github.com/edaniels/golog"
	ourcodec "github.com/edaniels/gostream/codec"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avutil"
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
//
// INSTALLING FFMPEG FROM SOURCE:
//
//	use v4.X https://github.com/FFmpeg/FFmpeg/tree/release/4.4
type encoder struct {
	img     image.Image
	codec   *avcodec.Codec
	context *avcodec.Context
	width   int
	height  int
	reader  video.Reader
	pixFmt  int
	// TODO: The resulting struct must be freed using av_frame_free().
	frame *avutil.Frame
}

func (h *encoder) Read() (img image.Image, release func(), err error) {
	return h.img, nil, nil
}

func NewEncoder(width, height, _ int, _ golog.Logger) (ourcodec.VideoEncoder, error) {
	h := &encoder{width: width, height: height}

	encName := "h264_v4l2m2m"
	h.codec = avcodec.AvcodecFindEncoderByName(encName)
	if h.codec == nil {
		panic(fmt.Sprintf("cannot find encoder '%s'", encName))
	}

	h.context = h.codec.AvcodecAllocContext3()
	if h.context == nil {
		panic("cannot allocate video codec context")
	}

	h.pixFmt = avcodec.AV_PIX_FMT_YUV420P
	h.context.SetEncodeParams(width, height, avcodec.PixelFormat(h.pixFmt))
	h.context.SetTimebase(1, 30) // inverse of fps e.g., 30 fps == 1/30
	if h.context.AvcodecOpen2(h.codec, nil) < 0 {
		return nil, errors.New("cannot open codec")
	}

	// AVFrame is typically allocated once and then reused multiple times to hold
	// different data (e.g. a single AVFrame to hold frames received from a
	// decoder). In such a case, av_frame_unref() will free any references held by
	// the frame and reset it to its original clean state before it
	// is reused again.
	if h.frame = avutil.AvFrameAlloc(); h.frame == nil {
		return nil, errors.New("cannot alloc frame")
	}

	h.reader = video.ToI420((video.ReaderFunc)(h.Read))
	return h, nil
}

func (h *encoder) Encode(_ context.Context, img image.Image) ([]byte, error) {
	defer avutil.AvFrameUnref(h.frame)

	if err := avutil.AvSetFrame(h.frame, h.width, h.height, h.pixFmt); err != nil {
		return nil, errors.Wrap(err, "cannot set frame")
	}

	fmt.Println("FRAME SET")

	fmt.Println("WRITING IMG TO FRAME...")
	if ret := avutil.AvFrameMakeWritable(h.frame); ret < 0 {
		return nil, errors.Wrap(avutil.ErrorFromCode(ret), "cannot make frame writable")
	}

	h.img = img
	yuvImg, _, err := h.reader.Read()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image")
	}

	h.frame.AvSetFrameFromImg(yuvImg)
	// set frame->pts to time stamp, i.e., frame->pts = h.inc .... h.inc++
	fmt.Println("SET FRAME FROM IMG")

	fmt.Println("GETTING BYTES...")
	return h.encodeBytes()
}

func (h *encoder) encodeBytes() ([]byte, error) {
	pkt := avcodec.AvPacketAlloc()
	if pkt == nil {
		return nil, errors.Errorf("cannot allocate packet")
	}
	defer pkt.AvFreePacket()
	defer pkt.AvPacketUnref()

	if ret := h.context.AvCodecSendFrame((*avcodec.Frame)(unsafe.Pointer(h.frame))); ret < 0 {
		return nil, errors.Wrap(avutil.ErrorFromCode(ret), "cannot send frame for encoding")
	}

	var bytes []byte
	for ret := 0; ret >= 0; {
		ret = h.context.AvCodecReceivePacket(pkt)
		if ret == avutil.AvErrorEOF || ret == avutil.AvErrorEAGAIN {
			break
		} else if ret < 0 {
			return nil, errors.Wrap(avutil.ErrorFromCode(ret), fmt.Sprintf("error during encoding %d", ret))
		}

		fmt.Printf("write package %d (size=%5d)", pkt.Pts(), pkt.Size())
		payload := C.GoBytes(unsafe.Pointer(pkt.Data()), C.int(pkt.Size()))
		bytes = append(bytes, payload...)

		pkt.AvPacketUnref()
	}

	return bytes, nil
}
