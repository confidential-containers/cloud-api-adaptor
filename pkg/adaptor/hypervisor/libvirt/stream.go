//go:build libvirt
// +build libvirt

package libvirt

// Code copied from https://github.com/openshift/cluster-api-provider-libvirt

import (
	"io"
	libvirt "libvirt.org/go/libvirt"
)

// StreamIO libvirt struct
type streamIO struct {
	stream libvirt.Stream
}

var _ io.Writer = &streamIO{}
var _ io.Reader = &streamIO{}
var _ io.Closer = &streamIO{}

// NewStreamIO returns libvirt StreamIO
func newStreamIO(s libvirt.Stream) *streamIO {
	return &streamIO{stream: s}
}

func (sio *streamIO) Read(p []byte) (int, error) {
	return sio.stream.Recv(p)
}

func (sio *streamIO) Write(p []byte) (int, error) {
	return sio.stream.Send(p)
}

// Close closes the stream
func (sio *streamIO) Close() error {
	return sio.stream.Finish()
}
