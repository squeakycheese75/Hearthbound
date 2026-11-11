package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Auth errors.
var (
	// ErrUnauthorized means the caller is not signed in or the credentials are wrong.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden means the caller is signed in but may not do this.
	ErrForbidden = errors.New("forbidden")
	// ErrNoKey means the paying account has not added the API key this needs.
	ErrNoKey = errors.New("no API key on file")
	// ErrOverBudget means the paying account hit its daily token cap.
	ErrOverBudget = errors.New("daily token cap reached")
	// ErrTooManyAttempts means too many failed sign-ins; try again later.
	ErrTooManyAttempts = errors.New("too many attempts")
)

// KeyKind names a third-party credential an account can bring.
type KeyKind string

// Supported keys.
const (
	KeyAnthropic KeyKind = "anthropic"
	KeyOpenAI    KeyKind = "openai"
)

// ParseKeyKind validates a key kind.
func ParseKeyKind(s string) (KeyKind, error) {
	switch KeyKind(s) {
	case KeyAnthropic, KeyOpenAI:
		return KeyKind(s), nil
	}
	return "", fmt.Errorf("%w: key kind %q", ErrUnknown, s)
}

// DefaultDailyTokenCap bounds what one account's agents can spend per day.
const DefaultDailyTokenCap = 2_000_000

// DefaultSponsorCap is what a sponsor pays for per player per day unless
// they choose otherwise: a few evenings of play.
const DefaultSponsorCap = 500_000

// MaxTokenCap bounds any daily cap.
const MaxTokenCap = 100_000_000

// Sponsored reports whether someone else pays for this player's agents.
func (a Account) Sponsored() bool { return a.SponsoredBy != "" && a.SponsorCap > 0 }

// Account is a player (and, for admins, the server operator).
type Account struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	PasswordHash  string `json:"-"`
	Admin         bool   `json:"admin"`
	DailyTokenCap int    `json:"dailyTokenCap"`
	// AgentModel is the model this player's characters' agents run on; empty
	// means the character role's default.
	AgentModel string `json:"agentModel,omitempty"`
	// SponsoredBy is the admin whose Anthropic key pays for this player's
	// agents while they have no key of their own; empty when nobody does.
	SponsoredBy string `json:"sponsoredBy,omitempty"`
	// SponsorCap is how many tokens a day the sponsor pays for.
	SponsorCap int       `json:"sponsorCap,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,24}$`)

// ValidateCredentials checks a new username and password.
func ValidateCredentials(username, password string) error {
	if !usernameRe.MatchString(username) {
		return fmt.Errorf("%w: username must be 3-24 letters, digits, dots, dashes or underscores", ErrInvalid)
	}
	if len(password) < 10 || len(password) > 128 {
		return fmt.Errorf("%w: password must be 10-128 characters", ErrInvalid)
	}
	return nil
}

// Session is a signed-in browser.
type Session struct {
	Token     string    `json:"-"`
	AccountID string    `json:"accountId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Invite lets one new person register.
type Invite struct {
	Code      string `json:"code"`
	CreatedBy string `json:"createdBy"`
	UsedBy    string `json:"usedBy,omitempty"`
	// Sponsored invites make sponsored accounts: the realm pays for their AI.
	Sponsored bool      `json:"sponsored,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// KeyInfo describes a stored key without revealing it.
type KeyInfo struct {
	Kind  KeyKind `json:"kind"`
	Set   bool    `json:"set"`
	Last4 string  `json:"last4,omitempty"`
}

// Last4 returns the final four characters of a secret, for display.
func Last4(secret string) string {
	if len(secret) <= 4 {
		return ""
	}
	return secret[len(secret)-4:]
}
