package h264

import (
	"github.com/edaniels/golog"
	"github.com/giorgisio/goav/avcodec"

	"github.com/viamrobotics/gostream"
	"github.com/viamrobotics/gostream/codec"
)

var DefaultStreamConfig gostream.StreamConfig

func init() {
	avcodec.AvcodecRegisterAll()
	DefaultStreamConfig.VideoEncoderFactory = NewEncoderFactory()
}

func NewEncoderFactory() codec.VideoEncoderFactory {
	return &factory{}
}

type factory struct{}

func (f *factory) New(width, height, keyFrameInterval int, logger golog.Logger) (codec.VideoEncoder, error) {
	return NewEncoder(width, height, keyFrameInterval, logger)
}

func (f *factory) MIMEType() string {
	return "video/H264"
}
