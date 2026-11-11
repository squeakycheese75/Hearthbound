package characters_test

import (
	"cmp"
	"context"
	"dnd/internal/adapter/srd"
	"dnd/internal/domain"
	"dnd/internal/usecase/characters"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()

var errBoom = errors.New("boom")

type memStore struct {
	chars map[string]domain.Character
	fail  string
	gold  int
}

func (m *memStore) err(op string) error {
	if m.fail == op {
		return errBoom
	}
	return nil
}

func (m *memStore) CreateCharacter(_ context.Context, ch domain.Character) error {
	if err := m.err("create"); err != nil {
		return err
	}
	m.chars[ch.ID] = ch
	return nil
}

func (m *memStore) Character(_ context.Context, id string) (domain.Character, error) {
	ch, ok := m.chars[id]
	if !ok {
		return domain.Character{}, domain.ErrNotFound
	}
	return ch, nil
}

func (m *memStore) CharactersByOwner(_ context.Context, owner string) ([]domain.Character, error) {
	var out []domain.Character
	for _, ch := range m.chars {
		if ch.OwnerID == owner {
			out = append(out, ch)
		}
	}
	return out, m.err("list")
}

func (m *memStore) RecordGold(_ context.Context, e domain.LedgerEntry) error {
	m.gold += e.Amount
	return m.err("gold")
}

func (m *memStore) IdleCharacters(_ context.Context, except string) ([]domain.Character, error) {
	var out []domain.Character
	for _, ch := range m.chars {
		if ch.OwnerID != except && ch.Status == domain.CharIdle {
			out = append(out, ch)
		}
	}
	return out, m.err("idle")
}

func (m *memStore) SaveCharacter(_ context.Context, ch domain.Character) error {
	if err := m.err("save"); err != nil {
		return err
	}
	m.chars[ch.ID] = ch
	return nil
}

func (m *memStore) DeleteCharacter(_ context.Context, id string) error {
	if err := m.err("delete"); err != nil {
		return err
	}
	delete(m.chars, id)
	return nil
}

type easel struct{ keys []string }

func (e *easel) Request(r domain.ArtRequest) { e.keys = append(e.keys, r.Key+"@"+r.Payer) }

type clock struct{}

func (clock) Now() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }

type ids struct{ n int }

func (i *ids) New() string { i.n++; return fmt.Sprintf("ch%d", i.n) }

func setup() (*characters.UseCase, *memStore, *easel) {
	store, art := &memStore{chars: map[string]domain.Character{}}, &easel{}
	return characters.New(srd.New(), srd.New(), store, art, clock{}, &ids{}, firstDie{}), store, art
}

// firstDie always rolls 1, so random picks take the first candidate.
type firstDie struct{}

func (firstDie) Roll(int) int { return 1 }

// muse overrides the catalog's name and class ideas.
type muse struct {
	*srd.Catalog
	names   []string
	classes []domain.ClassTemplate
}

func (m muse) Names(string, domain.Gender) []string { return m.names }

func (m muse) Classes() []domain.ClassTemplate {
	if m.classes == nil {
		return m.Catalog.Classes()
	}
	return m.classes
}

func input() characters.CreateInput {
	return characters.CreateInput{AccountID: "me", Name: " Squeaky ", Class: "Wizard", Race: "Gnome", Gender: "m", Archetype: "Bookish Scholar", Backstory: " Lost a familiar. "}
}

func TestCreateListGet(t *testing.T) {
	t.Parallel()
	uc, _, art := setup()
	ch, err := uc.Create(ctx, input())
	if err != nil {
		t.Fatal(err)
	}
	s := ch.Sheet
	if ch.ID != "ch1" || s.ID != "ch1" || s.Name != "Squeaky" || s.OwnerID != "me" || s.Level != 1 || ch.Status != domain.CharIdle || ch.Hire != domain.HireNo {
		t.Fatalf("character %+v sheet %+v", ch, s)
	}
	if s.Persona == nil || s.Persona.Archetype != "Bookish Scholar" || s.Persona.Backstory != "Lost a familiar." {
		t.Fatalf("persona %+v", s.Persona)
	}
	if strings.Join(art.keys, ",") != "characters/ch1@me" {
		t.Fatalf("portrait should be requested on the owner's key: %v", art.keys)
	}
	list, err := uc.List(ctx, "me")
	if err != nil || len(list) != 1 {
		t.Fatalf("list %v %v", list, err)
	}
	if got, err := uc.Get(ctx, "me", "ch1"); err != nil || got.ID != "ch1" {
		t.Fatalf("get %v", err)
	}
	if _, err := uc.Get(ctx, "stranger", "ch1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("strangers must not see other rosters: %v", err)
	}
	if _, err := uc.Get(ctx, "me", "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*characters.CreateInput)
		want   error
	}{
		{"signed out", func(in *characters.CreateInput) { in.AccountID = "" }, domain.ErrUnauthorized},
		{"bad class", func(in *characters.CreateInput) { in.Class = "Pilot" }, domain.ErrUnknown},
		{"bad race", func(in *characters.CreateInput) { in.Race = "Robot" }, domain.ErrUnknown},
		{"bad archetype", func(in *characters.CreateInput) { in.Archetype = "Mime" }, domain.ErrUnknown},
		{"blank name", func(in *characters.CreateInput) { in.Name = " " }, domain.ErrInvalid},
		{"long name", func(in *characters.CreateInput) { in.Name = strings.Repeat("a", 41) }, domain.ErrInvalid},
		{"no gender", func(in *characters.CreateInput) { in.Gender = "" }, domain.ErrInvalid},
		{"long backstory", func(in *characters.CreateInput) { in.Backstory = strings.Repeat("a", 401) }, domain.ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc, _, _ := setup()
			in := input()
			tc.mutate(&in)
			if _, err := uc.Create(ctx, in); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRosterLimitAndStoreFailures(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	for i := range characters.MaxRoster {
		store.chars[fmt.Sprint(i)] = domain.Character{ID: fmt.Sprint(i), OwnerID: "me"}
	}
	if _, err := uc.Create(ctx, input()); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("full roster: %v", err)
	}
	for _, op := range []string{"list", "create"} {
		uc, store, _ := setup()
		store.fail = op
		if _, err := uc.Create(ctx, input()); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	uc, store, _ = setup()
	store.fail = "list"
	if _, err := uc.List(ctx, "me"); !errors.Is(err, errBoom) {
		t.Fatalf("list: %v", err)
	}
}

func TestHireAndRetire(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	ch, _ := uc.Create(ctx, input())
	if err := uc.SetHire(ctx, "me", ch.ID, domain.HireAsk); err != nil {
		t.Fatal(err)
	}
	if store.chars[ch.ID].Hire != domain.HireAsk {
		t.Fatal("hire policy not saved")
	}
	if err := uc.SetHire(ctx, "stranger", ch.ID, domain.HireAuto); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stranger hire: %v", err)
	}
	store.fail = "save"
	if err := uc.SetHire(ctx, "me", ch.ID, domain.HireNo); !errors.Is(err, errBoom) {
		t.Fatalf("save failure: %v", err)
	}
	store.fail = ""

	away := store.chars[ch.ID]
	away.Status = domain.CharAdventuring
	store.chars[ch.ID] = away
	if err := uc.Retire(ctx, "me", ch.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("retire while adventuring: %v", err)
	}
	away.Status = domain.CharFallen
	store.chars[ch.ID] = away
	store.fail = "delete"
	if err := uc.Retire(ctx, "me", ch.ID); !errors.Is(err, errBoom) {
		t.Fatalf("delete failure: %v", err)
	}
	store.fail = ""
	if err := uc.Retire(ctx, "me", ch.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.chars[ch.ID]; ok {
		t.Fatal("retired character should be gone")
	}
	if err := uc.Retire(ctx, "me", "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("retire missing: %v", err)
	}
}

func TestSetGenderOnceForOlderHeroes(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	ch, _ := uc.Create(ctx, input())
	if err := uc.SetGender(ctx, "me", ch.ID, domain.Female); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a hero made with a gender keeps it: %v", err)
	}
	tests := []struct {
		name, account string
		gender        domain.Gender
		fail          string
		want          error
	}{
		{"not a gender", "me", "x", "", domain.ErrInvalid},
		{"someone else's hero", "stranger", domain.Female, "", domain.ErrNotFound},
		{"save fails", "me", domain.Female, "save", errBoom},
		{"set", "me", domain.Female, "", nil},
	}
	for _, tc := range tests {
		store.fail = tc.fail
		store.chars[ch.ID].Sheet.Gender = ""
		if err := uc.SetGender(ctx, tc.account, ch.ID, tc.gender); !errors.Is(err, tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if store.chars[ch.ID].Sheet.Gender != domain.Female {
		t.Fatal("gender not saved")
	}
}

func TestRandom(t *testing.T) {
	t.Parallel()
	uc, store, art := setup()
	first, err := uc.Random(ctx, characters.RandomInput{AccountID: "me", Avoid: []string{"fighter"}})
	if err != nil {
		t.Fatal(err)
	}
	s := first.Sheet
	if s.Class != "Barbarian" || s.Race != "Human" || s.Name != "Aldric" || s.Gender != domain.Male || s.OwnerID != "me" || first.Status != domain.CharIdle {
		t.Fatalf("random hero %+v", s)
	}
	if s.Persona == nil || s.Persona.Archetype != "Gallant Knight" || s.Persona.Backstory == "" {
		t.Fatalf("persona %+v", s.Persona)
	}
	second, err := uc.Random(ctx, characters.RandomInput{AccountID: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Sheet.Name != "Cassian" || second.Sheet.Class != "Fighter" {
		t.Fatalf("names must not repeat on a roster: %+v", second.Sheet)
	}
	if len(store.chars) != 2 || strings.Join(art.keys, ",") != "characters/ch1@me,characters/ch2@me" {
		t.Fatalf("stored %d, art %v", len(store.chars), art.keys)
	}
}

func TestRandomEdgeCases(t *testing.T) {
	t.Parallel()
	one := []domain.ClassTemplate{srd.New().Classes()[1]}
	tests := []struct {
		name      string
		muse      muse
		avoid     []string
		fail      string
		wantName  string
		wantClass string
		wantErr   error
	}{
		{"names run out", muse{names: []string{"Pip"}}, nil, "", "Pip the Younger", "Fighter", nil},
		{"every class avoided", muse{names: []string{"Zed"}, classes: one}, []string{"Barbarian"}, "", "Zed", "Barbarian", nil},
		{"no ideas at all", muse{names: []string{}, classes: []domain.ClassTemplate{}}, nil, "", "", "", domain.ErrUnknown},
		{"roster unreadable", muse{names: []string{"Zed"}}, nil, "list", "", "", errBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.muse.Catalog = srd.New()
			store := &memStore{chars: map[string]domain.Character{
				"old": {ID: "old", OwnerID: "me", Sheet: &domain.Creature{Name: "pip"}},
			}, fail: tc.fail}
			uc := characters.New(srd.New(), tc.muse, store, &easel{}, clock{}, &ids{}, firstDie{})
			ch, err := uc.Random(ctx, characters.RandomInput{AccountID: "me", Avoid: tc.avoid})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil || ch.Sheet.Name != tc.wantName || ch.Sheet.Class != tc.wantClass {
				t.Fatalf("got %+v %v", ch.Sheet, err)
			}
		})
	}
}

func TestForHireListsOtherPlayersHeroesLookingForAdventure(t *testing.T) {
	t.Parallel()
	hero := func(id, owner string, status domain.CharacterStatus, hire domain.HirePolicy, dead bool) domain.Character {
		return domain.Character{ID: id, OwnerID: owner, Status: status, Hire: hire, Sheet: &domain.Creature{ID: id, Name: id, Dead: dead}}
	}
	st := &memStore{chars: map[string]domain.Character{
		"auto":  hero("auto", "friend", domain.CharIdle, domain.HireAuto, false),
		"ask":   hero("ask", "friend", domain.CharIdle, domain.HireAsk, false),
		"no":    hero("no", "friend", domain.CharIdle, domain.HireNo, false),
		"away":  hero("away", "friend", domain.CharAdventuring, domain.HireAuto, false),
		"mine":  hero("mine", "me", domain.CharIdle, domain.HireAuto, false),
		"ghost": hero("ghost", "friend", domain.CharIdle, domain.HireAuto, true),
	}}
	uc := characters.New(srd.New(), srd.New(), st, &easel{}, clock{}, &ids{}, firstDie{})
	got, err := uc.ForHire(ctx, "me")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, ch := range got {
		ids[ch.ID] = true
	}
	if len(got) != 2 || !ids["auto"] || !ids["ask"] {
		t.Fatalf("only other players' living heroes in town who are for hire: %v", ids)
	}
	st.fail = "idle"
	if _, err := uc.ForHire(ctx, "me"); !errors.Is(err, errBoom) {
		t.Fatalf("store failure: %v", err)
	}
}

func TestNewHeroesComeEquippedAndCanChangeGearInTown(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	ch, err := uc.Create(ctx, input())
	if err != nil {
		t.Fatal(err)
	}
	s := ch.Sheet
	if !s.Outfitted || len(s.Inventory) != 0 || s.Equipped(domain.SlotMain) == nil || s.BaseMaxHP != s.MaxHP {
		t.Fatalf("a new hero is outfitted from their class gear: %+v", s.Items)
	}
	main := s.Equipped(domain.SlotMain)
	if err := uc.Equip(ctx, "me", ch.ID, main.ID, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := uc.Get(ctx, "me", ch.ID)
	if got.Sheet.Equipped(domain.SlotMain) != nil || got.Sheet.Actions[0].ID != "unarmed" && got.Sheet.Equipped(domain.SlotRanged) == nil {
		t.Fatalf("taking the weapon off is saved: %+v", got.Sheet.Actions)
	}
	if err := uc.Equip(ctx, "me", ch.ID, "nothing", domain.SlotMain); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown item: %v", err)
	}
	if err := uc.Equip(ctx, "friend", ch.ID, main.ID, domain.SlotMain); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("someone else's hero: %v", err)
	}

	legacy := store.chars[ch.ID]
	legacy.Sheet.Outfitted, legacy.Sheet.Items, legacy.Sheet.Inventory = false, nil, []string{"rope"}
	store.chars[ch.ID] = legacy
	if err := uc.Equip(ctx, "me", ch.ID, "itm-"+ch.ID+"-0", ""); err != nil {
		t.Fatalf("an old hero is outfitted first, then the change applies: %v", err)
	}
	if got, _ := uc.Get(ctx, "me", ch.ID); !got.Sheet.Outfitted || got.Sheet.Item("itm-"+ch.ID+"-0") == nil {
		t.Fatal("old heroes get their gear as items")
	}

	away := store.chars[ch.ID]
	away.Status = domain.CharAdventuring
	store.chars[ch.ID] = away
	if err := uc.Equip(ctx, "me", ch.ID, main.ID, domain.SlotMain); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("heroes on an adventure change gear there: %v", err)
	}
	away.Status, away.Sheet.Class = domain.CharIdle, "Jester"
	store.chars[ch.ID] = away
	if err := uc.Equip(ctx, "me", ch.ID, main.ID, domain.SlotMain); err == nil {
		t.Fatal("an unknown class fails cleanly")
	}
	away.Sheet.Class = "Fighter"
	store.chars[ch.ID] = away
	store.fail = "save"
	if err := uc.Equip(ctx, "me", ch.ID, main.ID, ""); !errors.Is(err, errBoom) {
		t.Fatalf("save failures: %v", err)
	}
}

func TestStartingGoldIsRecorded(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	if _, err := uc.Create(ctx, input()); err != nil || store.gold != 10 {
		t.Fatalf("a new hero's purse enters the realm: %d %v", store.gold, err)
	}
	store.fail = "gold"
	if _, err := uc.Create(ctx, input()); !errors.Is(err, errBoom) {
		t.Fatalf("ledger failure: %v", err)
	}
}

func TestRosterShowsClassFeatures(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	store.chars["old"] = domain.Character{ID: "old", OwnerID: "me", Sheet: &domain.Creature{Name: "Old", Class: "Fighter", Level: 2}}
	store.chars["blank"] = domain.Character{ID: "blank", OwnerID: "me"}
	list, err := uc.List(ctx, "me")
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v %v", list, err)
	}
	old, _ := uc.Get(ctx, "me", "old")
	if !old.Sheet.HasFeature(domain.FeatActionSurge) || old.Sheet.Outfitted {
		t.Fatalf("an older hero sees their features without being re-equipped: %+v", old.Sheet.Features)
	}
}

func TestImproveInTown(t *testing.T) {
	t.Parallel()
	uc, store, _ := setup()
	ch, _ := uc.Create(ctx, input())
	hero := store.chars[ch.ID]
	hero.Sheet.ASI = 1
	store.chars[ch.ID] = hero
	str := hero.Sheet.Abilities[domain.STR]
	tests := []struct {
		name, account string
		inc           map[domain.Ability]int
		status        domain.CharacterStatus
		class         string
		fail          string
		want          error
	}{
		{name: "someone else's", account: "stranger", inc: map[domain.Ability]int{domain.STR: 2}, want: domain.ErrNotFound},
		{name: "on an adventure", account: "me", inc: map[domain.Ability]int{domain.STR: 2}, status: domain.CharAdventuring, want: domain.ErrInvalid},
		{name: "unknown class", account: "me", inc: map[domain.Ability]int{domain.STR: 2}, class: "Mime", want: domain.ErrUnknown},
		{name: "too much", account: "me", inc: map[domain.Ability]int{domain.STR: 3}, want: domain.ErrInvalid},
		{name: "save fails", account: "me", inc: map[domain.Ability]int{domain.STR: 2}, fail: "save", want: errBoom},
		{name: "improves", account: "me", inc: map[domain.Ability]int{domain.STR: 2}},
	}
	for _, tc := range tests {
		c := store.chars[ch.ID]
		c.Status, c.Sheet.Class, c.Sheet.ASI = cmp.Or(tc.status, domain.CharIdle), cmp.Or(tc.class, "Wizard"), 1
		c.Sheet.BaseAbilities[domain.STR] = str
		store.chars[ch.ID], store.fail = c, tc.fail
		if err := uc.Improve(ctx, tc.account, ch.ID, tc.inc); !errors.Is(err, tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if got := store.chars[ch.ID].Sheet; got.Abilities[domain.STR] != str+2 || got.ASI != 0 {
		t.Fatalf("STR %d ASI %d", got.Abilities[domain.STR], got.ASI)
	}
}

func TestBarbariansTakeUpTheirGreataxe(t *testing.T) {
	t.Parallel()
	uc, _, _ := setup()
	in := input()
	in.Class, in.Race = "Barbarian", "Half-Orc"
	ch, err := uc.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if main, ranged := ch.Sheet.Equipped(domain.SlotMain), ch.Sheet.Equipped(domain.SlotRanged); main == nil || main.Name != "Greataxe" || ranged == nil || ranged.Name != "Javelin" {
		t.Fatalf("class weapons first: main %+v ranged %+v", main, ranged)
	}
}
