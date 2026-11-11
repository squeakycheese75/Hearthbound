package sqlite_test

import (
	"context"
	"dnd/internal/adapter/sqlite"
	"dnd/internal/domain"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func hero(id, name string) domain.Character {
	return domain.Character{
		ID: id, OwnerID: "me", Status: domain.CharIdle, Hire: domain.HireNo,
		Sheet: &domain.Creature{ID: id, OwnerID: "me", Name: name, Kind: domain.KindPC, HP: 10, MaxHP: 10, Level: 1},
	}
}

func TestCharacterCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	_ = s.CreateCharacter(ctx, hero("a", "Aria"))
	_ = s.CreateCharacter(ctx, hero("b", "Bram"))
	other := hero("c", "Theirs")
	other.OwnerID = "friend"
	_ = s.CreateCharacter(ctx, other)
	mine, err := s.CharactersByOwner(ctx, "me")
	if err != nil || len(mine) != 2 {
		t.Fatalf("roster %v %v", mine, err)
	}
	if others, err := s.IdleCharacters(ctx, "me"); err != nil || len(others) != 1 || others[0].ID != "c" {
		t.Fatalf("idle characters of other players: %v %v", others, err)
	}
	a, _ := s.Character(ctx, "a")
	a.Hire = domain.HireAuto
	_ = s.SaveCharacter(ctx, a)
	if got, _ := s.Character(ctx, "a"); got.Hire != domain.HireAuto || got.Sheet.Name != "Aria" {
		t.Fatalf("saved %+v", got)
	}
	if err := s.DeleteCharacter(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Character(ctx, "a"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if err := s.DeleteCharacter(ctx, "a"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func adventure(id string, party ...domain.Character) (*domain.Game, []domain.Character) {
	g := &domain.Game{ID: id, LeaderID: "me", Title: "Thornwatch", Roster: true, Mode: domain.ModeExploration, UpdatedAt: time.UnixMilli(5000)}
	for i := range party {
		c := *party[i].Sheet
		g.Creatures = append(g.Creatures, &c)
		g.Party = append(g.Party, c.ID)
		_ = party[i].Embark(id, time.UnixMilli(1000))
	}
	return g, party
}

func TestAdventureWritesBackAndReleases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	_ = s.CreateCharacter(ctx, hero("a", "Aria"))
	_ = s.CreateCharacter(ctx, hero("b", "Bram"))
	a, _ := s.Character(ctx, "a")
	b, _ := s.Character(ctx, "b")
	g, party := adventure("g1", a, b)
	if err := s.CreateAdventure(ctx, g, party); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Character(ctx, "a"); got.Status != domain.CharAdventuring || got.AdventureID != "g1" {
		t.Fatalf("embarked %+v", got)
	}

	// Mid-adventure progress is written back but the character stays away.
	ariaInGame, _ := g.Creature("a")
	ariaInGame.XP, ariaInGame.Gold, ariaInGame.Controller = 300, 42, domain.ControlHuman
	bramInGame, _ := g.Creature("b")
	bramInGame.Dead = true
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Character(ctx, "a")
	if got.Sheet.XP != 300 || got.Sheet.Gold != 42 || got.Status != domain.CharAdventuring || got.Sheet.Controller != domain.ControlAI {
		t.Fatalf("write-back %+v / %+v", got, got.Sheet)
	}
	if bram, _ := s.Character(ctx, "b"); bram.Status != domain.CharFallen {
		t.Fatalf("dead companion should fall: %+v", bram)
	}

	// Ending the adventure brings survivors home, rested, and records it in their history.
	ariaInGame.HP, ariaInGame.SlotsUsed = 0, map[int]int{1: 1}
	ariaInGame.AddCondition(domain.Unconscious)
	g.Mode = domain.ModeOver
	_ = s.Save(ctx, g)
	got, _ = s.Character(ctx, "a")
	if got.Sheet.HP != got.Sheet.MaxHP || len(got.Sheet.Conditions) != 0 || got.Sheet.SlotsUsed != nil || got.Sheet.XP != 300 {
		t.Fatalf("survivors rest in town but keep their XP: %+v", got.Sheet)
	}
	if got.Status != domain.CharIdle || got.AdventureID != "" || len(got.History) != 1 || got.History[0] != "Thornwatch" {
		t.Fatalf("after the adventure %+v", got)
	}
	if bram, _ := s.Character(ctx, "b"); bram.Status != domain.CharFallen {
		t.Fatalf("the fallen stay fallen: %+v", bram)
	}
	// A later save of the finished game must not touch characters that moved on.
	got.Status, got.AdventureID = domain.CharAdventuring, "g2"
	_ = s.SaveCharacter(ctx, got)
	_ = s.Save(ctx, g)
	if again, _ := s.Character(ctx, "a"); again.AdventureID != "g2" {
		t.Fatal("settled characters belong to their new adventure")
	}
}

func TestDeletingAnAdventureSendsThePartyHome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	_ = s.CreateCharacter(ctx, hero("a", "Aria"))
	_ = s.CreateCharacter(ctx, hero("gone", "Retired"))
	a, _ := s.Character(ctx, "a")
	gone, _ := s.Character(ctx, "gone")
	g, party := adventure("g1", a, gone)
	_ = s.CreateAdventure(ctx, g, party)
	_ = s.DeleteCharacter(ctx, "gone") // retired mid-adventure: skipped on settle
	if c, _ := g.Creature("a"); c != nil {
		c.Gold = 99
	}
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Character(ctx, "a")
	if got.Status != domain.CharIdle || got.Sheet.Gold != 99 || got.History[0] != "Thornwatch (abandoned)" {
		t.Fatalf("abandoned %+v", got)
	}
	if err := s.Delete(ctx, "g1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("delete missing game: %v", err)
	}
}

func TestImportLegacyParties(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	legacy := &domain.Game{ID: "old", LeaderID: "me", Title: "Squeaky's Tale", Party: []string{"pc1", "pc2"}, Creatures: []*domain.Creature{
		{ID: "pc1", Name: "Squeaky", Kind: domain.KindPC, Controller: domain.ControlHuman, XP: 50},
		{ID: "pc2", Name: "Maren", Kind: domain.KindPC, Dead: true},
		{ID: "goblin-1", Name: "Goblin", Kind: domain.KindMonster},
	}}
	_ = s.Save(ctx, legacy)
	_ = s.Save(ctx, &domain.Game{ID: "new", LeaderID: "me", Roster: true})
	_ = s.Save(ctx, &domain.Game{ID: "orphan"})
	n := 0
	ids := func() string { n++; return fmt.Sprintf("ch%d", n) }
	got, err := s.ImportLegacyParties(ctx, ids, time.UnixMilli(1))
	if err != nil || len(got) != 2 || got[0].CreatureID != "pc1" || got[0].CharacterID != "ch1" {
		t.Fatalf("imported %+v %v", got, err)
	}
	squeaky, _ := s.Character(ctx, "ch1")
	if squeaky.OwnerID != "me" || squeaky.Sheet.XP != 50 || squeaky.Sheet.Controller != domain.ControlAI || squeaky.Status != domain.CharIdle || squeaky.History[0] != "Imported from Squeaky's Tale" {
		t.Fatalf("squeaky %+v", squeaky)
	}
	if maren, _ := s.Character(ctx, "ch2"); maren.Status != domain.CharFallen {
		t.Fatalf("maren %+v", maren)
	}
	if again, _ := s.ImportLegacyParties(ctx, ids, time.UnixMilli(1)); len(again) != 0 {
		t.Fatal("import runs once per game")
	}
	if old, _ := s.Load(ctx, "old"); !old.Imported || old.Creatures[0].ID != "pc1" {
		t.Fatal("legacy game keeps its own party")
	}
}

func TestJoiningAndLeavingMidAdventure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	for _, h := range []domain.Character{hero("a", "Aria"), hero("b", "Bram"), hero("c", "Cael")} {
		_ = s.CreateCharacter(ctx, h)
	}
	a, _ := s.Character(ctx, "a")
	b, _ := s.Character(ctx, "b")
	g, party := adventure("g1", a, b)
	if err := s.CreateAdventure(ctx, g, party); err != nil {
		t.Fatal(err)
	}
	cael, _ := s.Character(ctx, "c")
	g.Join(cael)
	bram, _ := g.Creature("b")
	bram.XP = 120
	g.Leave(bram)
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Character(ctx, "c"); got.Status != domain.CharAdventuring || got.AdventureID != "g1" {
		t.Fatalf("the newcomer embarks: %+v", got)
	}
	got, _ := s.Character(ctx, "b")
	if got.Status != domain.CharIdle || got.AdventureID != "" || got.Sheet.XP != 120 || got.History[0] != "Thornwatch (left the party)" {
		t.Fatalf("the leaver goes home with what they earned: %+v", got)
	}
	_ = s.CreateCharacter(ctx, hero("d", "Dain"))
	dain, _ := s.Character(ctx, "d")
	g.Join(dain)
	_ = s.Save(ctx, g)
	fallen, _ := g.Creature("d")
	fallen.Dead = true
	g.Leave(fallen)
	_ = s.Save(ctx, g)
	if got, _ := s.Character(ctx, "d"); got.Status != domain.CharFallen || got.AdventureID != "" || got.History[0] != "Thornwatch (fell in battle)" {
		t.Fatalf("a fallen hero laid to rest goes home fallen: %+v", got)
	}
	if len(g.Joined) != 0 || len(g.Left) != 0 {
		t.Fatal("a successful save clears the changes")
	}
	if err := s.Save(ctx, g); err != nil {
		t.Fatalf("saving again repeats nothing: %v", err)
	}

	busy := &domain.Game{ID: "g2", LeaderID: "me", Roster: true, Mode: domain.ModeExploration}
	busy.Join(cael) // already away on g1
	if err := s.Save(ctx, busy); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a character can only be on one adventure: %v", err)
	}
	if got, _ := s.Character(ctx, "c"); got.AdventureID != "g1" {
		t.Fatal("the failed save changed nothing")
	}
	ghost := &domain.Game{ID: "g3", LeaderID: "me", Roster: true, Mode: domain.ModeExploration}
	ghost.Join(domain.Character{ID: "nobody", Sheet: &domain.Creature{}})
	if err := s.Save(ctx, ghost); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("joining an unknown character fails: %v", err)
	}
	gone := &domain.Game{ID: "g1", LeaderID: "me", Roster: true}
	gone.Left = []*domain.Creature{{ID: "nobody"}, {ID: "b"}}
	if err := s.Save(ctx, gone); err != nil {
		t.Fatalf("leavers who were retired or moved on are skipped: %v", err)
	}
}

func TestOldInventoriesBecomeItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	s.UseItemCatalogue(func(name string) (domain.BaseItem, bool) {
		if name == "rations" {
			return domain.BaseItem{Key: "rations", Name: "Rations", Kind: domain.ItemGear, Value: 1, Stack: true}, true
		}
		return domain.BaseItem{}, false
	})
	h := hero("a", "Aria")
	h.Sheet.Inventory = []string{"rations (5)", "a bone key"}
	_ = s.CreateCharacter(ctx, h)
	got, err := s.Character(ctx, "a")
	if err != nil || len(got.Sheet.Items) != 2 || got.Sheet.Inventory != nil {
		t.Fatalf("converted on load: %+v %v", got.Sheet, err)
	}
	if r := got.Sheet.Items[0]; r.ID != "itm-a-0" || r.Base != "rations" || r.Qty != 5 {
		t.Fatalf("catalogue item with quantity: %+v", r)
	}
	if k := got.Sheet.Items[1]; k.Kind != domain.ItemKeepsake || k.Name != "a bone key" {
		t.Fatalf("unknown names become keepsakes: %+v", k)
	}
	if list, _ := s.CharactersByOwner(ctx, "me"); len(list[0].Sheet.Items) != 2 {
		t.Fatal("lists convert too")
	}
	g, party := adventure("g1", got)
	g.Creatures[0].Items, g.Creatures[0].Inventory = nil, []string{"rations"}
	_ = s.CreateAdventure(ctx, g, party)
	if loaded, _ := s.Load(ctx, "g1"); len(loaded.Creatures[0].Items) != 1 || loaded.Creatures[0].Items[0].Qty != 1 {
		t.Fatalf("games convert too: %+v", loaded.Creatures[0])
	}
}

func TestGenderSetOnTheRosterReachesTheAdventure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	_ = s.CreateCharacter(ctx, hero("a", "Maren"))
	a, _ := s.Character(ctx, "a")
	g, party := adventure("g1", a)
	g.Creatures = append(g.Creatures, &domain.Creature{ID: "ghost", Kind: domain.KindPC, Name: "Pre-roster"})
	if err := s.CreateAdventure(ctx, g, party); err != nil {
		t.Fatal(err)
	}
	// The owner sets the gender on the roster while she is away.
	a, _ = s.Character(ctx, "a")
	a.Sheet.Gender = domain.Female
	_ = s.SaveCharacter(ctx, a)
	if chk, _ := s.Character(ctx, "a"); chk.Sheet.Gender != domain.Female {
		t.Fatalf("roster save: %+v", chk)
	}

	// A save from a game loaded before that must not wipe it...
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Character(ctx, "a"); got.Sheet.Gender != domain.Female {
		t.Fatalf("write-back kept the roster's gender: %q", got.Sheet.Gender)
	}
	// ...and the next load of the game picks it up.
	loaded, err := s.Load(ctx, "g1")
	if err != nil {
		t.Fatal(err)
	}
	if maren, _ := loaded.Creature("a"); maren.Gender != domain.Female {
		t.Fatalf("game copy: %q", maren.Gender)
	}
}
