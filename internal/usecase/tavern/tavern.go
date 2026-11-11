// Package tavern is the notice board where friends find seats in each
// other's adventures.
package tavern

import (
	"context"
	"dnd/internal/domain"
	"fmt"
)

// Games finds the running adventures with business on the notice board.
type Games interface {
	GamesOnTheBoard(ctx context.Context) ([]*domain.Game, error)
}

// Seats seats a friend in an adventure, or offers a hero to a party (the
// play use case).
type Seats interface {
	TakeSeat(ctx context.Context, accountID, name, gameID, characterID string) error
	OfferHero(ctx context.Context, accountID, gameID, characterID string) error
}

// Names gives an account's display name.
type Names interface {
	Username(ctx context.Context, accountID string) (string, error)
}

// AI reports whether an account can pay for an agent (has a key, or the realm covers it).
type AI interface {
	Ready(ctx context.Context, accountID string) bool
}

// UseCase runs the notice board.
type UseCase struct {
	games Games
	seats Seats
	names Names
	ai    AI
}

// New wires the use case.
func New(games Games, seats Seats, names Names, ai AI) *UseCase {
	return &UseCase{games: games, seats: seats, names: names, ai: ai}
}

// Seat is an open seat on the board.
type Seat struct {
	GameID string `json:"gameId"`
	Title  string `json:"title"`
	Leader string `json:"leader"`
	Scene  string `json:"scene"`
	// Character is the companion on offer; Party names everyone at the table.
	Character domain.Creature `json:"character"`
	Party     []string        `json:"party"`
}

// Board lists the open seats in other people's adventures.
func (u *UseCase) Board(ctx context.Context, accountID string) ([]Seat, error) {
	games, err := u.games.GamesOnTheBoard(ctx)
	if err != nil {
		return nil, fmt.Errorf("load the board: %w", err)
	}
	out := []Seat{}
	for _, g := range games {
		if g.CanView(accountID) {
			continue // your own tables are run from the game
		}
		seats, err := u.seatsIn(ctx, g)
		if err != nil {
			return nil, err
		}
		out = append(out, seats...)
	}
	return out, nil
}

func (u *UseCase) seatsIn(ctx context.Context, g *domain.Game) ([]Seat, error) {
	leader, err := u.names.Username(ctx, g.LeaderID)
	if err != nil {
		return nil, fmt.Errorf("leader's name: %w", err)
	}
	party := partyNames(g)
	var out []Seat
	for _, id := range g.OpenSeats {
		c, err := g.Creature(id)
		if err != nil {
			continue // the companion has left since
		}
		out = append(out, Seat{GameID: g.ID, Title: g.Title, Leader: leader, Scene: g.Scene.Title, Character: *c, Party: party})
	}
	return out, nil
}

// Take seats an account in an open seat. Whoever plays a seat pays for its
// agent, so they need a key (or the realm's backing) first.
func (u *UseCase) Take(ctx context.Context, accountID, gameID, characterID string) error {
	if !u.ai.Ready(ctx, accountID) {
		return fmt.Errorf("%w: add your Anthropic key in Settings first; whoever plays a seat pays for its AI", domain.ErrNoKey)
	}
	name, err := u.names.Username(ctx, accountID)
	if err != nil {
		return fmt.Errorf("your name: %w", err)
	}
	if err := u.seats.TakeSeat(ctx, accountID, name, gameID, characterID); err != nil {
		return fmt.Errorf("take seat: %w", err)
	}
	return nil
}

// Offer offers one of the account's heroes to a party looking for one. The
// hero's agent would run on the account's key, so they need one first.
func (u *UseCase) Offer(ctx context.Context, accountID, gameID, characterID string) error {
	if !u.ai.Ready(ctx, accountID) {
		return fmt.Errorf("%w: add your Anthropic key in Settings first; your hero's agent runs on it", domain.ErrNoKey)
	}
	if err := u.seats.OfferHero(ctx, accountID, gameID, characterID); err != nil {
		return fmt.Errorf("offer hero: %w", err)
	}
	return nil
}

// Post is a party's post on the board: who they are looking for.
type Post struct {
	GameID string   `json:"gameId"`
	Title  string   `json:"title"`
	Leader string   `json:"leader"`
	Scene  string   `json:"scene"`
	Text   string   `json:"text"`
	Party  []string `json:"party"`
	// Offered is the hero the viewer has already offered, if any.
	Offered string `json:"offered,omitempty"`
}

// Notice is a call about a hero, as one side of it sees it.
type Notice struct {
	GameID string      `json:"gameId"`
	Title  string      `json:"title"`
	Call   domain.Call `json:"call"`
	From   string      `json:"from"`
}

// Notices is everything on the board for one account: posts from other
// parties, calls waiting for the account's answer, and calls it is waiting on.
type Notices struct {
	Posts   []Post   `json:"posts"`
	Inbox   []Notice `json:"inbox"`
	Waiting []Notice `json:"waiting"`
}

// Notices gathers the board's posts and the account's calls.
func (u *UseCase) Notices(ctx context.Context, accountID string) (Notices, error) {
	games, err := u.games.GamesOnTheBoard(ctx)
	if err != nil {
		return Notices{}, fmt.Errorf("load the board: %w", err)
	}
	out := Notices{Posts: []Post{}, Inbox: []Notice{}, Waiting: []Notice{}}
	for _, g := range games {
		if err := u.noticesIn(ctx, g, accountID, &out); err != nil {
			return Notices{}, err
		}
	}
	return out, nil
}

func (u *UseCase) noticesIn(ctx context.Context, g *domain.Game, accountID string, out *Notices) error {
	if err := u.callsIn(ctx, g, accountID, out); err != nil {
		return err
	}
	return u.postIn(ctx, g, accountID, out)
}

// callsIn sorts a game's calls into the account's inbox and waiting list.
func (u *UseCase) callsIn(ctx context.Context, g *domain.Game, accountID string, out *Notices) error {
	for _, c := range g.Calls {
		n := Notice{GameID: g.ID, Title: g.Title, Call: c}
		switch accountID {
		case c.Answerer(g.LeaderID):
			name, err := u.names.Username(ctx, c.Asker(g.LeaderID))
			if err != nil {
				return fmt.Errorf("asker's name: %w", err)
			}
			n.From = name
			out.Inbox = append(out.Inbox, n)
		case c.Asker(g.LeaderID):
			out.Waiting = append(out.Waiting, n)
		}
	}
	return nil
}

// postIn adds a game's party-wanted post, unless the account leads that party.
func (u *UseCase) postIn(ctx context.Context, g *domain.Game, accountID string, out *Notices) error {
	if g.Wanted == "" || g.LeaderID == accountID {
		return nil
	}
	leader, err := u.names.Username(ctx, g.LeaderID)
	if err != nil {
		return fmt.Errorf("leader's name: %w", err)
	}
	p := Post{GameID: g.ID, Title: g.Title, Leader: leader, Scene: g.Scene.Title, Text: g.Wanted, Party: partyNames(g)}
	for _, c := range g.Calls {
		if c.Kind == domain.CallOffer && c.OwnerID == accountID {
			p.Offered = c.Name
		}
	}
	out.Posts = append(out.Posts, p)
	return nil
}

func partyNames(g *domain.Game) []string {
	var party []string
	for _, c := range g.PartyMembers() {
		party = append(party, c.Name)
	}
	return party
}
