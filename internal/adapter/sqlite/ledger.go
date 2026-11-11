package sqlite

import (
	"context"
	"database/sql"
	"dnd/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// recordGold appends one entry to the gold ledger.
func recordGold(ctx context.Context, db execer, e domain.LedgerEntry) error {
	_, err := db.ExecContext(ctx, `INSERT INTO gold_ledger (at, reason, amount, account_id, character_id, game_id, note) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.At.UnixMilli(), string(e.Reason), e.Amount, e.AccountID, e.CharacterID, e.GameID, e.Note)
	if err != nil {
		return fmt.Errorf("record gold: %w", err)
	}
	return nil
}

// RecordGold appends one entry to the gold ledger.
func (s *Store) RecordGold(ctx context.Context, e domain.LedgerEntry) error {
	return recordGold(ctx, s.db, e)
}

// Record appends to the gold ledger inside a market transaction.
func (t *Tx) Record(ctx context.Context, e domain.LedgerEntry) error {
	return recordGold(ctx, t.tx, e)
}

// GoldFlows totals the ledger by reason since a time.
func (s *Store) GoldFlows(ctx context.Context, since time.Time) (map[domain.GoldReason]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reason, SUM(amount) FROM gold_ledger WHERE at >= ? GROUP BY reason`, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("gold flows: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[domain.GoldReason]int{}
	for rows.Next() {
		var reason string
		var total int
		if err := rows.Scan(&reason, &total); err != nil {
			return nil, fmt.Errorf("scan gold flow: %w", err)
		}
		out[domain.GoldReason(reason)] = total
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gold flows: %w", err)
	}
	return out, nil
}

// GoldSupply is all the gold in the realm: every living hero's purse plus
// sale proceeds waiting to be collected. Sheets are decoded in Go (game data
// is stored as BLOB, so no json_extract).
func (s *Store) GoldSupply(ctx context.Context) (gold, heroes int, err error) {
	chars, err := s.queryCharacters(ctx, `SELECT data FROM characters WHERE status != ?`, string(domain.CharFallen))
	if err != nil {
		return 0, 0, err
	}
	for _, ch := range chars {
		if ch.Sheet != nil {
			gold += ch.Sheet.Gold
		}
	}
	sold, err := s.queryListings(ctx, `SELECT data FROM listings WHERE status = ?`, string(domain.ListingSold))
	if err != nil {
		return 0, 0, err
	}
	for _, l := range sold {
		if !l.Collected {
			gold += l.Proceeds
		}
	}
	return gold, len(chars), nil
}

// RecentSales lists listings sold since a time, newest first.
func (s *Store) RecentSales(ctx context.Context, since time.Time) ([]domain.Listing, error) {
	sold, err := s.queryListings(ctx, `SELECT data FROM listings WHERE status = ? ORDER BY created_at DESC`, string(domain.ListingSold))
	if err != nil {
		return nil, err
	}
	out := []domain.Listing{}
	for _, l := range sold {
		if !l.SoldAt.Before(since) {
			out = append(out, l)
		}
	}
	return out, nil
}

// Economy loads the realm's economy settings (the defaults until changed).
func (s *Store) Economy(ctx context.Context) (domain.Economy, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT data FROM realm_settings WHERE key = 'economy'`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultEconomy, nil
	}
	if err != nil {
		return domain.Economy{}, fmt.Errorf("load economy: %w", err)
	}
	var e domain.Economy
	if err := json.Unmarshal(data, &e); err != nil {
		return domain.Economy{}, fmt.Errorf("decode economy: %w", err)
	}
	return e, nil
}

// SetEconomy saves the realm's economy settings.
func (s *Store) SetEconomy(ctx context.Context, e domain.Economy) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encode economy: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO realm_settings (key, data) VALUES ('economy', ?)
ON CONFLICT(key) DO UPDATE SET data = excluded.data`, data); err != nil {
		return fmt.Errorf("save economy: %w", err)
	}
	return nil
}
