//go:build cgo

package h264

import "C"
import (
	"context"
	"fmt"
	"github.com/edaniels/golog"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avutil"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream/codec"
	"image"
	"unsafe"
)

const CODEC = "h264_v4l2m2m"

type encoder struct {
	img         image.Image
	reader      video.Reader
	codec       *avcodec.Codec
	context     *avcodec.Context
	width       int
	height      int
	pixelFormat int
	frame       *avutil.Frame
	pts         int64
	logger      golog.Logger
}

func (h *encoder) Read() (img image.Image, release func(), err error) {
	return h.img, nil, nil
}

func NewEncoder(width, height, keyFrameInterval int, logger golog.Logger) (codec.VideoEncoder, error) {
	h := &encoder{width: width, height: height, logger: logger}

	if h.codec = avcodec.AvcodecFindEncoderByName(CODEC); h.codec == nil {
		return nil, errors.Errorf("cannot find encoder '%s'", CODEC)
	}

	if h.context = h.codec.AvcodecAllocContext3(); h.context == nil {
		return nil, errors.Errorf("cannot allocate video codec context")
	}

	// This format is one of the output formats support by the bcm2835-codec at /dev/video11
	// It is also known as YU12. See https://www.kernel.org/doc/html/v4.10/media/uapi/v4l/pixfmt-yuv420.html
	h.pixelFormat = avcodec.AV_PIX_FMT_YUV420P

	h.context.SetEncodeParams(width, height, avcodec.PixelFormat(h.pixelFormat))
	h.context.SetTimebase(1, keyFrameInterval)

	h.reader = video.ToI420((video.ReaderFunc)(h.Read))

	if h.context.AvcodecOpen2(h.codec, nil) < 0 {
		return nil, errors.New("cannot open codec")
	}

	if h.frame = avutil.AvFrameAlloc(); h.frame == nil {
		h.context.AvcodecClose()
		return nil, errors.New("cannot alloc frame")
	}

	return h, nil
}

func (h *encoder) Encode(ctx context.Context, img image.Image) ([]byte, error) {
	if err := avutil.AvSetFrame(h.frame, h.width, h.height, h.pixelFormat); err != nil {
		return nil, errors.Wrap(err, "cannot set frame")
	}

	if ret := avutil.AvFrameMakeWritable(h.frame); ret < 0 {
		return nil, errors.Wrap(avutil.ErrorFromCode(ret), "cannot make frame writable")
	}

	h.img = img
	yuvImg, _, err := h.reader.Read()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image")
	}

	h.frame.AvSetFrameFromImg(yuvImg.(*image.YCbCr))
	h.frame.AvSetFramePTS(h.pts)
	h.pts++

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return h.encodeBytes(ctx)
	}
}

func (h *encoder) encodeBytes(ctx context.Context) ([]byte, error) {
	pkt := avcodec.AvPacketAlloc()
	if pkt == nil {
		return nil, errors.Errorf("cannot allocate packet")
	}
	defer pkt.AvFreePacket()
	defer pkt.AvPacketUnref()
	defer avutil.AvFrameUnref(h.frame)

	if ret := h.context.AvCodecSendFrame((*avcodec.Frame)(unsafe.Pointer(h.frame))); ret < 0 {
		return nil, errors.Wrap(avutil.ErrorFromCode(ret), "cannot send frame for encoding")
	}

	var bytes []byte
	for ret := 0; ret >= 0; {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		ret = h.context.AvCodecReceivePacket(pkt)
		if ret == avutil.AvErrorEOF || ret == avutil.AvErrorEAGAIN {
			break
		} else if ret == -11 {
			break
		} else if ret < 0 {
			return nil, errors.Wrap(avutil.ErrorFromCode(ret), fmt.Sprintf("error during encoding %d", ret))
		}

		payload := C.GoBytes(unsafe.Pointer(pkt.Data()), C.int(pkt.Size()))
		bytes = append(bytes, payload...)

		pkt.AvPacketUnref()
	}

	return bytes, nil
}
