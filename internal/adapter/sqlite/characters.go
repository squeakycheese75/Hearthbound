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

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func loadCharacter(ctx context.Context, db querier, id string) (domain.Character, error) {
	var data []byte
	err := db.QueryRowContext(ctx, `SELECT data FROM characters WHERE id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Character{}, fmt.Errorf("%w: character %s", domain.ErrNotFound, id)
	}
	if err != nil {
		return domain.Character{}, fmt.Errorf("load character: %w", err)
	}
	var ch domain.Character
	if err := json.Unmarshal(data, &ch); err != nil {
		return domain.Character{}, fmt.Errorf("decode character %s: %w", id, err)
	}
	return ch, nil
}

func saveCharacter(ctx context.Context, db execer, ch domain.Character) error {
	data, err := json.Marshal(ch)
	if err != nil {
		return fmt.Errorf("encode character: %w", err)
	}
	name := ""
	if ch.Sheet != nil {
		name = ch.Sheet.Name
	}
	_, err = db.ExecContext(ctx, `INSERT INTO characters (id, owner_id, status, name, updated_at, data) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET owner_id=excluded.owner_id, status=excluded.status, name=excluded.name,
	updated_at=excluded.updated_at, data=excluded.data`,
		ch.ID, ch.OwnerID, string(ch.Status), name, ch.UpdatedAt.UnixMilli(), data)
	if err != nil {
		return fmt.Errorf("save character %s: %w", ch.ID, err)
	}
	return nil
}

// CreateCharacter inserts a new character.
func (s *Store) CreateCharacter(ctx context.Context, ch domain.Character) error {
	return saveCharacter(ctx, s.db, ch)
}

// SaveCharacter updates a character.
func (s *Store) SaveCharacter(ctx context.Context, ch domain.Character) error {
	return saveCharacter(ctx, s.db, ch)
}

// Character loads one character.
func (s *Store) Character(ctx context.Context, id string) (domain.Character, error) {
	ch, err := loadCharacter(ctx, s.db, id)
	s.adoptItems(&ch)
	return ch, err
}

// CharactersByOwner lists an account's roster, most recently active first.
func (s *Store) CharactersByOwner(ctx context.Context, ownerID string) ([]domain.Character, error) {
	return s.queryCharacters(ctx, `SELECT data FROM characters WHERE owner_id = ? ORDER BY updated_at DESC`, ownerID)
}

// IdleCharacters lists characters in town that belong to anyone but exceptOwner.
func (s *Store) IdleCharacters(ctx context.Context, exceptOwner string) ([]domain.Character, error) {
	return s.queryCharacters(ctx, `SELECT data FROM characters WHERE status = ? AND owner_id != ? ORDER BY updated_at DESC`,
		string(domain.CharIdle), exceptOwner)
}

func (s *Store) queryCharacters(ctx context.Context, query string, args ...any) ([]domain.Character, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list characters: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Character{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan character: %w", err)
		}
		var ch domain.Character
		if err := json.Unmarshal(data, &ch); err != nil {
			return nil, fmt.Errorf("decode character: %w", err)
		}
		s.adoptItems(&ch)
		out = append(out, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate characters: %w", err)
	}
	return out, nil
}

// DeleteCharacter removes a character.
func (s *Store) DeleteCharacter(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM characters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete character: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: character %s", domain.ErrNotFound, id)
	}
	return nil
}

// CreateAdventure saves a new game and its party's updated statuses atomically.
func (s *Store) CreateAdventure(ctx context.Context, g *domain.Game, party []domain.Character) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := upsertGame(ctx, tx, g); err != nil {
			return err
		}
		for _, ch := range party {
			if err := saveCharacter(ctx, tx, ch); err != nil {
				return err
			}
		}
		return nil
	})
}

// Imported records a legacy party member copied into its owner's roster.
type Imported struct {
	GameID      string
	CreatureID  string
	CharacterID string
}

// ImportLegacyParties copies the party members of games made before rosters
// existed into their owners' rosters (once per game). The games themselves are
// left unchanged and keep playing as before.
func (s *Store) ImportLegacyParties(ctx context.Context, newID func() string, now time.Time) ([]Imported, error) {
	ids, err := s.legacyGameIDs(ctx)
	if err != nil {
		return nil, err
	}
	var out []Imported
	for _, id := range ids {
		g, err := s.Load(ctx, id)
		if err != nil {
			return out, err
		}
		got, err := s.importParty(ctx, g, newID, now)
		if err != nil {
			return out, err
		}
		out = append(out, got...)
	}
	return out, nil
}

func (s *Store) legacyGameIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, data FROM games WHERE leader_id != ''`)
	if err != nil {
		return nil, fmt.Errorf("find legacy games: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		var flags struct {
			Roster   bool `json:"roster"`
			Imported bool `json:"imported"`
		}
		if err := json.Unmarshal(data, &flags); err != nil {
			return nil, fmt.Errorf("decode game %s: %w", id, err)
		}
		if !flags.Roster && !flags.Imported {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

func (s *Store) importParty(ctx context.Context, g *domain.Game, newID func() string, now time.Time) ([]Imported, error) {
	var out []Imported
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, c := range g.PartyMembers() {
			sheet := *c
			sheet.ID, sheet.OwnerID, sheet.Controller = newID(), g.LeaderID, domain.ControlAI
			status := domain.CharIdle
			if sheet.Dead {
				status = domain.CharFallen
			}
			ch := domain.Character{
				ID: sheet.ID, OwnerID: g.LeaderID, Sheet: &sheet, Status: status, Hire: domain.HireNo,
				History: []string{"Imported from " + g.Title}, CreatedAt: now, UpdatedAt: now,
			}
			if err := saveCharacter(ctx, tx, ch); err != nil {
				return err
			}
			out = append(out, Imported{GameID: g.ID, CreatureID: c.ID, CharacterID: ch.ID})
		}
		g.Imported = true
		return upsertGame(ctx, tx, g)
	})
	return out, err
}
