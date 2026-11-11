package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Speech calls POST /audio/speech. Each call carries the caller's key, since
// every player pays for their own narration.
type Speech struct {
	model   string
	baseURL string
	http    *http.Client
}

// NewSpeech builds a client; empty values take the defaults
// (gpt-4o-mini-tts, the public API).
func NewSpeech(model, baseURL string) *Speech {
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Speech{model: model, baseURL: baseURL, http: &http.Client{Timeout: 90 * time.Second}}
}

type speechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	Instructions   string `json:"instructions,omitempty"`
	ResponseFormat string `json:"response_format"`
}

// Speak returns MP3 audio of the text read in a voice.
func (s *Speech) Speak(ctx context.Context, key, text, voice, instructions string) ([]byte, error) {
	body, err := json.Marshal(speechRequest{Model: s.model, Input: text, Voice: voice, Instructions: instructions, ResponseFormat: "mp3"})
	if err != nil {
		return nil, fmt.Errorf("encode speech request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build speech request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai speech: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read speech response: %w", err)
	}
	if res.StatusCode == http.StatusOK {
		return raw, nil
	}
	var out response
	if json.Unmarshal(raw, &out) == nil && out.Error != nil {
		return nil, fmt.Errorf("openai speech: HTTP %d: %s (%s)", res.StatusCode, out.Error.Message, out.Error.Code)
	}
	return nil, fmt.Errorf("openai speech: HTTP %d", res.StatusCode)
}
