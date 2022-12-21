// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/confidential-containers/cloud-api-adaptor/pkg/util"
)

func captureStdout(t *testing.T, fn func()) string {

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
	defer r.Close()

	old := os.Stdout
	os.Stdout = w

	fn()

	os.Stdout = old
	w.Close()

	var src io.Reader = r
	var dst bytes.Buffer

	if _, err := io.Copy(&dst, src); err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}

	return dst.String()
}

func TestShowVersion(t *testing.T) {

	programName := "test"

	output := captureStdout(t, func() {
		ShowVersion(programName)
	})

	if e, a := fmt.Sprintf("test, version: %s, commit: %s\ngo: %s\n", util.VERSION, util.COMMIT, runtime.Version()), output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}

	output = captureStdout(t, func() {
		ShowVersion(programName)
	})

	if e, a := fmt.Sprintf("test, version: %s, commit: %s\ngo: %s\n", util.VERSION, util.COMMIT, runtime.Version()), output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
}
