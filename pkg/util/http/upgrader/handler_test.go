// (C) Copyright IBM Corp. 2022.
// SPDX-License-Identifier: Apache-2.0

package upgrader

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeHTTP(t *testing.T) {
	rawURL := ""
	req, _ := http.NewRequest("POST", rawURL, nil)
	res := httptest.NewRecorder()

	handler := NewHandler()
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusInternalServerError, res.Result().StatusCode, "verify StatusInternalServerError")

}
