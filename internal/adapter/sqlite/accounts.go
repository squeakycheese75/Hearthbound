package sqlite

import (
	"context"
	"database/sql"
	"dnd/internal/domain"
	"errors"
	"fmt"
	"strings"
	"time"
)

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// CountAccounts returns how many accounts exist.
func (s *Store) CountAccounts(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count accounts: %w", err)
	}
	return n, nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAccount(ctx context.Context, db execer, a domain.Account) error {
	_, err := db.ExecContext(ctx, `INSERT INTO accounts (id, username, password_hash, admin, daily_token_cap, created_at, sponsored_by, sponsor_cap) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Username, a.PasswordHash, a.Admin, a.DailyTokenCap, a.CreatedAt.UnixMilli(), a.SponsoredBy, a.SponsorCap)
	if isUnique(err) {
		return fmt.Errorf("%w: username %q is taken", domain.ErrInvalid, a.Username)
	}
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// CreateAccount inserts an account; a taken username is ErrInvalid.
func (s *Store) CreateAccount(ctx context.Context, a domain.Account) error {
	return insertAccount(ctx, s.db, a)
}

const accountCols = `id, username, password_hash, admin, daily_token_cap, created_at, agent_model, sponsored_by, sponsor_cap`

// scanner is a *sql.Row or *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (domain.Account, error) {
	var a domain.Account
	var created int64
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Admin, &a.DailyTokenCap, &created, &a.AgentModel, &a.SponsoredBy, &a.SponsorCap)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, fmt.Errorf("%w: account", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("scan account: %w", err)
	}
	a.CreatedAt = time.UnixMilli(created).UTC()
	return a, nil
}

// Account loads an account by ID.
func (s *Store) Account(ctx context.Context, id string) (domain.Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, `SELECT `+accountCols+` FROM accounts WHERE id = ?`, id))
}

// AccountByUsername loads an account by username (case-insensitive).
func (s *Store) AccountByUsername(ctx context.Context, username string) (domain.Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, `SELECT `+accountCols+` FROM accounts WHERE username = ?`, username))
}

// SetDailyTokenCap updates an account's cap.
func (s *Store) SetDailyTokenCap(ctx context.Context, id string, tokenCap int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts SET daily_token_cap = ? WHERE id = ?`, tokenCap, id); err != nil {
		return fmt.Errorf("set cap: %w", err)
	}
	return nil
}

// Accounts lists every account, by username.
func (s *Store) Accounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+accountCols+` FROM accounts ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return out, nil
}

// SetSponsor records who pays for an account's agents, and how many tokens a
// day; an empty sponsor stops it.
func (s *Store) SetSponsor(ctx context.Context, id, sponsorID string, tokenCap int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts SET sponsored_by = ?, sponsor_cap = ? WHERE id = ?`, sponsorID, tokenCap, id); err != nil {
		return fmt.Errorf("set sponsor: %w", err)
	}
	return nil
}

// SetAgentModel stores the model an account's characters' agents run on.
func (s *Store) SetAgentModel(ctx context.Context, id, model string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts SET agent_model = ? WHERE id = ?`, model, id); err != nil {
		return fmt.Errorf("set agent model: %w", err)
	}
	return nil
}

// CreateSession stores a session.
func (s *Store) CreateSession(ctx context.Context, sess domain.Session) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token, account_id, expires_at) VALUES (?, ?, ?)`,
		sess.Token, sess.AccountID, sess.ExpiresAt.UnixMilli()); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// Session loads a session by token.
func (s *Store) Session(ctx context.Context, token string) (domain.Session, error) {
	sess := domain.Session{Token: token}
	var exp int64
	err := s.db.QueryRowContext(ctx, `SELECT account_id, expires_at FROM sessions WHERE token = ?`, token).Scan(&sess.AccountID, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, fmt.Errorf("%w: session", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("load session: %w", err)
	}
	sess.ExpiresAt = time.UnixMilli(exp).UTC()
	return sess, nil
}

// DeleteSession removes a session.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// CreateInvite stores an invite.
func (s *Store) CreateInvite(ctx context.Context, inv domain.Invite) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO invites (code, created_by, created_at, sponsored) VALUES (?, ?, ?, ?)`,
		inv.Code, inv.CreatedBy, inv.CreatedAt.UnixMilli(), inv.Sponsored); err != nil {
		return fmt.Errorf("insert invite: %w", err)
	}
	return nil
}

// Invites lists invites made by an account, newest first.
func (s *Store) Invites(ctx context.Context, createdBy string) ([]domain.Invite, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, created_by, used_by, created_at, sponsored FROM invites WHERE created_by = ? ORDER BY created_at DESC`, createdBy)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Invite{}
	for rows.Next() {
		var inv domain.Invite
		var created int64
		if err := rows.Scan(&inv.Code, &inv.CreatedBy, &inv.UsedBy, &created, &inv.Sponsored); err != nil {
			return nil, fmt.Errorf("scan invite: %w", err)
		}
		inv.CreatedAt = time.UnixMilli(created).UTC()
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invites: %w", err)
	}
	return out, nil
}

// Invite loads an invite by its code.
func (s *Store) Invite(ctx context.Context, code string) (domain.Invite, error) {
	var inv domain.Invite
	var created int64
	err := s.db.QueryRowContext(ctx, `SELECT code, created_by, used_by, created_at, sponsored FROM invites WHERE code = ?`, code).
		Scan(&inv.Code, &inv.CreatedBy, &inv.UsedBy, &created, &inv.Sponsored)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Invite{}, fmt.Errorf("%w: invite", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Invite{}, fmt.Errorf("load invite: %w", err)
	}
	inv.CreatedAt = time.UnixMilli(created).UTC()
	return inv, nil
}

// RegisterWithInvite consumes an unused invite and creates the account in one transaction.
func (s *Store) RegisterWithInvite(ctx context.Context, code string, a domain.Account) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE invites SET used_by = ? WHERE code = ? AND used_by = ''`, a.ID, code)
	if err != nil {
		return fmt.Errorf("use invite: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("%w: that invite code is not valid or was already used", domain.ErrInvalid)
	}
	if err := insertAccount(ctx, tx, a); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// PutKey stores or replaces an encrypted key.
func (s *Store) PutKey(ctx context.Context, accountID string, kind domain.KeyKind, sealed []byte, last4 string) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO api_keys (account_id, kind, sealed, last4) VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, kind) DO UPDATE SET sealed = excluded.sealed, last4 = excluded.last4`,
		accountID, string(kind), sealed, last4); err != nil {
		return fmt.Errorf("put key: %w", err)
	}
	return nil
}

// Key loads an encrypted key.
func (s *Store) Key(ctx context.Context, accountID string, kind domain.KeyKind) ([]byte, string, error) {
	var sealed []byte
	var last4 string
	err := s.db.QueryRowContext(ctx, `SELECT sealed, last4 FROM api_keys WHERE account_id = ? AND kind = ?`, accountID, string(kind)).Scan(&sealed, &last4)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("%w: %s key", domain.ErrNotFound, kind)
	}
	if err != nil {
		return nil, "", fmt.Errorf("load key: %w", err)
	}
	return sealed, last4, nil
}

// DeleteKey removes a key.
func (s *Store) DeleteKey(ctx context.Context, accountID string, kind domain.KeyKind) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE account_id = ? AND kind = ?`, accountID, string(kind)); err != nil {
		return fmt.Errorf("delete key: %w", err)
	}
	return nil
}

// AddUsage adds tokens to an account's daily total.
func (s *Store) AddUsage(ctx context.Context, accountID, day string, tokens int) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO usage (account_id, day, tokens) VALUES (?, ?, ?)
ON CONFLICT(account_id, day) DO UPDATE SET tokens = tokens + excluded.tokens`, accountID, day, tokens); err != nil {
		return fmt.Errorf("add usage: %w", err)
	}
	return nil
}

// Usage returns an account's tokens for a day.
func (s *Store) Usage(ctx context.Context, accountID, day string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT tokens FROM usage WHERE account_id = ? AND day = ?`, accountID, day).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load usage: %w", err)
	}
	return n, nil
}
