package h264

// ffmpeg -f v4l2 -i /dev/video0 -codec:v h264_v4l2m2m webcam.mkv
//
// Main options
// -f fmt (input/output)
// 		Force input or output file format. The format is normally auto-detected for input files and guessed from the file extension for output files, so this
// 		option is not needed in most cases.
// -i url (input)
// 		input file url
// -c[:stream_specifier] codec (input/output,per-stream)
// -codec[:stream_specifier] codec (input/output,per-stream)
// 		Select an encoder (when used before an output file) or a decoder (when used before an input file) for one or more streams. codec is the name of a
// 		decoder/encoder or a special value "copy" (output only) to indicate that the stream is not to be re-encoded.
//
// 		For example
//
// 				ffmpeg -i INPUT -map 0 -c:v libx264 -c:a copy OUTPUT
//
// 			encodes all video streams with libx264 and copies all audio streams.
//
// 			For each stream, the last matching "c" option is applied, so
//
// 				ffmpeg -i INPUT -map 0 -c copy -c:v:1 libx264 -c:a:137 libvorbis OUTPUT
//
// 			will copy all the streams except the second video, which will be encoded with libx264, and the 138th audio, which will be encoded with libvorbis.
//
