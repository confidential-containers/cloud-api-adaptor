// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package accesslog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type AccessLogger struct {
	http.Handler

	logger     *log.Logger
	label      string
	header     bool
	body       bool
	onlyErrors bool
}

type Option func(*AccessLogger)

func WithLogger(logger *log.Logger) Option {
	return func(l *AccessLogger) {
		l.logger = logger
	}
}

func WithLabel(label string) Option {
	return func(l *AccessLogger) {
		l.label = fmt.Sprintf("[%s] ", label)
	}
}

func WithHeader() Option {
	return func(l *AccessLogger) {
		l.header = true
	}
}

func WithBody() Option {
	return func(l *AccessLogger) {
		l.body = true
	}
}

func WithOnlyErrors() Option {
	return func(l *AccessLogger) {
		l.onlyErrors = true
	}
}

type wrapper struct {
	http.ResponseWriter

	header      http.Header
	statusCode  int
	multiWriter io.Writer
}

func NewLogger(handler http.Handler, opts ...Option) *AccessLogger {

	l := &AccessLogger{
		Handler: handler,
		logger:  log.Default(),
	}

	for _, fn := range opts {
		fn(l)
	}

	return l
}

func (w *wrapper) Header() http.Header {
	w.header = w.ResponseWriter.Header()
	return w.header
}

func (w *wrapper) Write(bytes []byte) (int, error) {
	return w.multiWriter.Write(bytes)
}

func (w *wrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *wrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("Not implement http.Hijacker: %T", w.ResponseWriter)
	}
	return hijacker.Hijack()
}

func (w *wrapper) Flush() {

	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}

	flusher.Flush()
}

func (l *AccessLogger) printRequest(request *http.Request) {

	if !l.header {
		l.logger.Printf("%sHTTP request from %s: %s %s %s", l.label, request.RemoteAddr, request.Method, request.URL.Path, request.Proto)
	} else {
		l.logger.Printf("%sHTTP request from %s", l.label, request.RemoteAddr)
		l.logger.Printf("    %s %s %s", request.Method, request.URL.Path, request.Proto)

		if request.Host != "" {
			l.logger.Printf("    Host: %s", request.Host)
		}
		for header, values := range request.Header {
			for _, value := range values {
				l.logger.Printf("    %s: %s", header, value)
			}
		}
		if l.body {
			l.logger.Print("    ")
		}
	}
	if l.body {
		// TODO: Encode binary data
		scanner := bufio.NewScanner(request.Body)
		for scanner.Scan() {
			line := scanner.Text()
			l.logger.Printf("    %s", line)
		}
	}
}

func (l *AccessLogger) printResponse(buffer *bytes.Buffer, wrapper *wrapper, request *http.Request) {
	code := wrapper.statusCode

	if !l.header {
		l.logger.Printf("%sHTTP response to %s: %s %d %s", l.label, request.RemoteAddr, request.Proto, code, http.StatusText(code))
	} else {
		l.logger.Printf("%sHTTP response to %s", l.label, request.RemoteAddr)
		l.logger.Printf("    %s %d %s", request.Proto, code, http.StatusText(code))

		// FIXME: automatically generated headers like DATE are not shown
		for header, values := range wrapper.header {
			for _, value := range values {
				l.logger.Printf("    %s: %s", header, value)
			}
		}
		if l.body {
			l.logger.Print("    ")
		}
	}
	if l.body {
		// TODO: Encode binary data
		scanner := bufio.NewScanner(buffer)
		for scanner.Scan() {
			line := scanner.Text()
			l.logger.Printf("    %s", line)
		}
	}
}

func (l *AccessLogger) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {

	if !l.onlyErrors {
		l.printRequest(request)
	}

	var buffer bytes.Buffer

	wrapper := &wrapper{
		ResponseWriter: responseWriter,
		multiWriter:    io.MultiWriter(responseWriter, &buffer),
	}

	l.Handler.ServeHTTP(wrapper, request)

	code := wrapper.statusCode
	if code == 0 {
		// WriteHeader is not called
		return
	}
	if code < 400 && l.onlyErrors {
		return
	}

	if l.onlyErrors {
		l.printRequest(request)
	}
	l.printResponse(&buffer, wrapper, request)
}
