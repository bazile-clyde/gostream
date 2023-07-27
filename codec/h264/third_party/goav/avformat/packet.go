// Use of this source code is governed by a MIT license that can be found in the LICENSE file.
// Giorgis (habtom@giorgis.io)

package avformat

//#cgo CFLAGS: -I${SRCDIR}/../../FFmpeg
//#include <libavformat/avformat.h>
import "C"
import (
	"unsafe"

	"github.com/viamrobotics/gostream/codec/h264/third_party/goav/avcodec"
)

func toCPacket(pkt *avcodec.Packet) *C.struct_AVPacket {
	return (*C.struct_AVPacket)(unsafe.Pointer(pkt))
}

func fromCPacket(pkt *C.struct_AVPacket) *avcodec.Packet {
	return (*avcodec.Packet)(unsafe.Pointer(pkt))
}
