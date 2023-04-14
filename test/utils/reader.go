// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"
	"sync"
)

// CustomReader helps uplaod cos object
type CustomReader struct {
	Fp           *os.File
	Size         int64
	Reader       int64
	SignMap      map[int64]struct{}
	Mux          sync.Mutex
	HideProgress string
}

func (r *CustomReader) Read(p []byte) (int, error) {
	return r.Fp.Read(p)
}

func (r *CustomReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.Fp.ReadAt(p, off)
	if err != nil {
		return n, err
	}

	r.Mux.Lock()
	// Ignore the first signature call
	if _, ok := r.SignMap[off]; ok {
		// Got the length have read( or means has uploaded), and you can construct your message
		r.Reader += int64(n)
		// DO NOT show the progress when HIDE_UPLOADER_PROGRESS env is set
		if r.HideProgress == "" {
			fmt.Printf("\rtotal read:%d    progress:%d%%", r.Reader, int(float32(r.Reader*100)/float32(r.Size)))
		}
	} else {
		r.SignMap[off] = struct{}{}
	}
	r.Mux.Unlock()
	return n, err
}

func (r *CustomReader) Seek(offset int64, whence int) (int64, error) {
	return r.Fp.Seek(offset, whence)
}
