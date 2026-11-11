// Package sqlite stores games as JSON snapshots in a SQLite database.
package sqlite

import (
	"context"
	"database/sql"
	"dnd/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver
)

const schema = `
CREATE TABLE IF NOT EXISTS games (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL,
	party      TEXT NOT NULL,
	mode       TEXT NOT NULL,
	scene      TEXT NOT NULL,
	updated_at INTEGER NOT NULL,
	data       BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS accounts (
	id              TEXT PRIMARY KEY,
	username        TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash   TEXT NOT NULL,
	admin           INTEGER NOT NULL DEFAULT 0,
	daily_token_cap INTEGER NOT NULL,
	created_at      INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS invites (
	code       TEXT PRIMARY KEY,
	created_by TEXT NOT NULL,
	used_by    TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS api_keys (
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	kind       TEXT NOT NULL,
	sealed     BLOB NOT NULL,
	last4      TEXT NOT NULL,
	PRIMARY KEY (account_id, kind)
);
CREATE TABLE IF NOT EXISTS characters (
	id         TEXT PRIMARY KEY,
	owner_id   TEXT NOT NULL,
	status     TEXT NOT NULL,
	name       TEXT NOT NULL,
	updated_at INTEGER NOT NULL,
	data       BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS characters_owner ON characters(owner_id);
CREATE TABLE IF NOT EXISTS gold_ledger (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	at           INTEGER NOT NULL,
	reason       TEXT NOT NULL,
	amount       INTEGER NOT NULL,
	account_id   TEXT NOT NULL DEFAULT '',
	character_id TEXT NOT NULL DEFAULT '',
	game_id      TEXT NOT NULL DEFAULT '',
	note         TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS gold_ledger_at ON gold_ledger(at);
CREATE TABLE IF NOT EXISTS realm_settings (
	key  TEXT PRIMARY KEY,
	data BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS listings (
	id         TEXT PRIMARY KEY,
	seller_id  TEXT NOT NULL,
	status     TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	data       BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS listings_status ON listings(status);
CREATE INDEX IF NOT EXISTS listings_seller ON listings(seller_id);
CREATE TABLE IF NOT EXISTS usage (
	account_id TEXT NOT NULL,
	day        TEXT NOT NULL,
	tokens     INTEGER NOT NULL,
	PRIMARY KEY (account_id, day)
);`

// Store persists games.
type Store struct {
	db *sql.DB
	// items converts old free-text inventories to catalogue items on load
	// (nil: they are kept as keepsakes).
	items func(name string) (domain.BaseItem, bool)
}

// Open opens (or creates) the database at path.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// migrate upgrades databases created by older versions.
func migrate(ctx context.Context, db *sql.DB) error {
	for _, col := range []struct{ name, def string }{
		{"leader_id", "TEXT NOT NULL DEFAULT ''"},
		{"members", "TEXT NOT NULL DEFAULT ''"},
		{"waiting_for", "TEXT NOT NULL DEFAULT ''"},
		{"deadline", "INTEGER NOT NULL DEFAULT 0"},
		{"scene_key", "TEXT NOT NULL DEFAULT ''"},
		{"open_seats", "TEXT NOT NULL DEFAULT ''"},
		{"on_board", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := addColumn(ctx, db, "games", col.name, col.def); err != nil {
			return err
		}
	}
	for _, col := range []struct{ table, name, def string }{
		{"accounts", "agent_model", "TEXT NOT NULL DEFAULT ''"},
		{"accounts", "sponsored_by", "TEXT NOT NULL DEFAULT ''"},
		{"accounts", "sponsor_cap", "INTEGER NOT NULL DEFAULT 0"},
		{"invites", "sponsored", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := addColumn(ctx, db, col.table, col.name, col.def); err != nil {
			return err
		}
	}
	return fillSceneKeys(ctx, db)
}

// fillSceneKeys copies each older game's scene key out of its data, for the
// adventure list's artwork.
func fillSceneKeys(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT id, data FROM games WHERE scene_key = ''`)
	if err != nil {
		return fmt.Errorf("find scene keys: %w", err)
	}
	keys := map[string]string{}
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan game: %w", err)
		}
		var g struct {
			Scene domain.Scene `json:"scene"`
		}
		if json.Unmarshal(data, &g) == nil && g.Scene.Key != "" {
			keys[id] = g.Scene.Key
		}
	}
	_ = rows.Close()
	for id, key := range keys {
		if _, err := db.ExecContext(ctx, `UPDATE games SET scene_key = ? WHERE id = ?`, key, id); err != nil {
			return fmt.Errorf("fill scene key: %w", err)
		}
	}
	return nil
}

func addColumn(ctx context.Context, db *sql.DB, table, name, def string) error {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, name).Scan(&n)
	if err != nil {
		return fmt.Errorf("inspect schema: %w", err)
	}
	if n > 0 {
		return nil
	}
	// #nosec G202 -- table, name and def are compile-time constants above.
	if _, err := db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+name+` `+def); err != nil {
		return fmt.Errorf("add column %s: %w", name, err)
	}
	return nil
}

// csv encodes account IDs as ",a,b," so membership is a simple LIKE match.
func csv(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return "," + strings.Join(ids, ",") + ","
}

func members(g *domain.Game) []string {
	seen := map[string]bool{g.LeaderID: true}
	out := []string{g.LeaderID}
	for _, c := range g.PartyMembers() {
		for _, id := range []string{c.OwnerID, c.PlayerID} {
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

func unixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// DueGames lists games whose autopilot deadline has passed.
func (s *Store) DueGames(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM games WHERE deadline > 0 AND deadline <= ?`, now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("find due games: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan due game: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ClaimOrphans gives games saved before accounts existed to the given account,
// making it the leader and the owner of every party member. It returns how many it claimed.
func (s *Store) ClaimOrphans(ctx context.Context, accountID string) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM games WHERE leader_id = ''`)
	if err != nil {
		return 0, fmt.Errorf("find orphans: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan orphan: %w", err)
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	for _, id := range ids {
		g, err := s.Load(ctx, id)
		if err != nil {
			return 0, err
		}
		g.LeaderID = accountID
		for _, c := range g.PartyMembers() {
			c.OwnerID = accountID
		}
		if err := s.Save(ctx, g); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Save writes the whole game and, for roster adventures, copies each party
// member's sheet back to its character in the same transaction.
// Save stores the game and, in the same transaction, embarks characters that
// joined, settles ones that left, and writes every party sheet back.
func (s *Store) Save(ctx context.Context, g *domain.Game) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := upsertGame(ctx, tx, g); err != nil {
			return err
		}
		if err := embarkJoined(ctx, tx, g); err != nil {
			return err
		}
		if err := settleLeft(ctx, tx, g); err != nil {
			return err
		}
		for _, e := range g.GoldMoves {
			if err := recordGold(ctx, tx, e); err != nil {
				return err
			}
		}
		return settleParty(ctx, tx, g, g.Mode == domain.ModeOver, g.Title)
	})
	if err == nil {
		g.Joined, g.Left, g.GoldMoves = nil, nil, nil // done; a later save must not repeat them
	}
	return err
}

// embarkJoined marks characters that joined mid-adventure as away on it.
func embarkJoined(ctx context.Context, tx *sql.Tx, g *domain.Game) error {
	for _, id := range g.Joined {
		ch, err := loadCharacter(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := ch.Embark(g.ID, g.UpdatedAt); err != nil {
			return err
		}
		if err := saveCharacter(ctx, tx, ch); err != nil {
			return err
		}
	}
	return nil
}

// settleLeft sends characters that left the party home with what they earned.
func settleLeft(ctx context.Context, tx *sql.Tx, g *domain.Game) error {
	for _, c := range g.Left {
		ch, err := loadCharacter(ctx, tx, c.ID)
		if errors.Is(err, domain.ErrNotFound) {
			continue // retired meanwhile
		}
		if err != nil {
			return err
		}
		if ch.AdventureID != g.ID {
			continue
		}
		sheet := *c
		sheet.Controller = domain.ControlAI
		if sheet.Gender == "" && ch.Sheet != nil {
			sheet.Gender = ch.Sheet.Gender // set on the roster since this game loaded
		}
		why := " (left the party)"
		if c.Dead {
			why = " (fell in battle)"
		}
		ch.Settle(&sheet, true, g.Title+why, g.UpdatedAt)
		if err := saveCharacter(ctx, tx, ch); err != nil {
			return err
		}
	}
	return nil
}

func upsertGame(ctx context.Context, tx execer, g *domain.Game) error {
	data, err := json.Marshal(g)
	if err != nil {
		return fmt.Errorf("encode game: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO games (id, title, party, mode, scene, scene_key, updated_at, data, leader_id, members, waiting_for, deadline, open_seats, on_board)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET title=excluded.title, party=excluded.party, mode=excluded.mode,
	scene=excluded.scene, scene_key=excluded.scene_key, open_seats=excluded.open_seats, on_board=excluded.on_board, updated_at=excluded.updated_at, data=excluded.data, leader_id=excluded.leader_id,
	members=excluded.members, waiting_for=excluded.waiting_for, deadline=excluded.deadline`,
		g.ID, g.Title, partyText(g), string(g.Mode), g.Scene.Title, g.Scene.Key, g.UpdatedAt.UnixMilli(), data, g.LeaderID,
		csv(members(g)), csv(waitingFor(g)), unixMilli(g.Deadline), csv(g.OpenSeats), g.OnTheBoard())
	if err != nil {
		return fmt.Errorf("save game %s: %w", g.ID, err)
	}
	return nil
}

// settleParty writes party sheets back to their characters; over returns them to the realm.
func settleParty(ctx context.Context, tx *sql.Tx, g *domain.Game, over bool, title string) error {
	if !g.Roster {
		return nil
	}
	for _, c := range g.PartyMembers() {
		ch, err := loadCharacter(ctx, tx, c.ID)
		if errors.Is(err, domain.ErrNotFound) {
			continue // retired mid-adventure
		}
		if err != nil {
			return err
		}
		if ch.AdventureID != g.ID {
			continue // already settled elsewhere
		}
		sheet := *c
		sheet.Controller = domain.ControlAI
		if sheet.Gender == "" && ch.Sheet != nil {
			sheet.Gender = ch.Sheet.Gender // set on the roster since this game loaded
		}
		ch.Settle(&sheet, over, title, g.UpdatedAt)
		if err := saveCharacter(ctx, tx, ch); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func partyText(g *domain.Game) string {
	var parts []string
	for _, c := range g.PartyMembers() {
		parts = append(parts, fmt.Sprintf("%s the %s", c.Name, c.Class))
	}
	return strings.Join(parts, ", ")
}

// Load reads one game.
func (s *Store) Load(ctx context.Context, id string) (*domain.Game, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT data FROM games WHERE id = ?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: game %s", domain.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("load game %s: %w", id, err)
	}
	var g domain.Game
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("decode game %s: %w", id, err)
	}
	if g.Transcripts == nil {
		g.Transcripts = map[string][]domain.AgentMessage{}
	}
	for _, c := range g.Creatures {
		c.AdoptLegacyItems(s.lookupItem)
	}
	if err := s.adoptGenders(ctx, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// adoptGenders gives heroes from before genders existed the gender their
// owner has since set on the roster.
func (s *Store) adoptGenders(ctx context.Context, g *domain.Game) error {
	for _, c := range g.Creatures {
		if c.Kind != domain.KindPC || c.Gender != "" {
			continue
		}
		ch, err := loadCharacter(ctx, s.db, c.ID)
		if errors.Is(err, domain.ErrNotFound) {
			continue // a pre-roster hero, or retired
		}
		if err != nil {
			return err
		}
		if ch.Sheet != nil {
			c.Gender = ch.Sheet.Gender
		}
	}
	return nil
}

// UseItemCatalogue sets how old free-text inventories become items on load.
func (s *Store) UseItemCatalogue(lookup func(name string) (domain.BaseItem, bool)) {
	s.items = lookup
}

func (s *Store) lookupItem(name string) (domain.BaseItem, bool) {
	if s.items == nil {
		return domain.BaseItem{}, false
	}
	return s.items(name)
}

// adoptItems converts a character's old inventory to items.
func (s *Store) adoptItems(ch *domain.Character) {
	if ch.Sheet != nil {
		ch.Sheet.AdoptLegacyItems(s.lookupItem)
	}
}

// waitingFor is who the game waits on, only while it is open for input.
func waitingFor(g *domain.Game) []string {
	if !g.Awaiting || !g.Started {
		return nil
	}
	return g.WaitingFor()
}

// GamesOnTheBoard loads the running games with business on the tavern's
// notice board (open seats, party-wanted posts, calls), newest first.
func (s *Store) GamesOnTheBoard(ctx context.Context) ([]*domain.Game, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM games WHERE (on_board = 1 OR open_seats != '') AND mode != ? ORDER BY updated_at DESC`, string(domain.ModeOver))
	if err != nil {
		return nil, fmt.Errorf("find open seats: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan game: %w", err)
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	out := make([]*domain.Game, 0, len(ids))
	for _, id := range ids {
		g, err := s.Load(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// List returns summaries of games an account leads or has a character in,
// with whether it is their turn; newest first.
func (s *Store) List(ctx context.Context, accountID string) ([]domain.Summary, error) {
	member := "%," + accountID + ",%"
	rows, err := s.db.QueryContext(ctx, `SELECT id, title, party, mode, scene, scene_key, updated_at, waiting_for LIKE ?, deadline
FROM games WHERE leader_id = ? OR members LIKE ? ORDER BY updated_at DESC`, member, accountID, member)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Summary
	for rows.Next() {
		var sum domain.Summary
		var mode string
		var ms, deadline int64
		if err := rows.Scan(&sum.ID, &sum.Title, &sum.Party, &mode, &sum.Scene, &sum.SceneKey, &ms, &sum.YourTurn, &deadline); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		if deadline > 0 {
			sum.Deadline = time.UnixMilli(deadline).UTC()
		}
		sum.Mode = domain.Mode(mode)
		sum.UpdatedAt = time.UnixMilli(ms).UTC()
		out = append(out, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate games: %w", err)
	}
	return out, nil
}

// Delete removes a game. Its roster characters come home with what they had gained.
func (s *Store) Delete(ctx context.Context, id string) error {
	g, err := s.Load(ctx, id)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM games WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete game %s: %w", id, err)
		}
		return settleParty(ctx, tx, g, true, g.Title+" (abandoned)")
	})
}

// Snapshot writes a consistent copy of the whole database to a new file
// (SQLite's VACUUM INTO), safely while the server keeps running.
func (s *Store) Snapshot(ctx context.Context, path string) error {
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("snapshot to %s: %w", path, err)
	}
	return nil
}
