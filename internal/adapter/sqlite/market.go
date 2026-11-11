package sqlite

import (
	"context"
	"database/sql"
	"dnd/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
)

// Tx is one market transaction: characters and listings read and written
// through it commit together.
type Tx struct {
	s  *Store
	tx *sql.Tx
}

// InTx runs fn in a transaction, committing only if it succeeds.
func (s *Store) InTx(ctx context.Context, fn func(*Tx) error) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return fn(&Tx{s: s, tx: tx}) })
}

// Character loads a character inside the transaction.
func (t *Tx) Character(ctx context.Context, id string) (domain.Character, error) {
	ch, err := loadCharacter(ctx, t.tx, id)
	t.s.adoptItems(&ch)
	return ch, err
}

// SaveCharacter writes a character inside the transaction.
func (t *Tx) SaveCharacter(ctx context.Context, ch domain.Character) error {
	return saveCharacter(ctx, t.tx, ch)
}

// Listing loads a listing inside the transaction.
func (t *Tx) Listing(ctx context.Context, id string) (domain.Listing, error) {
	var data []byte
	err := t.tx.QueryRowContext(ctx, `SELECT data FROM listings WHERE id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Listing{}, fmt.Errorf("%w: listing %s", domain.ErrNotFound, id)
	}
	if err != nil {
		return domain.Listing{}, fmt.Errorf("load listing: %w", err)
	}
	var l domain.Listing
	if err := json.Unmarshal(data, &l); err != nil {
		return domain.Listing{}, fmt.Errorf("decode listing %s: %w", id, err)
	}
	return l, nil
}

// SaveListing writes a listing inside the transaction.
func (t *Tx) SaveListing(ctx context.Context, l domain.Listing) error {
	data, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("encode listing: %w", err)
	}
	_, err = t.tx.ExecContext(ctx, `
INSERT INTO listings (id, seller_id, status, created_at, data) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, data=excluded.data`,
		l.ID, l.SellerID, string(l.Status), l.CreatedAt.UnixMilli(), data)
	if err != nil {
		return fmt.Errorf("save listing: %w", err)
	}
	return nil
}

// OpenListings lists everything for sale, newest first.
func (s *Store) OpenListings(ctx context.Context) ([]domain.Listing, error) {
	return s.queryListings(ctx, `SELECT data FROM listings WHERE status = ? ORDER BY created_at DESC`, string(domain.ListingOpen))
}

// ListingsBySeller lists an account's listings, newest first.
func (s *Store) ListingsBySeller(ctx context.Context, sellerID string) ([]domain.Listing, error) {
	return s.queryListings(ctx, `SELECT data FROM listings WHERE seller_id = ? ORDER BY created_at DESC`, sellerID)
}

func (s *Store) queryListings(ctx context.Context, query string, args ...any) ([]domain.Listing, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list listings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Listing{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan listing: %w", err)
		}
		var l domain.Listing
		if err := json.Unmarshal(data, &l); err != nil {
			return nil, fmt.Errorf("decode listing: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listings: %w", err)
	}
	return out, nil
}
