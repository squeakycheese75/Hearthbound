package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Limits on what players type, since their words reach other players' agents.
const (
	// MaxAction bounds one action or line of speech, in characters.
	MaxAction = 500
)

var playerTag = regexp.MustCompile(`(?i)</?\s*player_content\s*>`)

// PlayerText marks text a player wrote (an action, a backstory) as data for
// agent prompts: it removes any player_content tags inside it, so the text
// can't close the block early, and wraps it in <player_content> tags. Agents
// are told text in those tags is fiction, never instructions.
func PlayerText(s string) string {
	return "<player_content>" + strings.TrimSpace(playerTag.ReplaceAllString(s, "")) + "</player_content>"
}

// TypingMode says who may type free-text actions at a table. Everyone else
// plays from the quick choices.
type TypingMode string

// Typing modes.
const (
	TypeEveryone TypingMode = ""        // the default
	TypeLeader   TypingMode = "leader"  // only the adventure's leader types
	TypeChoices  TypingMode = "choices" // everyone plays from quick choices
)

// ParseTypingMode validates a typing mode ("everyone" is the default).
func ParseTypingMode(s string) (TypingMode, error) {
	switch s {
	case "", "everyone":
		return TypeEveryone, nil
	case string(TypeLeader), string(TypeChoices):
		return TypingMode(s), nil
	}
	return "", fmt.Errorf("%w: typing mode %q", ErrUnknown, s)
}

// FallbackChoices are offered when the DM has suggested none.
var FallbackChoices = []string{
	"Look around carefully",
	"Talk to the nearest stranger",
	"Ask my companions what they think",
	"Search for anything hidden",
}

// MayType reports whether an account may type its own actions here.
func (g *Game) MayType(accountID string) bool {
	switch g.Typing {
	case TypeChoices:
		return false
	case TypeLeader:
		return accountID == g.LeaderID
	}
	return true
}

// ChoicesNow are the quick choices on offer: the DM's, else the fallbacks.
func (g *Game) ChoicesNow() []string {
	if len(g.Choices) > 0 {
		return g.Choices
	}
	return FallbackChoices
}
