// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"runtime"
)

var (
	VERSION = "unknown"
	COMMIT  = "unknown"
)

func ShowVersion(programName string) {
	fmt.Printf("%s, version: %s, commit: %v\n", programName, VERSION, COMMIT)

	fmt.Printf("go: %s\n", runtime.Version())
}

type (
	Foo struct {
		A int
		B string
	}
	FooHasPointer struct {
		A *int
		B string
	}
)

func escapeValue() *int {
	var a int // moved to heap: a
	a = 1
	return &a
}

func noescapeNew() {
	newa := new(int) // noescapeNew new(int) does not escape
	*newa = 1
}

func escapePointer() FooHasPointer {
	var foo FooHasPointer
	i := 10 //moved to heap: i
	foo.A = &i
	foo.B = "a"
	return foo
}

func noescapeValue() Foo {
	var foo Foo
	i := 10
	foo.A = i
	foo.B = "a"
	return foo
}
