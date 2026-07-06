// Package apiclient is the HTTP + SSE client for the Abstraction backend's CLI
// endpoints.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Client talks to the backend over HTTP, authenticated with an API key.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New builds a Client. It sets no client-side timeout — callers bound requests
// via context (the backend caps a run at 5 minutes).
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{},
	}
}

// Workspaces lists the authenticated user's workspaces.
func (c *Client) Workspaces(ctx context.Context) ([]Workspace, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/workspaces", nil)
	if err != nil {
		return nil, err
	}
	c.auth(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFrom(resp)
	}

	var out listWorkspacesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Workspaces, nil
}

// AskBuffered runs one ask and returns the whole formatted answer.
func (c *Client) AskBuffered(ctx context.Context, req AskRequest) (string, error) {
	req.Stream = false
	resp, err := c.postAsk(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errorFrom(resp)
	}

	var out askResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Answer, nil
}

// AskStream runs one ask and delivers formatted output/status frames to h as
// they arrive. A streamed `error` frame is returned as an *APIError.
func (c *Client) AskStream(ctx context.Context, req AskRequest, h StreamHandlers) error {
	req.Stream = true
	resp, err := c.postAsk(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorFrom(resp)
	}

	r := newSSEReader(resp.Body)
	for {
		ev, err := r.next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		switch ev.Type {
		case "stream_end":
			return nil
		case "output":
			var o outputFrame
			if json.Unmarshal([]byte(ev.Data), &o) == nil && h.OnOutput != nil {
				h.OnOutput(o.Text)
			}
		case "status":
			var m messageFrame
			if json.Unmarshal([]byte(ev.Data), &m) == nil && h.OnStatus != nil {
				h.OnStatus(m.Message)
			}
		case "error":
			var m messageFrame
			_ = json.Unmarshal([]byte(ev.Data), &m)
			msg := m.Message
			if msg == "" {
				msg = "the agent run failed"
			}
			return &APIError{Message: msg}
		}
	}
}

func (c *Client) postAsk(ctx context.Context, req AskRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/cli/ask", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.auth(httpReq)
	return c.HTTP.Do(httpReq)
}

func (c *Client) auth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
}

func errorFrom(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return &APIError{Status: resp.StatusCode, Message: msg}
}
