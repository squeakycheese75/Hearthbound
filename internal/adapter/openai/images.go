// Package openai generates illustrations with the OpenAI Images API.
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config selects the model and output.
type Config struct {
	APIKey  string
	Model   string // default gpt-image-2
	Quality string // low | medium | high (default medium)
	BaseURL string // default https://api.openai.com/v1
}

// Images calls POST /images/generations.
type Images struct {
	cfg  Config
	http *http.Client
}

// New builds a client. Images take a while, so the timeout is generous.
func New(cfg Config) *Images {
	if cfg.Model == "" {
		cfg.Model = "gpt-image-2"
	}
	if cfg.Quality == "" {
		cfg.Quality = "medium"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	return &Images{cfg: cfg, http: &http.Client{Timeout: 4 * time.Minute}}
}

type request struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	Size              string `json:"size"`
	Quality           string `json:"quality"`
	OutputFormat      string `json:"output_format"`
	OutputCompression int    `json:"output_compression"`
	N                 int    `json:"n"`
}

type response struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Generate returns a WebP image for the prompt.
func (c *Images) Generate(ctx context.Context, prompt, size string) ([]byte, error) {
	body, err := json.Marshal(request{
		Model: c.cfg.Model, Prompt: prompt, Size: size, Quality: c.cfg.Quality,
		OutputFormat: "webp", OutputCompression: 85, N: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("encode image request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build image request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai images: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	return decode(res)
}

func decode(res *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("read image response: %w", err)
	}
	var out response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("openai images: HTTP %d: undecodable response: %w", res.StatusCode, err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("openai images: HTTP %d: %s (%s)", res.StatusCode, out.Error.Message, out.Error.Code)
	}
	if res.StatusCode != http.StatusOK || len(out.Data) == 0 || out.Data[0].B64JSON == "" {
		return nil, fmt.Errorf("openai images: HTTP %d: %w", res.StatusCode, errNoImage)
	}
	img, err := base64.StdEncoding.DecodeString(out.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

var errNoImage = errors.New("no image in response")
