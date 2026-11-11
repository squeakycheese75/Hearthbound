package domain

import (
	"fmt"
	"time"
)

// Mode is the current phase of play.
type Mode string

// Game modes.
const (
	ModeExploration Mode = "exploration"
	ModeCombat      Mode = "combat"
	ModeOver        Mode = "over"
)

// DMSeat is the transcript key for the Dungeon Master agent.
const DMSeat = "dm"

// SceneKeys are the illustrated scenes the UI has art for. The DM may only pick from these.
var SceneKeys = []string{
	"tavern", "village", "town", "road", "forest", "swamp", "mountains",
	"cave", "crypt", "dungeon", "castle", "camp", "ruins", "temple",
}

// Scene is the current backdrop. Visual is the DM's description for the illustrator.
type Scene struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Mood   string `json:"mood"`
	Visual string `json:"visual,omitempty"`
}

// Encounter is an active combat.
type Encounter struct {
	Round      int            `json:"round"`
	Order      []string       `json:"order"`
	Initiative map[string]int `json:"initiative"`
	Turn       int            `json:"turn"`
	GridW      int            `json:"gridW"`
	GridH      int            `json:"gridH"`
	// Spent is what the current combatant has used so far this turn.
	Spent TurnSpent `json:"spent"`
	// Reacted marks creatures that have used their reaction since their last turn.
	Reacted map[string]bool `json:"reacted,omitempty"`
}

// TurnSpent tracks one turn's action economy.
type TurnSpent struct {
	// Actions taken, and extra actions granted (Action Surge).
	Actions int `json:"actions,omitempty"`
	// Attacks counts weapon attacks made with Extra Attack; an odd count means
	// the second attack of the action is still to come.
	Attacks int  `json:"attacks,omitempty"`
	Extra   int  `json:"extra,omitempty"`
	Bonus   bool `json:"bonus,omitempty"`
	// Free lists once-per-turn free features already used (Reckless Attack, Action Surge).
	Free []FeatureKey `json:"free,omitempty"`
	// Sneak and Colossus are once-per-turn damage riders already applied.
	Sneak    bool `json:"sneak,omitempty"`
	Colossus bool `json:"colossus,omitempty"`
	// Dash is extra movement this turn, in feet; Raged marks a rage begun this turn.
	Dash  int  `json:"dash,omitempty"`
	Raged bool `json:"raged,omitempty"`
}

// ActionsLeft is how many actions the current combatant can still take.
func (t TurnSpent) ActionsLeft() int {
	return 1 + t.Extra - t.Actions
}

// Current returns the ID of the creature whose turn it is.
func (e *Encounter) Current() string {
	if e == nil || len(e.Order) == 0 {
		return ""
	}
	return e.Order[e.Turn%len(e.Order)]
}

// TokenUsage accumulates model usage for cost display.
type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
}

// Add accumulates u into t.
func (t *TokenUsage) Add(u TokenUsage) {
	t.Input += u.Input
	t.Output += u.Output
	t.CacheRead += u.CacheRead
	t.CacheWrite += u.CacheWrite
}

// Billed weighs tokens the way they are billed, in input-token terms: cache
// reads cost a tenth of fresh input and cache writes a quarter more. Agents
// re-read the cached story on every call, so a raw sum looks far scarier than
// the bill.
func (t TokenUsage) Billed() int {
	return t.Input + t.Output + t.CacheWrite*5/4 + t.CacheRead/10
}

// SeatUsage is what one agent seat has spent in an adventure, for debugging
// and cost display. Payer is the account whose key the seat's latest call used.
type SeatUsage struct {
	Payer string `json:"payer"`
	// Model is what the seat's latest call ran on.
	Model string     `json:"model,omitempty"`
	Calls int        `json:"calls"`
	Total TokenUsage `json:"total"`
	// Last is the seat's most recent call; its input side is the prompt size.
	Last TokenUsage `json:"last"`
}

// Charge records one agent call against a seat's ledger and the game total.
func (g *Game) Charge(seat, payer, model string, u TokenUsage) {
	g.Usage.Add(u)
	if g.Spend == nil {
		g.Spend = map[string]SeatUsage{}
	}
	s := g.Spend[seat]
	s.Payer, s.Model = payer, model
	s.Calls++
	s.Total.Add(u)
	s.Last = u
	g.Spend[seat] = s
}

// Game is a whole saved campaign session.
type Game struct {
	ID          string                    `json:"id"`
	LeaderID    string                    `json:"leaderId"`
	Title       string                    `json:"title"`
	DMStyle     string                    `json:"dmStyle"`
	CreatedAt   time.Time                 `json:"createdAt"`
	UpdatedAt   time.Time                 `json:"updatedAt"`
	Mode        Mode                      `json:"mode"`
	Scene       Scene                     `json:"scene"`
	Creatures   []*Creature               `json:"creatures"`
	Party       []string                  `json:"party"`
	Encounter   *Encounter                `json:"encounter,omitempty"`
	Transcripts map[string][]AgentMessage `json:"transcripts"`
	Log         []Event                   `json:"log"`
	// Context is how many tokens each seat's last request carried (its whole
	// prompt: system, history and the new message), used to decide when to
	// close a chapter and compact the agents' histories.
	Context map[string]int `json:"context,omitempty"`
	// Chapter counts chapter breaks; Recap is the DM's latest "story so far".
	Chapter int    `json:"chapter,omitempty"`
	Recap   string `json:"recap,omitempty"`
	// LogBase counts events trimmed from the front of Log, so clients can give
	// every event a stable position.
	LogBase int        `json:"logBase,omitempty"`
	Usage   TokenUsage `json:"usage"`
	// Spend breaks Usage down by agent seat: "dm", a character ID, or a helper
	// ("choices", "recap").
	Spend      map[string]SeatUsage `json:"spend,omitempty"`
	Awaiting   bool                 `json:"awaiting"`
	RoundNotes []string             `json:"roundNotes,omitempty"`
	Started    bool                 `json:"started"`
	// Roster is true when the party are persistent characters (IDs are character IDs).
	Roster bool `json:"roster,omitempty"`
	// Tutorial marks a new player's guided first adventure: the DM keeps it
	// short and explains each new thing as it happens.
	Tutorial bool `json:"tutorial,omitempty"`
	// OpenSeats are agent-played companions the leader has offered to friends
	// on the tavern's notice board.
	OpenSeats []string `json:"openSeats,omitempty"`
	// Wanted is the leader's post on the tavern's notice board: who the party
	// is looking for. Shown to players only, never to agents.
	Wanted string `json:"wanted,omitempty"`
	// Calls are open questions between this party and heroes' owners: the
	// leader asking for a hero, or an owner offering one.
	Calls []Call `json:"calls,omitempty"`
	// Quests is the party's journal, kept by the DM.
	Quests []Quest `json:"quests,omitempty"`
	// Imported is set once a pre-roster game's party has been copied into rosters.
	Imported bool `json:"imported,omitempty"`
	// Pace and Deadline drive autopilot: when Deadline passes, absent live
	// players' agents act for them. A zero Deadline means nothing is due.
	Pace     Pace      `json:"pace"`
	Deadline time.Time `json:"deadline,omitzero"`
	// Pending holds this exploration round's submitted actions, by character ID.
	Pending map[string]string `json:"pending,omitempty"`
	// Covering marks live characters whose agent is standing in for an away
	// owner. It clears as soon as the owner acts again.
	Covering map[string]bool `json:"covering,omitempty"`
	// Hold marks live characters whose owner has not let their agent cover for
	// them: when the owner is away, the character holds back instead.
	Hold map[string]bool `json:"hold,omitempty"`
	// NPCs are the people and creatures the DM has put on the table.
	NPCs []*NPC `json:"npcs,omitempty"`
	// CastAt is one past the log position of the last narration checked for
	// NPCs (0 = none yet), so older scenes get checked once when opened.
	CastAt int `json:"castAt,omitempty"`
	// TableNotes tell the DM about changes to the table (who joined or left),
	// in its next message.
	TableNotes []string `json:"tableNotes,omitempty"`
	// Joined and Left are party changes since the game was loaded. They are
	// not stored: the store embarks or settles those characters as it saves.
	Joined []string    `json:"-"`
	Left   []*Creature `json:"-"`
	// GoldMoves are gold ledger entries from this turn, recorded by the store
	// as it saves (not stored with the game).
	GoldMoves []LedgerEntry `json:"-"`
	// ItemSeq numbers the items created in this adventure, for their IDs.
	ItemSeq int `json:"itemSeq,omitempty"`
	// Typing says who may type free-text actions; the rest use quick choices.
	Typing TypingMode `json:"typing,omitempty"`
	// Choices are the DM's suggested next actions, shown as quick buttons.
	Choices []string `json:"choices,omitempty"`
}

// Summary is a short description of a saved game for menus.
type Summary struct {
	YourTurn  bool      `json:"yourTurn"`
	Deadline  time.Time `json:"deadline,omitzero"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Party     string    `json:"party"`
	Mode      Mode      `json:"mode"`
	Scene     string    `json:"scene"`
	SceneKey  string    `json:"sceneKey"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CanView reports whether an account may see and play this game: the leader,
// or the owner of any party member.
func (g *Game) CanView(accountID string) bool {
	if accountID == "" {
		return false
	}
	if g.LeaderID == accountID {
		return true
	}
	for _, c := range g.PartyMembers() {
		if c.OwnerID == accountID || c.PlayerID == accountID {
			return true
		}
	}
	return false
}

// Payer returns the account billed for a creature's agent: whoever plays it
// (a friend holding the seat, else the owner), else the leader.
func (g *Game) Payer(c *Creature) string {
	if c != nil && PlayerOf(c) != "" {
		return PlayerOf(c)
	}
	return g.LeaderID
}

// Creature finds a creature by ID.
func (g *Game) Creature(id string) (*Creature, error) {
	for _, c := range g.Creatures {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%w: creature %q", ErrNotFound, id)
}

// PartyMembers returns the player characters in seating order.
func (g *Game) PartyMembers() []*Creature {
	out := make([]*Creature, 0, len(g.Party))
	for _, id := range g.Party {
		c, err := g.Creature(id)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Hero returns the human-controlled character.
func (g *Game) Hero() (*Creature, error) {
	for _, c := range g.PartyMembers() {
		if c.Controller == ControlHuman {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%w: human hero", ErrNotFound)
}

// Monsters returns monsters still in the game.
func (g *Game) Monsters() []*Creature {
	var out []*Creature
	for _, c := range g.Creatures {
		if c.Kind == KindMonster {
			out = append(out, c)
		}
	}
	return out
}

// Hostile reports whether a and b are on opposite sides.
func Hostile(a, b *Creature) bool {
	return (a.Kind == KindMonster) != (b.Kind == KindMonster)
}

// AppendLog adds an event to the visible history, keeping the last 500.
func (g *Game) AppendLog(e Event) {
	g.Log = append(g.Log, e)
	if extra := len(g.Log) - 500; extra > 0 {
		g.Log = g.Log[extra:]
		g.LogBase += extra
	}
}
