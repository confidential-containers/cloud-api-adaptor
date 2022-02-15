// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"bytes"
	"flag"
	"testing"
)

func capture(fn func()) (string, bool, int) {

	oldOutput := flag.CommandLine.Output()
	defer flag.CommandLine.SetOutput(oldOutput)

	var buffer bytes.Buffer
	flag.CommandLine.SetOutput(&buffer)

	exitCh := make(chan int, 1)
	Exit = func(exitCode int) {
		exitCh <- exitCode
		close(exitCh)
	}

	fn()

	var exited bool
	var exitCode int
	select {
	case exitCode = <-exitCh:
		exited = true
	default:
	}

	return buffer.String(), exited, exitCode
}

func TestParse(t *testing.T) {

	args := []string{"command", "-option", "hello"}

	var opt string
	output, exited, _ := capture(func() {
		Parse(args[0], args, func(flags *flag.FlagSet) {
			flags.StringVar(&opt, "option", "", "an option")
		})
	})

	if e, a := "hello", opt; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := "", output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := false, exited; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
}

func TestParseHelp(t *testing.T) {

	args := []string{"command", "-help"}

	var opt string

	output, exited, exitCode := capture(func() {
		Parse(args[0], args, func(flags *flag.FlagSet) {
			flags.StringVar(&opt, "option", "", "an option")
		})
	})

	if e, a := "", opt; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := "Usage of command:\n  -option string\n    \tan option\n  -version\n    \tShow version information\n", output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := true, exited; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
	if e, a := 0, exitCode; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
}

func TestParseVersion(t *testing.T) {

	args := []string{"command", "-version"}

	var opt string

	output, exited, exitCode := capture(func() {
		Parse(args[0], args, func(flags *flag.FlagSet) {
			flags.StringVar(&opt, "option", "", "an option")
		})
	})

	if e, a := "", opt; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := "", output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := true, exited; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
	if e, a := 0, exitCode; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
}

func TestParseError(t *testing.T) {

	args := []string{"command", "-undef"}

	var opt string

	output, exited, exitCode := capture(func() {
		Parse(args[0], args, func(flags *flag.FlagSet) {
			flags.StringVar(&opt, "option", "", "an option")
		})
	})

	if e, a := "", opt; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := "flag provided but not defined: -undef\nUsage of command:\n  -option string\n    \tan option\n  -version\n    \tShow version information\n", output; e != a {
		t.Fatalf("Expect %q, got %q", e, a)
	}
	if e, a := true, exited; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
	if e, a := 1, exitCode; e != a {
		t.Fatalf("Expect %v, got %v", e, a)
	}
}
