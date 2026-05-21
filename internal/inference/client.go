package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

const socketPath = "/tmp/eidolon.sock"

type Request struct {
	RequestID   string  `json:"request_id"`
	Prefix      string  `json:"prefix"`
	Suffix      string  `json:"suffix"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
}

type Token struct {
	RequestID string `json:"request_id"`
	Token     string `json:"token"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{
		http: &http.Client{Transport: transport, Timeout: 120 * time.Second},
	}
}

func (c *Client) Complete(ctx context.Context, req Request) (<-chan Token, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://unix/complete", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("inference request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("inference server returned %d", resp.StatusCode)
	}

	tokens := make(chan Token, 32)
	go func() {
		defer close(tokens)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var tok Token
			if err := json.Unmarshal(line, &tok); err != nil {
				continue
			}
			select {
			case tokens <- tok:
			case <-ctx.Done():
				return
			}
			if tok.Done {
				return
			}
		}
	}()

	return tokens, nil
}

func (c *Client) Health(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://unix/health", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
