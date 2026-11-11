package main

import (
	"context"
	"dnd/internal/adapter/claude"
	"dnd/internal/adapter/openai"
	"dnd/internal/domain"
	"dnd/internal/usecase/accounts"
	"dnd/internal/usecase/play"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// completer is anything that can run one agent turn: the Claude client or the offline demo.
type completer interface {
	Complete(ctx context.Context, req claude.Request) (claude.Response, error)
}

// keyring finds the API key a payer's calls should use. In -local mode it falls
// back to the server's own environment keys when the account has none on file.
type keyring struct {
	accounts *accounts.UseCase
	local    bool
}

var envKeys = map[domain.KeyKind]string{domain.KeyAnthropic: "ANTHROPIC_API_KEY", domain.KeyOpenAI: "OPENAI_API_KEY"}

func (k keyring) key(ctx context.Context, payer string, kind domain.KeyKind) (string, error) {
	key, err := k.accounts.Credential(ctx, payer, kind)
	if err == nil {
		return key, nil
	}
	if k.local && errors.Is(err, domain.ErrNoKey) {
		if env := os.Getenv(envKeys[kind]); env != "" {
			return env, nil
		}
	}
	return "", err
}

// voiceBridge reads lines aloud on the listener's own OpenAI key.
type voiceBridge struct {
	keys keyring
	tts  *openai.Speech
}

func (v voiceBridge) Speak(ctx context.Context, accountID, text, voice, instructions string) ([]byte, error) {
	key, err := v.keys.key(ctx, accountID, domain.KeyOpenAI)
	if err != nil {
		return nil, err
	}
	audio, err := v.tts.Speak(ctx, key, text, voice, instructions)
	if err != nil {
		return nil, fmt.Errorf("narrate: %w", err)
	}
	return audio, nil
}

// tableAI tells newgame whether an account has registered the AI that runs a
// table's agents. The scripted demo needs no key.
type tableAI struct {
	keys keyring
	demo bool
}

func (t tableAI) Ready(ctx context.Context, accountID string) bool {
	if t.demo {
		return true
	}
	_, err := t.keys.key(ctx, accountID, domain.KeyAnthropic)
	return !errors.Is(err, domain.ErrNoKey) // over today's cap still counts as registered
}

// agentBridge adapts Claude to the play use case's Agent port, using the
// paying account's own key and recording what it spent.
type agentBridge struct {
	keys  keyring
	model string // default for calls that name none
	log   *slog.Logger
	demo  completer // when set, every call is answered offline

	mu      sync.Mutex
	clients map[string]*claude.Client
}

func (a *agentBridge) client(key string) *claude.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.clients == nil {
		a.clients = map[string]*claude.Client{}
	}
	c, ok := a.clients[key]
	if !ok {
		c = claude.New(claude.Config{Model: a.model, APIKey: key}, a.log)
		a.clients[key] = c
	}
	return c
}

func (a *agentBridge) Act(ctx context.Context, t play.Turn) (play.Reply, error) {
	model := a.modelFor(ctx, t)
	req := claude.Request{Seat: t.Seat, Model: model, System: t.System, Tools: t.Tools, History: t.History, Effort: t.Effort, OnText: t.OnText}
	c := a.demo
	if c == nil {
		key, err := a.keys.key(ctx, t.Payer, domain.KeyAnthropic)
		if err != nil {
			return play.Reply{}, err
		}
		c = a.client(key)
	}
	res, err := c.Complete(ctx, req)
	if err != nil {
		return play.Reply{}, err
	}
	u := res.Usage
	if err := a.keys.accounts.RecordUsage(ctx, t.Payer, u.Billed()); err != nil {
		a.log.Warn("usage not recorded", "payer", t.Payer, "err", err)
	}
	return play.Reply{Message: res.Message, Usage: res.Usage, Model: answeredBy(res.Model, model, a.demo != nil)}, nil
}

// modelFor picks the model for a call: the role's, except that a character's
// agent runs on the model its paying player chose in Settings.
func (a *agentBridge) modelFor(ctx context.Context, t play.Turn) string {
	if t.Role != domain.RoleCharacter {
		return t.Model
	}
	chosen, err := a.keys.accounts.AgentModel(ctx, t.Payer)
	if err != nil || chosen == "" {
		return t.Model
	}
	return chosen
}

func answeredBy(reported, requested string, demo bool) string {
	switch {
	case demo:
		return "scripted demo"
	case reported != "":
		return reported
	}
	return requested
}

// painter generates images with the paying account's OpenAI key.
type painter struct {
	keys    keyring
	model   string
	quality string
}

func (p painter) Ready(ctx context.Context, payer string) bool {
	_, err := p.keys.key(ctx, payer, domain.KeyOpenAI)
	return err == nil
}

func (p painter) Generate(ctx context.Context, payer, prompt, size string) ([]byte, error) {
	key, err := p.keys.key(ctx, payer, domain.KeyOpenAI)
	if err != nil {
		return nil, err
	}
	return openai.New(openai.Config{APIKey: key, Model: p.model, Quality: p.quality}).Generate(ctx, prompt, size)
}
