package play

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

// toolHandler runs one tool call and returns the result text for the agent.
type toolHandler func(call domain.ToolCall) (string, error)

// seatRun describes one agent turn.
type seatRun struct {
	seat  string
	payer string
	actor string
	// role is the agent definition supplying model, effort and allowed tools.
	role string
	// prompt names the role whose template is the system prompt (default: role),
	// rendered with data. The DM's recap reuses the DM's prompt to stay cached.
	prompt  string
	data    any
	tools   []domain.ToolSpec
	message string
	final   domain.EventType
	handle  toolHandler
	// quiet suppresses streaming, for helper calls whose text players should not see.
	quiet bool
	// ledger names the token ledger entry when it differs from the seat (the
	// DM's chapter recap is billed as "recap").
	ledger string
}

func (r seatRun) ledgerKey() string {
	if r.ledger != "" {
		return r.ledger
	}
	return r.seat
}

// runAgent appends the user message, then loops agent replies and tool results
// until the agent stops calling tools or the step budget runs out. It returns
// everything the agent said.
func (s *session) runAgent(ctx context.Context, run seatRun) (string, error) {
	g := s.g
	turn, err := s.turnFor(run)
	if err != nil {
		return "", err
	}
	g.Transcripts[run.seat] = append(g.Transcripts[run.seat], domain.AgentMessage{Role: domain.RoleUser, Text: run.message})
	var texts []string
	if !run.quiet {
		s.spoke = false // a quiet helper builds on what the previous run said
	}
	for step := range s.u.cfg.MaxToolSteps {
		s.emit(domain.Event{Type: domain.EvThinking, Actor: run.actor})
		start := s.u.clock.Now()
		turn.History = g.Transcripts[run.seat]
		reply, err := s.u.agent.Act(ctx, turn)
		took := s.u.clock.Now().Sub(start)
		if err != nil {
			s.u.log.Warn("agent call failed", "game", g.ID, "seat", run.seat, "step", step, "took", took, "err", err)
			return strings.Join(texts, "\n\n"), fmt.Errorf("%s agent: %w", run.seat, err)
		}
		g.Charge(run.ledgerKey(), run.payer, reply.Model, reply.Usage)
		s.measure(run, reply.Usage)
		msg := reply.Message
		s.u.log.Debug("agent call", "game", g.ID, "seat", run.seat, "role", run.role, "model", reply.Model, "ledger", run.ledgerKey(), "payer", run.payer, "step", step, "took", took,
			"history", len(g.Transcripts[run.seat]), "tools", toolNames(msg.ToolCalls), "text_chars", len(msg.Text),
			"tokens_in", reply.Usage.Input, "tokens_out", reply.Usage.Output, "cache_read", reply.Usage.CacheRead, "cache_write", reply.Usage.CacheWrite,
			"prompt", reply.Usage.Input+reply.Usage.CacheRead+reply.Usage.CacheWrite, "billed", reply.Usage.Billed(), "seat_billed", g.Spend[run.ledgerKey()].Total.Billed())
		g.Transcripts[run.seat] = append(g.Transcripts[run.seat], msg)
		if t := strings.TrimSpace(msg.Text); t != "" {
			texts = append(texts, t)
			s.spoke = true
			if !run.quiet { // quiet helpers (choices, recaps) never reach the players
				s.emit(domain.Event{Type: run.final, Actor: run.actor, Text: t})
			}
		}
		if len(msg.ToolCalls) == 0 {
			return strings.Join(texts, "\n\n"), nil
		}
		results := s.runTools(run.seat, msg.ToolCalls, run.handle)
		g.Transcripts[run.seat] = append(g.Transcripts[run.seat], domain.AgentMessage{Role: domain.RoleUser, ToolResults: results})
		if s.endTurn {
			s.endTurn = false
			return strings.Join(texts, "\n\n"), nil
		}
	}
	s.u.log.Warn("agent hit its tool-step budget", "game", g.ID, "seat", run.seat, "steps", s.u.cfg.MaxToolSteps)
	return strings.Join(texts, "\n\n"), nil
}

func toolNames(calls []domain.ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, c := range calls {
		names = append(names, c.Name)
	}
	return names
}

// turnFor builds the request for a run from its role's definition. History
// is filled in on each step.
func (s *session) turnFor(run seatRun) (Turn, error) {
	def := s.u.roles.Role(run.role)
	prompt := run.prompt
	if prompt == "" {
		prompt = run.role
	}
	system, err := s.u.render(prompt, run.data)
	if err != nil {
		return Turn{}, err
	}
	t := Turn{Seat: run.seat, Role: run.role, Model: def.Model, Payer: run.payer, System: system, Tools: allowed(run.tools, def.Tools), Effort: def.Effort}
	if !run.quiet {
		t.OnText = func(delta string) {
			s.emit(domain.Event{Type: domain.EvNarrationDelta, Actor: run.actor, Text: delta, Stream: run.seat})
		}
	}
	return t, nil
}

// allowed keeps the tools a role's definition names, in the engine's order.
// A definition that names none gets them all.
func allowed(all []domain.ToolSpec, names []string) []domain.ToolSpec {
	if len(names) == 0 {
		return all
	}
	out := make([]domain.ToolSpec, 0, len(names))
	for _, t := range all {
		if slices.Contains(names, t.Name) {
			out = append(out, t)
		}
	}
	return out
}

// render fills in a role's prompt template.
func (u *UseCase) render(role string, data any) (string, error) {
	t, err := u.template(role)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", fmt.Errorf("render the %s prompt: %w", role, err)
	}
	return b.String(), nil
}

// template parses a role's prompt once and keeps it.
func (u *UseCase) template(role string) (*template.Template, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if t, ok := u.templates[role]; ok {
		return t, nil
	}
	t, err := template.New(role).Parse(u.roles.Role(role).Prompt)
	if err != nil {
		return nil, fmt.Errorf("parse the %s prompt: %w", role, err)
	}
	u.templates[role] = t
	return t, nil
}

func (s *session) runTools(seat string, calls []domain.ToolCall, handle toolHandler) []domain.ToolResult {
	results := make([]domain.ToolResult, 0, len(calls))
	for _, call := range calls {
		out, err := handle(call)
		s.u.log.Debug("tool call", "game", s.g.ID, "seat", seat, "tool", call.Name, "call_id", call.ID, "ok", err == nil, "err", err)
		if err != nil {
			results = append(results, domain.ToolResult{CallID: call.ID, Content: "Error: " + err.Error(), IsError: true})
			continue
		}
		results = append(results, domain.ToolResult{CallID: call.ID, Content: out})
	}
	return results
}

// measure records how big a seat's prompt has grown. Quiet helper calls are
// throwaway conversations, so they never count towards a chapter break.
func (s *session) measure(run seatRun, u domain.TokenUsage) {
	if run.quiet {
		return
	}
	if s.g.Context == nil {
		s.g.Context = map[string]int{}
	}
	s.g.Context[run.seat] = u.Input + u.CacheRead + u.CacheWrite
}
