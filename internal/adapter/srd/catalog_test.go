package srd_test

import (
	"dnd/internal/adapter/srd"
	"dnd/internal/domain"
	"strings"
	"testing"
)

func TestEveryClassAndRaceBuilds(t *testing.T) {
	t.Parallel()
	cat := srd.New()
	for _, cls := range cat.Classes() {
		for _, race := range cat.Races() {
			for level := 1; level <= 3; level++ {
				c, err := domain.BuildCharacter(cls, race, domain.BuildSpec{ID: "x", Name: "Test", Level: level})
				if err != nil {
					t.Fatal(err)
				}
				if c.HP < 6 || c.AC < 10 || len(c.Actions) == 0 {
					t.Errorf("%s %s L%d: hp=%d ac=%d actions=%d", race.Name, cls.Name, level, c.HP, c.AC, len(c.Actions))
				}
			}
		}
	}
}

func TestFighterNumbers(t *testing.T) {
	t.Parallel()
	cat := srd.New()
	cls, _ := cat.Class("fighter")
	race, _ := cat.Race("human")
	c, _ := domain.BuildCharacter(cls, race, domain.BuildSpec{ID: "x", Name: "Bram", Level: 1})
	// STR 15+1=16 (+3), CON 14+1=15 (+2): HP 12, AC 16+2 = 18, longsword +5 1d8+3.
	sword, _ := c.Action("longsword")
	if c.HP != 12 || c.AC != 18 || sword.ToHit != 5 || sword.Dice.String() != "1d8+3" {
		t.Fatalf("hp=%d ac=%d tohit=%d dice=%s", c.HP, c.AC, sword.ToHit, sword.Dice)
	}
}

func TestMonsters(t *testing.T) {
	t.Parallel()
	cat := srd.New()
	for _, k := range cat.MonsterKeys() {
		m, err := cat.Monster(k, "m1")
		if err != nil || m.HP <= 0 || len(m.Actions) == 0 {
			t.Fatalf("%s: %v", k, err)
		}
	}
	if owlbear, _ := cat.Monster("owlbear", "o"); !owlbear.HasFeature(domain.FeatExtraAttack) {
		t.Fatal("the owlbear's Multiattack")
	}
	if goblin, _ := cat.Monster("goblin", "g"); len(goblin.Features) != 0 {
		t.Fatal("goblins attack once")
	}
	if _, err := cat.Monster("tarrasque", "x"); err == nil {
		t.Fatal("unknown monster should error")
	}
	if cat.DMStyle("nope").Key != "heroic" {
		t.Fatal("default style")
	}
}

func TestNamesAndBackstories(t *testing.T) {
	t.Parallel()
	cat := srd.New()
	for _, r := range cat.Races() {
		if len(cat.Names(r.Name, domain.Male)) < 6 || len(cat.Names(r.Name, domain.Female)) < 6 {
			t.Errorf("%s needs name ideas", r.Name)
		}
	}
	if len(cat.Names("Robot", domain.Female)) == 0 || len(cat.Backstories()) == 0 {
		t.Fatal("fallback names and backstories must not be empty")
	}
	for _, b := range cat.Backstories() {
		if len(b) > 400 {
			t.Errorf("backstory too long for a character: %q", b)
		}
	}
}

func TestEveryMonsterHasAFightingStyle(t *testing.T) {
	t.Parallel()
	cat := srd.New()
	for _, k := range cat.MonsterKeys() {
		m, err := cat.Monster(k, "x")
		if err != nil || m.Tactic == "" {
			t.Errorf("%s needs a tactic (%v)", k, err)
		}
	}
	if !strings.Contains(cat.MonsterSummary(), "goblin: Goblin (CR 1/4, 50 XP, AC 15, HP 7, fights as a skirmisher)") {
		t.Fatal("the DM should see how monsters fight")
	}
}

func TestItemCatalogue(t *testing.T) {
	t.Parallel()
	c := srd.New()
	seen := map[string]bool{}
	for _, b := range c.Items() {
		if seen[b.Key] || b.Name == "" || b.Kind == "" {
			t.Fatalf("bad or duplicate item %+v", b)
		}
		seen[b.Key] = true
	}
	for _, cls := range c.Classes() {
		for _, g := range cls.Gear {
			name := strings.TrimSpace(strings.Split(g, "(")[0])
			if _, ok := c.ItemByName(name); !ok {
				t.Errorf("%s's starting gear %q isn't in the catalogue", cls.Name, g)
			}
		}
	}
	if b, ok := c.ItemByName("Chain Mail"); !ok || b.AC != 16 {
		t.Fatalf("lookup by name: %+v", b)
	}
	if _, err := c.Item("longsword"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Item("vorpal-blade"); err == nil {
		t.Fatal("unknown keys fail")
	}
}
