// Package speech reads the story aloud with an AI voice, paid for by the
// listening player's own key.
package speech

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"slices"
	"strings"
)

// Voice turns text into audio for an account, using that account's key.
type Voice interface {
	Speak(ctx context.Context, accountID, text, voice, instructions string) ([]byte, error)
}

// Games loads a game, to find out who is speaking.
type Games interface {
	Load(ctx context.Context, id string) (*domain.Game, error)
}

// UseCase narrates lines.
type UseCase struct {
	voice Voice
	games Games
}

// New builds the use case.
func New(voice Voice, games Games) *UseCase {
	return &UseCase{voice: voice, games: games}
}

// Input is one line to read.
type Input struct {
	AccountID string
	Text      string
	Voice     string
	Style     domain.SpeechStyle
	// GameID and Speaker (a creature or NPC ID) say who says a character
	// line, so it can be performed in character. Both optional.
	GameID  string `json:"gameId"`
	Speaker string `json:"speaker"`
}

// Output is the spoken line.
type Output struct {
	Audio       []byte
	ContentType string
}

// Performance directions per style. Fixed text: nothing a player wrote goes
// into the instructions.
var directions = map[domain.SpeechStyle]string{
	domain.StyleNarrator: "You are the narrator of a fantasy tabletop adventure, reading by candlelight. " +
		"Warm, measured and atmospheric; let tension build in danger and soften in quiet moments.",
	domain.StyleCharacter: "Speak this line in character as an adventurer in a fantasy tale. " +
		"Natural and expressive, matching the emotion of the words.",
}

// Execute reads a line aloud.
func (u *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	text := strings.TrimSpace(in.Text)
	how, ok := directions[in.Style]
	switch {
	case text == "":
		return Output{}, fmt.Errorf("%w: nothing to read", domain.ErrInvalid)
	case len([]rune(text)) > domain.MaxSpeechChars:
		return Output{}, fmt.Errorf("%w: at most %d characters at a time", domain.ErrInvalid, domain.MaxSpeechChars)
	case !slices.Contains(domain.SpeechVoices, in.Voice):
		return Output{}, fmt.Errorf("%w: unknown voice %q", domain.ErrInvalid, in.Voice)
	case !ok:
		return Output{}, fmt.Errorf("%w: unknown style %q", domain.ErrInvalid, in.Style)
	}
	if in.Style == domain.StyleCharacter && in.Speaker != "" {
		who, err := u.persona(ctx, in)
		if err != nil {
			return Output{}, err
		}
		how = who + " " + how
	}
	audio, err := u.voice.Speak(ctx, in.AccountID, text, in.Voice, how)
	if err != nil {
		return Output{}, fmt.Errorf("speak: %w", err)
	}
	return Output{Audio: audio, ContentType: "audio/mpeg"}, nil
}

// persona describes the speaker for the voice: a hero's race and class, a
// monster, or an NPC's role and attitude. Names are left out (players choose
// them); an unknown speaker gets no description.
func (u *UseCase) persona(ctx context.Context, in Input) (string, error) {
	g, err := u.games.Load(ctx, in.GameID)
	if err != nil {
		return "", fmt.Errorf("load game: %w", err)
	}
	if !g.CanView(in.AccountID) {
		return "", fmt.Errorf("%w: game %s", domain.ErrNotFound, in.GameID)
	}
	if c, err := g.Creature(in.Speaker); err == nil {
		return creaturePersona(c), nil
	}
	for _, n := range g.NPCs {
		if n.ID == in.Speaker {
			return npcPersona(n), nil
		}
	}
	return "", nil
}

func creaturePersona(c *domain.Creature) string {
	if c.Kind == domain.KindMonster {
		return monsterVoice
	}
	who := strings.TrimSpace(c.Gender.Word() + " " + strings.ToLower(c.Race) + " " + strings.ToLower(c.Class))
	return fmt.Sprintf("You are voicing a %s adventurer.", who)
}

const monsterVoice = "You are voicing a monster: rough, guttural and menacing."

func npcPersona(n *domain.NPC) string {
	role := strings.Join(strings.Fields(n.Role), " ")
	if r := []rune(role); len(r) > maxRole {
		role = string(r[:maxRole])
	}
	who := fmt.Sprintf("You are voicing %s, %s toward the party.", role, n.Attitude)
	if g := n.Gender.Word(); g != "" {
		who += " The speaker is " + g + "."
	}
	if n.Monster {
		who += " " + monsterVoice
	}
	return who
}

// maxRole bounds the DM-written role passed to the voice.
const maxRole = 80
