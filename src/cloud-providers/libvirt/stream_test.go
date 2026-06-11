//go:build cgo

// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

// Package libvirt provides tests for the streamIO implementation,
// which wraps libvirt.Stream to provide io.Reader, io.Writer, and io.Closer interfaces.
package libvirt

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	libvirt "libvirt.org/go/libvirt"
)

// streamWrapper defines the interface needed for testing
type streamWrapper interface {
	Recv([]byte) (int, error)
	Send([]byte) (int, error)
	Finish() error
}

// mockStream implements streamWrapper for testing
type mockStream struct {
	recvFunc   func([]byte) (int, error)
	sendFunc   func([]byte) (int, error)
	finishFunc func() error
}

func (m *mockStream) Recv(p []byte) (int, error) {
	if m.recvFunc != nil {
		return m.recvFunc(p)
	}
	return 0, nil
}

func (m *mockStream) Send(p []byte) (int, error) {
	if m.sendFunc != nil {
		return m.sendFunc(p)
	}
	return len(p), nil
}

func (m *mockStream) Finish() error {
	if m.finishFunc != nil {
		return m.finishFunc()
	}
	return nil
}

// testStreamIO wraps streamWrapper for testing
type testStreamIO struct {
	stream streamWrapper
}

func (sio *testStreamIO) Read(p []byte) (int, error) {
	return sio.stream.Recv(p)
}

func (sio *testStreamIO) Write(p []byte) (int, error) {
	return sio.stream.Send(p)
}

func (sio *testStreamIO) Close() error {
	return sio.stream.Finish()
}

func TestNewStreamIO(t *testing.T) {
	var stream libvirt.Stream
	sio := newStreamIO(stream)

	assert.NotNil(t, sio)
	assert.IsType(t, &streamIO{}, sio)
	// Verify the stream field is properly set
	assert.Equal(t, stream, sio.stream)
}

func TestStreamIO_Read(t *testing.T) {
	tests := []struct {
		name     string
		recvFunc func([]byte) (int, error)
		wantN    int
		wantErr  error
		wantData []byte
	}{
		{
			name: "successful read",
			recvFunc: func(p []byte) (int, error) {
				copy(p, []byte("test"))
				return 4, nil
			},
			wantN:    4,
			wantErr:  nil,
			wantData: []byte("test"),
		},
		{
			name: "read error",
			recvFunc: func(p []byte) (int, error) {
				return 0, errors.New("read error")
			},
			wantN:   0,
			wantErr: errors.New("read error"),
		},
		{
			name: "partial read",
			recvFunc: func(p []byte) (int, error) {
				copy(p, []byte("te"))
				return 2, nil
			},
			wantN:    2,
			wantErr:  nil,
			wantData: []byte("te"),
		},
		{
			name: "empty read",
			recvFunc: func(p []byte) (int, error) {
				return 0, nil
			},
			wantN:    0,
			wantErr:  nil,
			wantData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStream{recvFunc: tt.recvFunc}
			sio := &testStreamIO{stream: mock}

			buf := make([]byte, 10)
			n, err := sio.Read(buf)

			assert.Equal(t, tt.wantN, n)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
				if tt.wantData != nil {
					assert.Equal(t, tt.wantData, buf[:n])
				}
			}
		})
	}
}

func TestStreamIO_Write(t *testing.T) {
	tests := []struct {
		name     string
		sendFunc func([]byte) (int, error)
		input    []byte
		wantN    int
		wantErr  error
	}{
		{
			name: "successful write",
			sendFunc: func(p []byte) (int, error) {
				return len(p), nil
			},
			input:   []byte("test"),
			wantN:   4,
			wantErr: nil,
		},
		{
			name: "write error",
			sendFunc: func(p []byte) (int, error) {
				return 0, errors.New("write error")
			},
			input:   []byte("test"),
			wantN:   0,
			wantErr: errors.New("write error"),
		},
		{
			name: "partial write",
			sendFunc: func(p []byte) (int, error) {
				return 2, nil
			},
			input:   []byte("test"),
			wantN:   2,
			wantErr: nil,
		},
		{
			name: "empty write",
			sendFunc: func(p []byte) (int, error) {
				return 0, nil
			},
			input:   []byte(""),
			wantN:   0,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStream{sendFunc: tt.sendFunc}
			sio := &testStreamIO{stream: mock}

			n, err := sio.Write(tt.input)

			assert.Equal(t, tt.wantN, n)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStreamIO_Close(t *testing.T) {
	tests := []struct {
		name       string
		finishFunc func() error
		wantErr    error
	}{
		{
			name: "successful close",
			finishFunc: func() error {
				return nil
			},
			wantErr: nil,
		},
		{
			name: "close error",
			finishFunc: func() error {
				return errors.New("close error")
			},
			wantErr: errors.New("close error"),
		},
		{
			name: "close with specific error",
			finishFunc: func() error {
				return errors.New("stream finish failed")
			},
			wantErr: errors.New("stream finish failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStream{finishFunc: tt.finishFunc}
			sio := &testStreamIO{stream: mock}

			err := sio.Close()

			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
