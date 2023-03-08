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
	img        image.Image
	codec      *avcodec.Codec
	context    *avcodec.Context
	width      int
	height     int
	sentFrames int
	maxFrames  int
	reader     video.Reader
}

func (h *encoder) Read() (img image.Image, release func(), err error) {
	return h.img, nil, nil
}

func NewEncoder(width, height, _ int, _ golog.Logger) (ourcodec.VideoEncoder, error) {
	h := &encoder{width: width, height: height, maxFrames: 2}

	encName := "h264_v4l2m2m"
	h.codec = avcodec.AvcodecFindEncoderByName(encName)
	if h.codec == nil {
		panic(fmt.Sprintf("cannot find encoder '%s'", encName))
	}

	h.context = h.codec.AvcodecAllocContext3()
	if h.context == nil {
		panic("cannot allocate video codec context")
	}

	h.reader = video.ToI420((video.ReaderFunc)(h.Read))
	return h, nil
}

func (h *encoder) Encode(_ context.Context, img image.Image) ([]byte, error) {
	pkt := avcodec.AvPacketAlloc()
	if pkt == nil {
		return nil, errors.Errorf("cannot allocate packet")
	}
	defer pkt.AvFreePacket()

	pixFmt := avcodec.AV_PIX_FMT_YUV
	h.context.SetEncodeParams2(
		h.width,
		h.height,
		avcodec.PixelFormat(pixFmt),
		true,
		30,
	)

	h.context.SetTimebase(1, 25)
	if h.context.AvcodecOpen2(h.codec, nil) < 0 {
		return nil, errors.New("cannot open codec")
	}

	fmt.Println("ALLOCATING FRAME...")
	frame := avutil.AvFrameAlloc()
	defer avutil.AvFrameFree(frame)
	if err := avutil.AvSetFrame(frame, h.width, h.height, pixFmt); err != nil {
		return nil, errors.New("cannot allocate the video frame data")
	}
	fmt.Println("ALLOCATED FRAME")

	h.img = img
	yuvImg, _, err := h.reader.Read()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image")
	}

	avutil.SetPicture(frame, yuvImg.(*image.YCbCr))

	fmt.Println("SENDING FRAME...")
	if ret := h.context.AvCodecSendFrame((*avcodec.Frame)(unsafe.Pointer(frame))); ret < 0 {
		return nil, errors.New("error sending a frame for encoding")
	}
	fmt.Println("FRAME SENT")

	// if h.sentFrames < h.maxFrames {
	// 	h.sentFrames += 1
	// 	return []byte{}, nil
	// }

	// h.sentFrames = 0

	var bytes []byte
	fmt.Println("GETTING BYTES")
	for {
		ret := h.context.AvCodecReceivePacket(pkt)
		if ret == avutil.AvErrorEOF || ret == avutil.AvErrorEAGAIN || ret == -11 {
			break
		}
		if ret < 0 {
			return nil, errors.Errorf("error during encoding: %v", ret)
		}

		fmt.Printf("SUCCESS GETTING PACKET: %v\n", len(bytes))

		payload := C.GoBytes(unsafe.Pointer(pkt.Data()), C.int(pkt.Size()))
		bytes = append(bytes, payload...)

		pkt.AvPacketUnref()
	}

	return bytes, nil
}
