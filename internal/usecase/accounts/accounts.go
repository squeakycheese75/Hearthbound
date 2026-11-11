// Package accounts handles invites, sign-in, bring-your-own API keys and spend caps.
package accounts

import (
	"context"
	"dnd/internal/domain"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AccountStore persists accounts.
type AccountStore interface {
	CountAccounts(ctx context.Context) (int, error)
	CreateAccount(ctx context.Context, a domain.Account) error
	Account(ctx context.Context, id string) (domain.Account, error)
	AccountByUsername(ctx context.Context, username string) (domain.Account, error)
	SetDailyTokenCap(ctx context.Context, id string, tokenCap int) error
	SetAgentModel(ctx context.Context, id, model string) error
	Accounts(ctx context.Context) ([]domain.Account, error)
	SetSponsor(ctx context.Context, id, sponsorID string, tokenCap int) error
}

// SessionStore persists sign-ins.
type SessionStore interface {
	CreateSession(ctx context.Context, s domain.Session) error
	Session(ctx context.Context, token string) (domain.Session, error)
	DeleteSession(ctx context.Context, token string) error
}

// InviteStore persists invites. RegisterWithInvite must consume the invite and
// create the account atomically.
type InviteStore interface {
	CreateInvite(ctx context.Context, inv domain.Invite) error
	Invites(ctx context.Context, createdBy string) ([]domain.Invite, error)
	Invite(ctx context.Context, code string) (domain.Invite, error)
	RegisterWithInvite(ctx context.Context, code string, a domain.Account) error
}

// KeyStore persists encrypted API keys.
type KeyStore interface {
	PutKey(ctx context.Context, accountID string, kind domain.KeyKind, sealed []byte, last4 string) error
	Key(ctx context.Context, accountID string, kind domain.KeyKind) (sealed []byte, last4 string, err error)
	DeleteKey(ctx context.Context, accountID string, kind domain.KeyKind) error
}

// UsageStore tracks tokens spent per account per day.
type UsageStore interface {
	AddUsage(ctx context.Context, accountID, day string, tokens int) error
	Usage(ctx context.Context, accountID, day string) (int, error)
}

// Store is everything the use case persists.
type Store interface {
	AccountStore
	SessionStore
	InviteStore
	KeyStore
	UsageStore
}

// Hasher hashes and checks passwords.
type Hasher interface {
	Hash(password string) (string, error)
	Matches(hash, password string) bool
}

// Cipher encrypts secrets at rest.
type Cipher interface {
	Seal(plaintext []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

// Tokens makes unguessable random strings.
type Tokens interface {
	New() string
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// SessionTTL is how long a sign-in lasts.
const SessionTTL = 30 * 24 * time.Hour

// dummyHash keeps login timing similar for unknown usernames.
const dummyHash = "$2a$12$CwTycUXWue0Thq9StjUM0uJ8.Xh3Ug3Ou2N3OAzrM6bGKb9vY9cNa"

// UseCase manages accounts.
type UseCase struct {
	store  Store
	hash   Hasher
	cipher Cipher
	tokens Tokens
	clock  Clock
}

// New wires the use case.
func New(store Store, hash Hasher, cipher Cipher, tokens Tokens, clock Clock) *UseCase {
	return &UseCase{store: store, hash: hash, cipher: cipher, tokens: tokens, clock: clock}
}

// Bootstrap creates the first admin account if the server has none. It returns
// whether an account was created.
func (u *UseCase) Bootstrap(ctx context.Context, username, password string) (bool, error) {
	n, err := u.store.CountAccounts(ctx)
	if err != nil {
		return false, fmt.Errorf("count accounts: %w", err)
	}
	if n > 0 {
		return false, nil
	}
	a, err := u.newAccount(username, password, true)
	if err != nil {
		return false, err
	}
	if err := u.store.CreateAccount(ctx, a); err != nil {
		return false, fmt.Errorf("create admin: %w", err)
	}
	return true, nil
}

// Local returns the single dev account used by -local mode, creating it if needed.
func (u *UseCase) Local(ctx context.Context) (domain.Account, error) {
	a, err := u.store.AccountByUsername(ctx, "local")
	if err == nil {
		return a, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Account{}, fmt.Errorf("find local account: %w", err)
	}
	a = domain.Account{ID: u.tokens.New(), Username: "local", Admin: true, DailyTokenCap: domain.DefaultDailyTokenCap, CreatedAt: u.clock.Now()}
	if err := u.store.CreateAccount(ctx, a); err != nil {
		return domain.Account{}, fmt.Errorf("create local account: %w", err)
	}
	return a, nil
}

func (u *UseCase) newAccount(username, password string, admin bool) (domain.Account, error) {
	username = strings.TrimSpace(username)
	if err := domain.ValidateCredentials(username, password); err != nil {
		return domain.Account{}, err
	}
	hash, err := u.hash.Hash(password)
	if err != nil {
		return domain.Account{}, fmt.Errorf("hash password: %w", err)
	}
	return domain.Account{
		ID: u.tokens.New(), Username: username, PasswordHash: hash, Admin: admin,
		DailyTokenCap: domain.DefaultDailyTokenCap, CreatedAt: u.clock.Now(),
	}, nil
}

// CreateInvite makes a one-use invite code. Admins only. A sponsored invite
// makes an account whose agents run on the admin's own Anthropic key, so a
// friend can play without one.
func (u *UseCase) CreateInvite(ctx context.Context, adminID string, sponsored bool) (domain.Invite, error) {
	if err := u.requireAdmin(ctx, adminID); err != nil {
		return domain.Invite{}, err
	}
	if sponsored {
		if err := u.canSponsor(ctx, adminID); err != nil {
			return domain.Invite{}, err
		}
	}
	inv := domain.Invite{Code: u.tokens.New(), CreatedBy: adminID, Sponsored: sponsored, CreatedAt: u.clock.Now()}
	if err := u.store.CreateInvite(ctx, inv); err != nil {
		return domain.Invite{}, fmt.Errorf("create invite: %w", err)
	}
	return inv, nil
}

// Invites lists the invites an admin has made.
func (u *UseCase) Invites(ctx context.Context, adminID string) ([]domain.Invite, error) {
	if err := u.requireAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	out, err := u.store.Invites(ctx, adminID)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	return out, nil
}

func (u *UseCase) requireAdmin(ctx context.Context, id string) error {
	a, err := u.store.Account(ctx, id)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	if !a.Admin {
		return fmt.Errorf("%w: admins only", domain.ErrForbidden)
	}
	return nil
}

// Register redeems an invite, creates the account and signs it in.
func (u *UseCase) Register(ctx context.Context, code, username, password string) (domain.Session, error) {
	a, err := u.newAccount(username, password, false)
	if err != nil {
		return domain.Session{}, err
	}
	code = strings.TrimSpace(code)
	inv, err := u.store.Invite(ctx, code)
	if err != nil {
		return domain.Session{}, fmt.Errorf("%w: that invite code is not valid or was already used", domain.ErrInvalid)
	}
	if inv.Sponsored {
		a.SponsoredBy, a.SponsorCap = inv.CreatedBy, domain.DefaultSponsorCap
	}
	if err := u.store.RegisterWithInvite(ctx, code, a); err != nil {
		return domain.Session{}, fmt.Errorf("register: %w", err)
	}
	return u.startSession(ctx, a.ID)
}

// Login checks a username and password and starts a session.
func (u *UseCase) Login(ctx context.Context, username, password string) (domain.Session, error) {
	a, err := u.store.AccountByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		u.hash.Matches(dummyHash, password)
		return domain.Session{}, fmt.Errorf("%w: wrong username or password", domain.ErrUnauthorized)
	}
	if !u.hash.Matches(a.PasswordHash, password) {
		return domain.Session{}, fmt.Errorf("%w: wrong username or password", domain.ErrUnauthorized)
	}
	return u.startSession(ctx, a.ID)
}

func (u *UseCase) startSession(ctx context.Context, accountID string) (domain.Session, error) {
	s := domain.Session{Token: u.tokens.New(), AccountID: accountID, ExpiresAt: u.clock.Now().Add(SessionTTL)}
	if err := u.store.CreateSession(ctx, s); err != nil {
		return domain.Session{}, fmt.Errorf("create session: %w", err)
	}
	return s, nil
}

// Logout ends a session.
func (u *UseCase) Logout(ctx context.Context, token string) error {
	if err := u.store.DeleteSession(ctx, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Authenticate resolves a session token to its account.
func (u *UseCase) Authenticate(ctx context.Context, token string) (domain.Account, error) {
	s, err := u.store.Session(ctx, token)
	if err != nil || !u.clock.Now().Before(s.ExpiresAt) {
		return domain.Account{}, fmt.Errorf("%w: please sign in", domain.ErrUnauthorized)
	}
	a, err := u.store.Account(ctx, s.AccountID)
	if err != nil {
		return domain.Account{}, fmt.Errorf("%w: account missing", domain.ErrUnauthorized)
	}
	return a, nil
}

// SetKey stores (or, with an empty secret, removes) an account's API key.
func (u *UseCase) SetKey(ctx context.Context, accountID string, kind domain.KeyKind, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		if err := u.store.DeleteKey(ctx, accountID, kind); err != nil {
			return fmt.Errorf("delete key: %w", err)
		}
		return nil
	}
	if len(secret) < 20 || len(secret) > 400 || strings.ContainsAny(secret, " \t\n") {
		return fmt.Errorf("%w: that does not look like an API key", domain.ErrInvalid)
	}
	sealed, err := u.cipher.Seal([]byte(secret))
	if err != nil {
		return fmt.Errorf("encrypt key: %w", err)
	}
	if err := u.store.PutKey(ctx, accountID, kind, sealed, domain.Last4(secret)); err != nil {
		return fmt.Errorf("store key: %w", err)
	}
	return nil
}

// Keys describes which keys an account has stored, never the keys themselves.
func (u *UseCase) Keys(ctx context.Context, accountID string) ([]domain.KeyInfo, error) {
	out := make([]domain.KeyInfo, 0, 2)
	for _, kind := range []domain.KeyKind{domain.KeyAnthropic, domain.KeyOpenAI} {
		_, last4, err := u.store.Key(ctx, accountID, kind)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			out = append(out, domain.KeyInfo{Kind: kind})
		case err != nil:
			return nil, fmt.Errorf("load key: %w", err)
		default:
			out = append(out, domain.KeyInfo{Kind: kind, Set: true, Last4: last4})
		}
	}
	return out, nil
}

// SetDailyTokenCap changes how many tokens the account's agents may use per day.
func (u *UseCase) SetDailyTokenCap(ctx context.Context, accountID string, tokenCap int) error {
	if tokenCap < 0 || tokenCap > domain.MaxTokenCap {
		return fmt.Errorf("%w: cap must be between 0 and 100,000,000", domain.ErrInvalid)
	}
	if err := u.store.SetDailyTokenCap(ctx, accountID, tokenCap); err != nil {
		return fmt.Errorf("set cap: %w", err)
	}
	return nil
}

// SetAgentModel chooses the model the account's characters' agents run on.
// Empty means the table's default.
func (u *UseCase) SetAgentModel(ctx context.Context, accountID, model string) error {
	model, err := domain.ParseAgentModel(model)
	if err != nil {
		return err
	}
	if err := u.store.SetAgentModel(ctx, accountID, model); err != nil {
		return fmt.Errorf("set agent model: %w", err)
	}
	return nil
}

// IsAdmin reports whether an account runs the realm.
func (u *UseCase) IsAdmin(ctx context.Context, accountID string) (bool, error) {
	a, err := u.store.Account(ctx, accountID)
	if err != nil {
		return false, fmt.Errorf("load account: %w", err)
	}
	return a.Admin, nil
}

// Username returns an account's display name.
func (u *UseCase) Username(ctx context.Context, accountID string) (string, error) {
	a, err := u.store.Account(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("load account: %w", err)
	}
	return a.Username, nil
}

// AgentModel returns the account's chosen model for its characters ("" = default).
func (u *UseCase) AgentModel(ctx context.Context, accountID string) (string, error) {
	a, err := u.store.Account(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("load account: %w", err)
	}
	return a.AgentModel, nil
}

// Spend reports today's usage and cap. SponsorCap is what a sponsor pays for
// each day, when someone does.
type Spend struct {
	Used       int `json:"used"`
	Cap        int `json:"cap"`
	SponsorCap int `json:"sponsorCap,omitempty"`
}

// Today returns the account's spend so far today (UTC).
func (u *UseCase) Today(ctx context.Context, accountID string) (Spend, error) {
	a, err := u.store.Account(ctx, accountID)
	if err != nil {
		return Spend{}, fmt.Errorf("load account: %w", err)
	}
	used, err := u.store.Usage(ctx, accountID, u.day())
	if err != nil {
		return Spend{}, fmt.Errorf("load usage: %w", err)
	}
	out := Spend{Used: used, Cap: a.DailyTokenCap}
	if a.Sponsored() {
		out.SponsorCap = a.SponsorCap
	}
	return out, nil
}

func (u *UseCase) day() string {
	return u.clock.Now().UTC().Format("2006-01-02")
}

// Credential returns the decrypted key an agent call should use for the paying
// account, refusing if the account has none or is over its daily cap. A
// sponsored player without an Anthropic key of their own uses their sponsor's.
func (u *UseCase) Credential(ctx context.Context, accountID string, kind domain.KeyKind) (string, error) {
	sealed, _, err := u.store.Key(ctx, accountID, kind)
	if errors.Is(err, domain.ErrNotFound) && kind == domain.KeyAnthropic {
		return u.sponsorsKey(ctx, accountID)
	}
	if errors.Is(err, domain.ErrNotFound) {
		return "", fmt.Errorf("%w: add your %s key in Settings", domain.ErrNoKey, kind)
	}
	if err != nil {
		return "", fmt.Errorf("load key: %w", err)
	}
	if kind == domain.KeyAnthropic {
		spend, err := u.Today(ctx, accountID)
		if err != nil {
			return "", err
		}
		if spend.Used >= spend.Cap {
			return "", fmt.Errorf("%w (%d of %d tokens)", domain.ErrOverBudget, spend.Used, spend.Cap)
		}
	}
	return u.open(sealed)
}

func (u *UseCase) open(sealed []byte) (string, error) {
	plain, err := u.cipher.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("decrypt key: %w", err)
	}
	return string(plain), nil
}

// sponsorsKey returns the sponsor's Anthropic key for a sponsored player,
// within what the sponsor pays for each day.
func (u *UseCase) sponsorsKey(ctx context.Context, accountID string) (string, error) {
	a, err := u.store.Account(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("load account: %w", err)
	}
	if !a.Sponsored() {
		return "", fmt.Errorf("%w: add your %s key in Settings", domain.ErrNoKey, domain.KeyAnthropic)
	}
	sealed, _, err := u.store.Key(ctx, a.SponsoredBy, domain.KeyAnthropic)
	if errors.Is(err, domain.ErrNotFound) {
		return "", fmt.Errorf("%w: your sponsor has no key on file; add your own in Settings", domain.ErrNoKey)
	}
	if err != nil {
		return "", fmt.Errorf("load sponsor's key: %w", err)
	}
	used, err := u.store.Usage(ctx, accountID, u.day())
	if err != nil {
		return "", fmt.Errorf("load usage: %w", err)
	}
	if limit := min(a.DailyTokenCap, a.SponsorCap); used >= limit {
		return "", fmt.Errorf("%w (%d of %d tokens your sponsor pays for today)", domain.ErrOverBudget, used, limit)
	}
	return u.open(sealed)
}

// canSponsor checks an admin has an Anthropic key to pay for others with.
func (u *UseCase) canSponsor(ctx context.Context, adminID string) error {
	_, _, err := u.store.Key(ctx, adminID, domain.KeyAnthropic)
	if errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("%w: add your own Anthropic key before paying for other players", domain.ErrNoKey)
	}
	if err != nil {
		return fmt.Errorf("load key: %w", err)
	}
	return nil
}

// Player is one account as the realm's admin sees it.
type Player struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Admin      bool   `json:"admin"`
	OwnKey     bool   `json:"ownKey"`
	Sponsored  bool   `json:"sponsored"`
	SponsorCap int    `json:"sponsorCap"`
	UsedToday  int    `json:"usedToday"`
}

// Players lists the realm's accounts with today's spend. Admins only.
func (u *UseCase) Players(ctx context.Context, adminID string) ([]Player, error) {
	if err := u.requireAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	all, err := u.store.Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	out := make([]Player, 0, len(all))
	for _, a := range all {
		p, err := u.player(ctx, a)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (u *UseCase) player(ctx context.Context, a domain.Account) (Player, error) {
	used, err := u.store.Usage(ctx, a.ID, u.day())
	if err != nil {
		return Player{}, fmt.Errorf("load usage: %w", err)
	}
	_, _, err = u.store.Key(ctx, a.ID, domain.KeyAnthropic)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return Player{}, fmt.Errorf("load key: %w", err)
	}
	return Player{
		ID: a.ID, Username: a.Username, Admin: a.Admin, OwnKey: err == nil,
		Sponsored: a.Sponsored(), SponsorCap: a.SponsorCap, UsedToday: used,
	}, nil
}

// Sponsor has an admin pay for a player's agents on the admin's own key, up
// to tokenCap tokens a day; 0 stops paying. Players with a key of their own
// always use it.
func (u *UseCase) Sponsor(ctx context.Context, adminID, accountID string, tokenCap int) error {
	if err := u.requireAdmin(ctx, adminID); err != nil {
		return err
	}
	if tokenCap < 0 || tokenCap > domain.MaxTokenCap || accountID == adminID {
		return fmt.Errorf("%w: pay for another player, up to %d tokens a day", domain.ErrInvalid, domain.MaxTokenCap)
	}
	if _, err := u.store.Account(ctx, accountID); err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	sponsor := ""
	if tokenCap > 0 {
		if err := u.canSponsor(ctx, adminID); err != nil {
			return err
		}
		sponsor = adminID
	}
	if err := u.store.SetSponsor(ctx, accountID, sponsor, tokenCap); err != nil {
		return fmt.Errorf("set sponsor: %w", err)
	}
	return nil
}

// RecordUsage adds tokens to the account's spend for today.
func (u *UseCase) RecordUsage(ctx context.Context, accountID string, tokens int) error {
	if tokens <= 0 {
		return nil
	}
	if err := u.store.AddUsage(ctx, accountID, u.day(), tokens); err != nil {
		return fmt.Errorf("record usage: %w", err)
	}
	return nil
}
