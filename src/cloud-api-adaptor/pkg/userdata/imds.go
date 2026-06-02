package userdata

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

type kvPair struct {
	k string
	v string
}

const awsIMDSv2TokenFetchTimeout = 5 * time.Second

func awsIMDSv2Token(ctx context.Context, tokenURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create IMDSv2 token request: %w", err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", AWSIMDSv2TokenTTL)

	client := &http.Client{Timeout: awsIMDSv2TokenFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send IMDSv2 token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IMDSv2 token endpoint returned %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IMDSv2 token response: %w", err)
	}
	return string(body), nil
}

func awsIMDSHeaders(ctx context.Context, tokenURL string) []kvPair {
	token, err := awsIMDSv2Token(ctx, tokenURL)
	if err != nil {
		logger.Printf("IMDSv2 token fetch failed (%v); falling back to IMDSv1\n", err)
		return nil
	}
	return []kvPair{{"X-aws-ec2-metadata-token", token}}
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
