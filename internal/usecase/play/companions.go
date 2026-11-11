package play

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"encoding/json"
	"fmt"
	"strings"
)

const chooseActionTool = "choose_action"

func companionTools() []domain.ToolSpec {
	return []domain.ToolSpec{spec(chooseActionTool,
		"Choose your combat action for this turn by option number, with a short in-character line.",
		`{"type":"object","properties":{"option":{"type":"integer","minimum":0},"line":{"type":"string"}},"required":["option","line"],"additionalProperties":false}`)}
}

func (s *session) companionRun(c *domain.Creature, message string, handle toolHandler) seatRun {
	return seatRun{
		seat: c.ID, role: domain.RoleCharacter, payer: s.g.Payer(c), actor: c.ID, data: characterPrompt(s.g, c, s.tactics(c)), tools: companionTools(),
		message: message, final: domain.EvSpeech, handle: handle,
	}
}

func noTools(call domain.ToolCall) (string, error) {
	return "", fmt.Errorf("%w: %s cannot be used right now", domain.ErrInvalid, call.Name)
}

func (s *session) lastNarration() string {
	for i := len(s.g.Log) - 1; i >= 0; i-- {
		if e := s.g.Log[i]; e.Type == domain.EvNarration {
			return e.Text
		}
	}
	return "(nothing yet)"
}

// companionCombatChoice asks an AI companion to pick from its legal options.
// It falls back to the engine's tactical choice if the agent fails or picks badly.
func (s *session) companionCombatChoice(ctx context.Context, c *domain.Creature, opts []rules.Option) (rules.Option, string) {
	var b strings.Builder
	fmt.Fprintf(&b, "Combat, round %d. It is your turn. HP %d/%d.\nBattlefield:\n%s\nYour options:\n", s.g.Encounter.Round, c.HP, c.MaxHP, battlefield(s.g))
	for i, o := range opts {
		fmt.Fprintf(&b, "%d. %s\n", i, o.Summary)
	}
	b.WriteString("\nFight the way your class does (see \"How you fight\"): mind which options are melee and which move you. Call choose_action with the option number and a short battle cry or quip. Do not narrate the result.")
	var picked *rules.Option
	var line string
	handle := func(call domain.ToolCall) (string, error) {
		var in struct {
			Option int    `json:"option"`
			Line   string `json:"line"`
		}
		if err := json.Unmarshal(call.Input, &in); err != nil || in.Option < 0 || in.Option >= len(opts) {
			return "", fmt.Errorf("%w: pick an option number from 0 to %d", domain.ErrInvalid, len(opts)-1)
		}
		picked, line = &opts[in.Option], in.Line
		return "Locked in: " + opts[in.Option].Summary, nil
	}
	run := s.companionRun(c, b.String(), handle)
	run.final = domain.EvThinking
	if _, err := s.runAgent(ctx, run); err != nil || picked == nil {
		fallback, _ := rules.MonsterChoice(s.u.roller, s.g, c.ID)
		return fallback, ""
	}
	return *picked, line
}

// tactics is how the character's class fights, for its agent.
func (s *session) tactics(c *domain.Creature) string {
	cls, err := s.u.catalog.Class(c.Class)
	if err != nil || cls.Tactics == "" {
		return "Fight sensibly for your class: tough melee characters hold the front, fragile ones stay back and use ranged attacks, spells and healing."
	}
	return cls.Tactics
}
