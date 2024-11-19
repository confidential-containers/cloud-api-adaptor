package userdata

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

type kvPair struct {
	k string
	v string
}

func imdsGet(ctx context.Context, url string, b64 bool, headers []kvPair) ([]byte, error) {
	// If url is empty then return empty string
	if url == "" {
		return nil, fmt.Errorf("url is empty")
	}

	// Create a new HTTP client
	client := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", err)

	}

	for _, header := range headers {
		req.Header.Add(header.k, header.v)
	}

	// Send the request and retrieve the response
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %s", err)

	}
	defer resp.Body.Close()

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint %s returned != 200 status code: %s", url, resp.Status)
	}

	// Read the response body and return it as a string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", err)
	}

	if !b64 {
		return body, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to decode b64 encoded userData: %s", err)
	}
	return decoded, nil
}
