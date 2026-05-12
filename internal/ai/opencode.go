package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Session struct {
	ID string `json:"id"`
}

type Part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type messageResponse struct {
	Info  messageInfo `json:"info"`
	Parts []Part      `json:"parts"`
}

type messageInfo struct {
	ID string `json:"id"`
}

type createSessionBody struct {
	Title string `json:"title"`
}

type sendMessageBody struct {
	Parts []Part `json:"parts"`
}

func NewClient() *Client {
	return &Client{
		baseURL: "http://127.0.0.1:4096",
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func NewClientWithURL(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/global/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode server at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opencode server at %s returned status %d", c.baseURL, resp.StatusCode)
	}
	return nil
}

func (c *Client) CreateSession(ctx context.Context) (*Session, error) {
	body := createSessionBody{Title: "plums chat"}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal session body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("opencode server returned status %d", resp.StatusCode)
	}
	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Client) SendMessage(ctx context.Context, sessionID, text string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		b := sendMessageBody{
			Parts: []Part{{Type: "text", Text: text}},
		}
		data, err := json.Marshal(b)
		if err != nil {
			select {
			case <-ctx.Done():
			case out <- fmt.Sprintf("\n⚠️  marshal error: %v\n", err):
			}
			return
		}
		req, err := http.NewRequestWithContext(ctx, "POST",
			c.baseURL+"/session/"+url.PathEscape(sessionID)+"/message",
			bytes.NewReader(data))
		if err != nil {
			select {
			case <-ctx.Done():
			case out <- fmt.Sprintf("\n⚠️  request error: %v\n", err):
			}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case out <- "\n⚠️  Opencode server not available (is `opencode serve` running?)\n":
			}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			select {
			case <-ctx.Done():
			case out <- fmt.Sprintf("\n⚠️  opencode server returned status %d\n", resp.StatusCode):
			}
			return
		}
		var msgResp messageResponse
		if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
			select {
			case <-ctx.Done():
			case out <- fmt.Sprintf("\n⚠️  decode error: %v\n", err):
			}
			return
		}
		for _, part := range msgResp.Parts {
			if part.Type == "text" && part.Text != "" {
				for _, r := range part.Text {
					select {
					case <-ctx.Done():
						return
					case out <- string(r):
					}
					time.Sleep(3 * time.Millisecond)
				}
			}
		}
	}()
	return out
}
