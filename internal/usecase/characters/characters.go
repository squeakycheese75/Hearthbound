// Package characters manages each player's roster of persistent heroes.
package characters

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"strings"
	"time"
)

// Catalog supplies SRD templates and personality presets.
type Catalog interface {
	Class(name string) (domain.ClassTemplate, error)
	Race(name string) (domain.RaceTemplate, error)
	Archetypes() []domain.Persona
	Item(key string) (domain.BaseItem, error)
	ItemByName(name string) (domain.BaseItem, bool)
}

// Muse suggests ingredients for randomly rolled heroes.
type Muse interface {
	Classes() []domain.ClassTemplate
	Races() []domain.RaceTemplate
	Names(race string, g domain.Gender) []string
	Backstories() []string
}

// Roller rolls a die: a uniform value in 1..sides.
type Roller interface {
	Roll(sides int) int
}

// Store persists characters.
type Store interface {
	CreateCharacter(ctx context.Context, ch domain.Character) error
	RecordGold(ctx context.Context, e domain.LedgerEntry) error
	Character(ctx context.Context, id string) (domain.Character, error)
	CharactersByOwner(ctx context.Context, ownerID string) ([]domain.Character, error)
	IdleCharacters(ctx context.Context, exceptOwner string) ([]domain.Character, error)
	SaveCharacter(ctx context.Context, ch domain.Character) error
	DeleteCharacter(ctx context.Context, id string) error
}

// Illustrator paints portraits in the background.
type Illustrator interface {
	Request(r domain.ArtRequest)
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// IDGen makes unique IDs.
type IDGen interface {
	New() string
}

// CreateInput describes a new hero.
type CreateInput struct {
	AccountID string `json:"-"`
	Name      string `json:"name"`
	Class     string `json:"class"`
	Race      string `json:"race"`
	Gender    string `json:"gender"`
	Archetype string `json:"archetype"`
	Backstory string `json:"backstory"`
}

// MaxRoster bounds how many characters one account may keep.
const MaxRoster = 24

// maxBackstory bounds backstories, which other players' agents will read.
const maxBackstory = 400

// UseCase manages rosters.
type UseCase struct {
	catalog Catalog
	store   Store
	art     Illustrator
	clock   Clock
	ids     IDGen
	muse    Muse
	dice    Roller
}

// New wires the use case.
func New(catalog Catalog, muse Muse, store Store, art Illustrator, clock Clock, ids IDGen, dice Roller) *UseCase {
	return &UseCase{catalog: catalog, muse: muse, store: store, art: art, clock: clock, ids: ids, dice: dice}
}

// Create builds a level 1 hero, saves it to the roster and starts painting its portrait.
func (u *UseCase) Create(ctx context.Context, in CreateInput) (domain.Character, error) {
	if in.AccountID == "" {
		return domain.Character{}, fmt.Errorf("%w: sign in to create characters", domain.ErrUnauthorized)
	}
	roster, err := u.store.CharactersByOwner(ctx, in.AccountID)
	if err != nil {
		return domain.Character{}, fmt.Errorf("load roster: %w", err)
	}
	if len(roster) >= MaxRoster {
		return domain.Character{}, fmt.Errorf("%w: rosters hold at most %d characters", domain.ErrInvalid, MaxRoster)
	}
	sheet, err := u.sheet(in)
	if err != nil {
		return domain.Character{}, err
	}
	now := u.clock.Now()
	ch := domain.Character{
		ID: sheet.ID, OwnerID: in.AccountID, Sheet: sheet, Status: domain.CharIdle, Hire: domain.HireNo,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := u.store.CreateCharacter(ctx, ch); err != nil {
		return domain.Character{}, fmt.Errorf("save character: %w", err)
	}
	starting := domain.LedgerEntry{At: now, Reason: domain.GoldStarting, Amount: sheet.Gold, AccountID: in.AccountID, CharacterID: ch.ID, Note: sheet.Name}
	if err := u.store.RecordGold(ctx, starting); err != nil {
		return domain.Character{}, fmt.Errorf("record starting gold: %w", err)
	}
	u.art.Request(domain.CharacterPortrait(&ch))
	return ch, nil
}

func (u *UseCase) sheet(in CreateInput) (*domain.Creature, error) {
	persona, err := u.persona(in.Archetype)
	if err != nil {
		return nil, err
	}
	persona.Backstory = strings.TrimSpace(in.Backstory)
	if len(persona.Backstory) > maxBackstory {
		return nil, fmt.Errorf("%w: backstory must be under %d characters", domain.ErrInvalid, maxBackstory)
	}
	cls, err := u.catalog.Class(in.Class)
	if err != nil {
		return nil, err
	}
	race, err := u.catalog.Race(in.Race)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(in.Name)) > 40 {
		return nil, fmt.Errorf("%w: names must be 40 characters or fewer", domain.ErrInvalid)
	}
	gender, err := domain.ParseGender(in.Gender)
	if err != nil {
		return nil, err
	}
	sheet, err := domain.BuildCharacter(cls, race, domain.BuildSpec{
		ID: u.ids.New(), Name: in.Name, Level: 1, Controller: domain.ControlAI, Persona: &persona,
		Portrait: strings.ToLower(cls.Name), Gender: gender,
	})
	if err != nil {
		return nil, err
	}
	sheet.OwnerID = in.AccountID
	u.outfit(sheet, cls)
	return sheet, nil
}

// outfit turns a sheet's starting gear and class weapons into equipped items.
func (u *UseCase) outfit(sheet *domain.Creature, cls domain.ClassTemplate) {
	sheet.AdoptLegacyItems(u.catalog.ItemByName)
	sheet.Outfit(cls, u.catalog.ItemByName, u.itemLookup)
	sheet.Recalculate(cls, u.itemLookup)
}

func (u *UseCase) itemLookup(key string) (domain.BaseItem, bool) {
	b, err := u.catalog.Item(key)
	return b, err == nil
}

// Equip changes the gear of one of the account's heroes who is in town (an
// empty slot takes the item off). Heroes on an adventure change gear there.
func (u *UseCase) Equip(ctx context.Context, accountID, id, itemID string, slot domain.Slot) error {
	ch, err := u.Get(ctx, accountID, id)
	if err != nil {
		return err
	}
	if ch.Status == domain.CharAdventuring {
		return fmt.Errorf("%w: %s is on an adventure; change their gear there", domain.ErrInvalid, ch.Sheet.Name)
	}
	cls, err := u.catalog.Class(ch.Sheet.Class)
	if err != nil {
		return fmt.Errorf("class: %w", err)
	}
	if !ch.Sheet.Outfitted {
		u.outfit(ch.Sheet, cls)
	}
	if err := ch.Sheet.Equip(itemID, slot, cls, u.itemLookup); err != nil {
		return err
	}
	ch.Sheet.Recalculate(cls, u.itemLookup)
	ch.UpdatedAt = u.clock.Now()
	if err := u.store.SaveCharacter(ctx, ch); err != nil {
		return fmt.Errorf("save character: %w", err)
	}
	return nil
}

func (u *UseCase) persona(archetype string) (domain.Persona, error) {
	for _, p := range u.catalog.Archetypes() {
		if strings.EqualFold(p.Archetype, archetype) {
			return p, nil
		}
	}
	return domain.Persona{}, fmt.Errorf("%w: personality %q", domain.ErrUnknown, archetype)
}

// List returns the account's roster.
func (u *UseCase) List(ctx context.Context, accountID string) ([]domain.Character, error) {
	out, err := u.store.CharactersByOwner(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("load roster: %w", err)
	}
	for i := range out {
		u.refresh(&out[i])
	}
	return out, nil
}

// refresh brings a sheet's class features (and, once it is outfitted, its
// derived numbers) up to date for display. Nothing is saved.
func (u *UseCase) refresh(ch *domain.Character) {
	if ch.Sheet == nil {
		return
	}
	cls, err := u.catalog.Class(ch.Sheet.Class)
	if err != nil {
		return
	}
	ch.Sheet.SyncFeatures(cls)
	if ch.Sheet.Outfitted {
		ch.Sheet.Recalculate(cls, u.itemLookup)
	}
}

// ForHire lists other players' heroes who are in town and looking for
// adventure (hire policy ask or auto), for the tavern.
func (u *UseCase) ForHire(ctx context.Context, accountID string) ([]domain.Character, error) {
	all, err := u.store.IdleCharacters(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("load tavern: %w", err)
	}
	out := []domain.Character{}
	for _, ch := range all {
		if ch.Hire != domain.HireNo && ch.Available() {
			u.refresh(&ch)
			out = append(out, ch)
		}
	}
	return out, nil
}

// Get returns one of the account's characters.
func (u *UseCase) Get(ctx context.Context, accountID, id string) (domain.Character, error) {
	ch, err := u.store.Character(ctx, id)
	if err != nil {
		return domain.Character{}, fmt.Errorf("load character: %w", err)
	}
	if ch.OwnerID != accountID {
		return domain.Character{}, fmt.Errorf("%w: character %s", domain.ErrNotFound, id)
	}
	u.refresh(&ch)
	return ch, nil
}

// SetHire changes whether others may recruit the character from the pub.
func (u *UseCase) SetHire(ctx context.Context, accountID, id string, policy domain.HirePolicy) error {
	ch, err := u.Get(ctx, accountID, id)
	if err != nil {
		return err
	}
	ch.Hire, ch.UpdatedAt = policy, u.clock.Now()
	if err := u.store.SaveCharacter(ctx, ch); err != nil {
		return fmt.Errorf("save character: %w", err)
	}
	return nil
}

// Improve spends one of a hero's Ability Score Improvements while they are in
// town (+2 to one ability or +1 to two). Heroes on an adventure choose there.
func (u *UseCase) Improve(ctx context.Context, accountID, id string, inc map[domain.Ability]int) error {
	ch, err := u.Get(ctx, accountID, id)
	if err != nil {
		return err
	}
	if ch.Status == domain.CharAdventuring {
		return fmt.Errorf("%w: %s is on an adventure; choose there", domain.ErrInvalid, ch.Sheet.Name)
	}
	cls, err := u.catalog.Class(ch.Sheet.Class)
	if err != nil {
		return fmt.Errorf("class: %w", err)
	}
	if err := ch.Sheet.Improve(inc); err != nil {
		return err
	}
	ch.Sheet.Recalculate(cls, u.itemLookup)
	ch.UpdatedAt = u.clock.Now()
	if err := u.store.SaveCharacter(ctx, ch); err != nil {
		return fmt.Errorf("save character: %w", err)
	}
	return nil
}

// SetGender gives a hero made before genders existed their gender. It can be
// set once; a hero on an adventure picks it up there the next time the game loads.
func (u *UseCase) SetGender(ctx context.Context, accountID, id string, g domain.Gender) error {
	if _, err := domain.ParseGender(string(g)); err != nil {
		return err
	}
	ch, err := u.Get(ctx, accountID, id)
	if err != nil {
		return err
	}
	if ch.Sheet == nil || ch.Sheet.Gender != "" {
		return fmt.Errorf("%w: a hero's gender is set when they are made", domain.ErrInvalid)
	}
	ch.Sheet.Gender, ch.UpdatedAt = g, u.clock.Now()
	if err := u.store.SaveCharacter(ctx, ch); err != nil {
		return fmt.Errorf("save character: %w", err)
	}
	return nil
}

// Retire removes a character from the roster. Characters on an adventure must come home first.
func (u *UseCase) Retire(ctx context.Context, accountID, id string) error {
	ch, err := u.Get(ctx, accountID, id)
	if err != nil {
		return err
	}
	if ch.Status == domain.CharAdventuring {
		return fmt.Errorf("%w: %s is on an adventure; finish or delete it first", domain.ErrInvalid, ch.Sheet.Name)
	}
	if err := u.store.DeleteCharacter(ctx, id); err != nil {
		return fmt.Errorf("delete character: %w", err)
	}
	return nil
}
