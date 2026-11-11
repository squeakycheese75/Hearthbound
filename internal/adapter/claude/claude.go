// Package claude implements a game agent on the Claude Messages API.
package claude

import (
	"context"
	"dnd/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/anthropics/anthropic-sdk-go/v2/packages/param"
	"github.com/anthropics/anthropic-sdk-go/v2/shared/constant"
)

// ErrRefused is returned when the model declines and no fallback answered.
var ErrRefused = errors.New("the model declined to continue")

// Config selects the model and request limits.
type Config struct {
	Model     string
	MaxTokens int64
	// APIKey overrides the environment/profile credentials (per-player keys).
	APIKey string
}

// Request is one agent call in domain terms.
type Request struct {
	Seat string
	// Model overrides the client's default model for this call.
	Model   string
	System  string
	Tools   []domain.ToolSpec
	History []domain.AgentMessage
	Effort  string
	OnText  func(string)
}

// Response is the agent's message plus usage and the model that answered
// (which differs from the request when a refusal fallback stepped in).
type Response struct {
	Message domain.AgentMessage
	Usage   domain.TokenUsage
	Model   string
}

// Client talks to Claude.
type Client struct {
	api anthropic.Client
	cfg Config
	log *slog.Logger
}

// New builds a client. Credentials come from the environment or `ant auth login`.
func New(cfg Config, log *slog.Logger, opts ...option.RequestOption) *Client {
	if cfg.Model == "" {
		cfg.Model = "claude-opus-5-5"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 32000
	}
	if cfg.APIKey != "" {
		opts = append([]option.RequestOption{option.WithAPIKey(cfg.APIKey)}, opts...)
	}
	return &Client{api: anthropic.NewClient(opts...), cfg: cfg, log: log}
}

// Complete streams one model turn, forwarding text deltas to req.OnText.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	params, err := c.params(req)
	if err != nil {
		return Response{}, err
	}
	start := time.Now()
	res, err := c.stream(ctx, params, req.OnText)
	if err != nil {
		c.log.Warn("claude call failed", "seat", req.Seat, "model", params.Model, "took", time.Since(start).Round(time.Millisecond), "err", err)
		return Response{}, err
	}
	c.log.Debug("claude call", "seat", req.Seat, "model", params.Model, "answered_by", res.Model, "effort", req.Effort, "messages", len(params.Messages),
		"tools_offered", len(params.Tools), "took", time.Since(start).Round(time.Millisecond), "stop", res.stop,
		"tool_calls", len(res.Message.ToolCalls), "in", res.Usage.Input, "out", res.Usage.Output,
		"cache_read", res.Usage.CacheRead, "cache_write", res.Usage.CacheWrite)
	return res.Response, nil
}

type streamed struct {
	Response
	stop anthropic.BetaStopReason
}

func (c *Client) stream(ctx context.Context, params anthropic.BetaMessageNewParams, onText func(string)) (streamed, error) {
	stream := c.api.Beta.Messages.NewStreaming(ctx, params)
	msg := anthropic.BetaMessage{}
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return streamed{}, fmt.Errorf("accumulate stream: %w", err)
		}
		forwardText(ev, onText)
	}
	if err := stream.Err(); err != nil {
		return streamed{}, fmt.Errorf("claude stream: %w", err)
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return streamed{}, fmt.Errorf("%w (%s)", ErrRefused, msg.StopDetails.Category)
	}
	usage := domain.TokenUsage{
		Input: int(msg.Usage.InputTokens), Output: int(msg.Usage.OutputTokens),
		CacheRead: int(msg.Usage.CacheReadInputTokens), CacheWrite: int(msg.Usage.CacheCreationInputTokens),
	}
	out, err := toDomain(msg)
	if err != nil {
		return streamed{}, err
	}
	return streamed{Response: Response{Message: out, Usage: usage, Model: string(msg.Model)}, stop: msg.StopReason}, nil
}

func forwardText(ev anthropic.BetaRawMessageStreamEventUnion, onText func(string)) {
	if onText == nil {
		return
	}
	delta, ok := ev.AsAny().(anthropic.BetaRawContentBlockDeltaEvent)
	if !ok {
		return
	}
	if t, ok := delta.Delta.AsAny().(anthropic.BetaTextDelta); ok && t.Text != "" {
		onText(t.Text)
	}
}

func (c *Client) params(req Request) (anthropic.BetaMessageNewParams, error) {
	tools, err := toTools(req.Tools)
	if err != nil {
		return anthropic.BetaMessageNewParams{}, err
	}
	messages, err := toMessages(req.History)
	if err != nil {
		return anthropic.BetaMessageNewParams{}, err
	}
	model := c.cfg.Model
	if req.Model != "" {
		model = req.Model
	}
	p := anthropic.BetaMessageNewParams{
		Model:     model,
		MaxTokens: c.cfg.MaxTokens,
		// The system prompt is frozen per seat, so tools + system cache together.
		System:   []anthropic.BetaTextBlockParam{{Text: req.System, CacheControl: anthropic.NewBetaCacheControlEphemeralParam()}},
		Tools:    tools,
		Messages: messages,
		// Automatic breakpoint on the newest message caches the append-only history.
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
	}
	if hasFallbacks(model) {
		// Route policy declines to a fallback model instead of stalling the game.
		p.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
		p.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
	}
	if req.Effort != "" {
		p.OutputConfig = anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffort(req.Effort)}
	}
	return p, nil
}

// hasFallbacks reports whether the API offers server-side refusal fallbacks
// for a model. Haiku has none: a refusal there just fails that agent's turn,
// and the game copes (the character hesitates or the engine picks its move).
func hasFallbacks(model string) bool {
	return !strings.HasPrefix(model, "claude-haiku")
}

type schema struct {
	Properties any      `json:"properties"`
	Required   []string `json:"required"`
}

func toTools(specs []domain.ToolSpec) ([]anthropic.BetaToolUnionParam, error) {
	out := make([]anthropic.BetaToolUnionParam, 0, len(specs))
	for _, s := range specs {
		var sc schema
		if err := json.Unmarshal(s.Schema, &sc); err != nil {
			return nil, fmt.Errorf("tool %s schema: %w", s.Name, err)
		}
		tool := anthropic.BetaToolParam{
			Name:        s.Name,
			Description: anthropic.String(s.Description),
			InputSchema: anthropic.BetaToolInputSchemaParam{
				Properties: sc.Properties, Required: sc.Required,
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}
		out = append(out, anthropic.BetaToolUnionParam{OfTool: &tool})
	}
	return out, nil
}

// toMessages converts the transcript, replaying assistant turns byte-for-byte
// from Raw and merging consecutive user turns into one.
func toMessages(history []domain.AgentMessage) ([]anthropic.BetaMessageParam, error) {
	var out []anthropic.BetaMessageParam
	for _, m := range history {
		if m.Role == domain.RoleAssistant {
			a, err := assistantParam(m)
			if err != nil {
				return nil, err
			}
			out = append(out, a)
			continue
		}
		blocks := userBlocks(m)
		if n := len(out); n > 0 && out[n-1].OfUser != nil {
			out[n-1].OfUser.Content = append(out[n-1].OfUser.Content, blocks...)
			continue
		}
		out = append(out, anthropic.NewBetaUserMessage(blocks...))
	}
	return out, nil
}

func userBlocks(m domain.AgentMessage) []anthropic.BetaContentBlockParamUnion {
	blocks := make([]anthropic.BetaContentBlockParamUnion, 0, len(m.ToolResults)+1)
	for _, r := range m.ToolResults {
		blocks = append(blocks, anthropic.NewBetaToolResultBlock(r.CallID, r.Content, r.IsError))
	}
	if m.Text != "" {
		blocks = append(blocks, anthropic.NewBetaTextBlock(m.Text))
	}
	return blocks
}

func assistantParam(m domain.AgentMessage) (anthropic.BetaMessageParam, error) {
	if len(m.Raw) == 0 {
		return anthropic.BetaMessageParam{}, fmt.Errorf("%w: assistant message without raw content", domain.ErrInvalid)
	}
	p := param.Override[anthropic.BetaAssistantMessageParam](json.RawMessage(m.Raw))
	return anthropic.BetaMessageParam{OfAssistant: &p}, nil
}

func toDomain(msg anthropic.BetaMessage) (domain.AgentMessage, error) {
	raw, err := json.Marshal(msg.ToParam())
	if err != nil {
		return domain.AgentMessage{}, fmt.Errorf("encode assistant turn: %w", err)
	}
	out := domain.AgentMessage{Role: domain.RoleAssistant, Raw: raw}
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.BetaTextBlock:
			out.Text += b.Text
		case anthropic.BetaToolUseBlock:
			out.ToolCalls = append(out.ToolCalls, domain.ToolCall{ID: b.ID, Name: b.Name, Input: json.RawMessage(b.JSON.Input.Raw())})
		}
	}
	return out, nil
}
