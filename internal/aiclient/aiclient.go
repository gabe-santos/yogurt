// Package aiclient sends one request to an OpenAI-compatible chat-completions
// endpoint and returns the text of its reply. It is the model client behind
// Gists (ADR-0018): unlike the feed fetch client, it dials any address,
// including private and loopback ones, because a Reader's own local model —
// Ollama, LM Studio — lives there, and it carries its own timeout and
// response-size cap rather than the feed client's.
package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// systemPrompt is the one instruction every Gist request carries. It is
// fixed, not configurable (GABE-201).
const systemPrompt = "Condense the text you are given into 3 to 5 plain-text bullet points, each one short sentence, in the same language as the text. Reply with only the bullets, one per line, and nothing else: no heading, preamble, or closing remark."

// inputCharLimit caps the text sent to the model, so one Gist request
// cannot run up a large bill or exceed a small model's context.
const inputCharLimit = 20000

// maxReplyTokens caps the provider's own reply length.
const maxReplyTokens = 400

// maxReplyBytes caps how much of a response this client will read, so a
// misbehaving or hostile endpoint cannot exhaust memory.
const maxReplyBytes = 1 << 20 // 1 MiB

// DefaultTimeout is how long one Gist request waits for a reply before
// giving up, when New is given no timeout of its own.
const DefaultTimeout = 30 * time.Second

// Client sends chat-completion requests to whatever endpoint the Reader
// configured.
type Client struct {
	http *http.Client
}

// New builds a Client that gives up after timeout, or DefaultTimeout when
// timeout is zero or negative.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{http: &http.Client{Timeout: timeout}}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Gist asks the model at endpoint to condense text, authenticating with
// apiKey as a bearer credential when it is not empty. An empty or
// unparseable reply is a failure, same as a non-2xx status or a timeout.
func (c *Client) Gist(ctx context.Context, endpoint, model, apiKey, text string) (string, error) {
	if len(text) > inputCharLimit {
		text = text[:inputCharLimit]
	}

	payload, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
		MaxTokens: maxReplyTokens,
	})
	if err != nil {
		return "", fmt.Errorf("encode Gist request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build Gist request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("reach the Gist model: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes+1))
	if err != nil {
		return "", fmt.Errorf("read Gist reply: %w", err)
	}
	if len(body) > maxReplyBytes {
		return "", errors.New("the Gist model's reply was too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("the Gist model answered with status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("the Gist model's reply could not be parsed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("the Gist model's reply carried no choices")
	}
	reply := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if reply == "" {
		return "", errors.New("the Gist model's reply was empty")
	}
	return reply, nil
}
