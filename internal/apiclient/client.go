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
	"net/url"
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

// PRReviews lists the pull-request reviews (diff reports) for a workspace.
func (c *Client) PRReviews(ctx context.Context, workspace string) ([]PRReview, error) {
	u := c.BaseURL + "/api/workspaces/" + url.PathEscape(workspace) + "/pr-reviews"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

	var out listPRReviewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Reviews, nil
}

// ListChats lists the workspace's stored conversations, newest first. The backend
// lists the ones a person browses — chats opened in the app and the ones the CLI
// started — so both surfaces offer the same conversations.
func (c *Client) ListChats(ctx context.Context, workspace string) ([]Chat, error) {
	u := c.BaseURL + "/api/workspaces/" + url.PathEscape(workspace) + "/chats"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

	var out listChatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Chats, nil
}

// GetChat loads one conversation with its full message history, oldest exchange
// first. The slug identifies the conversation on its own, so no workspace is
// needed.
func (c *Client) GetChat(ctx context.Context, chatSlug string) (ChatWithMessages, error) {
	u := c.BaseURL + "/api/chats/" + url.PathEscape(chatSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ChatWithMessages{}, err
	}
	c.auth(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ChatWithMessages{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ChatWithMessages{}, errorFrom(resp)
	}

	var out getChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatWithMessages{}, err
	}
	return out.Chat, nil
}

// AskBuffered runs one ask and returns the whole formatted answer, plus the
// conversation it ran in.
func (c *Client) AskBuffered(ctx context.Context, req AskRequest) (AskResult, error) {
	req.Stream = false
	resp, err := c.postAsk(ctx, req)
	if err != nil {
		return AskResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AskResult{}, errorFrom(resp)
	}

	var out askResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return AskResult{}, err
	}
	return AskResult{Answer: out.Answer, ChatSlug: out.ChatSlug}, nil
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
		case "conversation":
			var c conversationFrame
			if json.Unmarshal([]byte(ev.Data), &c) == nil && c.ChatSlug != "" && h.OnConversation != nil {
				h.OnConversation(c.ChatSlug)
			}
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
