// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package accesslog

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAcccessLog(t *testing.T) {

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header["HEADER1"] = []string{"value1"}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Hello")); err != nil {
			t.Errorf("%v", err)
		}
	})
	logger := NewLogger(mux, WithLogger(log.Default()), WithLabel("test"), WithHeader(), WithBody())

	httpServer := httptest.NewServer(logger)
	defer httpServer.Close()

	_, err := http.Get(httpServer.URL)
	if err != nil {
		t.Fatalf("Expect no error, got %v", err)
	}
}
