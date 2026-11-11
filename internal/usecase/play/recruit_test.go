package play_test

import (
	"dnd/internal/domain"
	"errors"
	"strings"
	"testing"
)

func rosterHero(t *testing.T, id, name, owner string, status domain.CharacterStatus) domain.Character {
	t.Helper()
	sheet := build(t, id, name, "Wizard", "Elf", domain.ControlAI, nil)
	sheet.OwnerID = owner
	return domain.Character{ID: id, OwnerID: owner, Sheet: sheet, Status: status}
}

func recruitTable(t *testing.T) *harness {
	t.Helper()
	g := newGame(t)
	g.Started, g.Roster = true, true
	h := setup(t, g)
	h.store.chars = map[string]domain.Character{
		"nim":    rosterHero(t, "nim", "Nim", me, domain.CharIdle),
		"oslo":   rosterHero(t, "oslo", "Oslo", me, domain.CharIdle),
		"busy":   rosterHero(t, "busy", "Busy", me, domain.CharAdventuring),
		"theirs": rosterHero(t, "theirs", "Theirs", friend, domain.CharIdle),
		"hired":  forHire(rosterHero(t, "hired", "Hired", friend, domain.CharIdle), domain.HireAuto),
		"asking": forHire(rosterHero(t, "asking", "Asking", friend, domain.CharIdle), domain.HireAsk),
	}
	return h
}

func forHire(ch domain.Character, p domain.HirePolicy) domain.Character {
	ch.Hire = p
	return ch
}

func TestRecruitAnotherPlayersHeroForHire(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	if err := h.uc.Recruit(ctx, me, "g1", "asking"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("an ask-first hero needs an invite: %v", err)
	}
	if err := h.uc.Recruit(ctx, me, "g1", "hired"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	hired, _ := g.Creature("hired")
	if hired.OwnerID != friend || g.Payer(hired) != friend || !g.CanView(friend) || hired.Controller != domain.ControlAI {
		t.Fatalf("the friend's hero joins on the friend's key, and the friend can see the game: %+v", hired)
	}
}

func TestRecruitAtTheTavern(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	if err := h.uc.Recruit(ctx, me, "g1", "nim"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	nim, err := g.Creature("nim")
	if err != nil || len(g.Party) != 3 || nim.Controller != domain.ControlAI || nim.OwnerID != me {
		t.Fatalf("Nim joins as an agent-played companion: %+v %v", g.Party, err)
	}
	if len(h.store.joined) != 1 || h.store.joined[0] != "nim" || !h.pub.has(domain.EvSystem, "Nim joins the party.") || !h.art.has("characters/nim") {
		t.Fatal("the store embarks Nim; the table is told; the portrait is requested")
	}
	if len(h.agent.turns) != 0 {
		t.Fatal("recruiting costs no model call")
	}
	if err := h.uc.Act(ctx, me, "g1", "I welcome Nim."); err != nil {
		t.Fatal(err)
	}
	if msg := lastDMMessage(t, h); !strings.HasPrefix(msg, "Changes at the table: Nim (nim, a level 1 Elf Wizard) has just joined the party here; introduce them.") ||
		!strings.Contains(msg, "Nim (nim, played by its agent)") {
		t.Fatalf("the DM introduces the newcomer: %s", msg)
	}
	if g := h.store.game(t); len(g.TableNotes) != 0 || !strings.Contains(h.agent.seatTurns(domain.DMSeat)[0].System, "- nim: Nim, Elf Wizard.") {
		t.Fatal("notes are delivered once; the DM's party list includes Nim")
	}
	if err := h.uc.Recruit(ctx, me, "g1", "oslo"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Recruit(ctx, me, "g1", "nim"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("the party is full at four: %v", err)
	}
}

func TestRecruitRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		account, char string
		mutate        func(*domain.Game)
		want          error
	}{
		"not the leader":       {friend, "nim", nil, domain.ErrNotFound},
		"leader of a stranger": {me, "theirs", nil, domain.ErrNotFound},
		"unknown hero":         {me, "ghost", nil, domain.ErrNotFound},
		"already away":         {me, "busy", nil, domain.ErrInvalid},
		"out in the woods":     {me, "nim", func(g *domain.Game) { g.Scene.Key = "forest" }, domain.ErrInvalid},
		"mid-fight":            {me, "nim", func(g *domain.Game) { g.Mode = domain.ModeCombat }, domain.ErrInvalid},
		"after the end":        {me, "nim", func(g *domain.Game) { g.Mode = domain.ModeOver }, domain.ErrInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := recruitTable(t)
			g := h.store.game(t)
			if tc.mutate != nil {
				tc.mutate(g)
			}
			_ = h.store.Save(ctx, g)
			if err := h.uc.Recruit(ctx, tc.account, "g1", tc.char); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	h := recruitTable(t)
	g := h.store.game(t)
	cleric, _ := g.Creature("pc2")
	cleric.OwnerID = friend
	_ = h.store.Save(ctx, g)
	if err := h.uc.Recruit(ctx, friend, "g1", "theirs"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a party member who is not the leader cannot recruit (yet): %v", err)
	}
	before := newGame(t) // not started: recruiting before setting out is fine anywhere
	before.Scene.Key = "forest"
	hb := setup(t, before)
	hb.store.chars = map[string]domain.Character{"nim": rosterHero(t, "nim", "Nim", me, domain.CharIdle)}
	if err := hb.uc.Recruit(ctx, me, "g1", "nim"); err != nil {
		t.Fatalf("before setting out: %v", err)
	}
}

func TestSendHome(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	if err := h.uc.SendHome(ctx, me, "g1", "pc1"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("you cannot send home the character you are playing: %v", err)
	}
	if err := h.uc.SendHome(ctx, friend, "g1", "pc2"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("only the owner sends a character home: %v", err)
	}
	if err := h.uc.SendHome(ctx, me, "g1", "nim"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("only party members can be sent home: %v", err)
	}
	g := h.store.game(t)
	g.Pending, g.Covering = map[string]string{"pc2": "x"}, map[string]bool{"pc2": true}
	g.Transcripts["pc2"] = []domain.AgentMessage{{Role: domain.RoleUser, Text: "hi"}}
	_ = h.store.Save(ctx, g)
	if err := h.uc.SendHome(ctx, me, "g1", "pc2"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if _, err := g.Creature("pc2"); err == nil || len(g.Party) != 1 || g.Pending["pc2"] != "" || g.Covering["pc2"] || g.Transcripts["pc2"] != nil {
		t.Fatalf("Mae is gone, seat state and all: %+v", g.Party)
	}
	if len(h.store.left) != 1 || h.store.left[0] != "pc2" || !h.pub.has(domain.EvSystem, "Mae leaves the party and heads home.") {
		t.Fatal("the store settles Mae; the table is told")
	}
	if !strings.Contains(strings.Join(g.TableNotes, " "), "Mae (pc2) has left the party") {
		t.Fatal("the DM hears about it next turn")
	}
	hero, _ := g.Creature("pc1")
	hero.Dead = true
	_ = h.store.Save(ctx, g)
	if err := h.uc.SendHome(ctx, me, "g1", "pc1"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("the last member stays: %v", err)
	}
	g.Scene.Key = "cave"
	_ = h.store.Save(ctx, g)
	if err := h.uc.SendHome(ctx, me, "g1", "pc1"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("only at a tavern: %v", err)
	}
}

func TestCheckCastRunsOnceForOldScenes(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Started = true
	g.Log = []domain.Event{{Type: domain.EvNarration, Text: "Old Tom leans on his hoe."}, {Type: domain.EvSystem, Text: "x"}}
	g.LogBase = 10
	h := setup(t, g)
	h.agent.on("cast", call("show_npc", `{"name":"Old Tom","type":"farmer","role":"farmer","attitude":"friendly","gender":"m"}`), say("Done."))
	if err := h.uc.CheckCast(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if g.FindNPC("Old Tom") == nil || g.CastAt != 11 || g.CastStale() {
		t.Fatalf("the old scene's NPC is found and the check recorded: castAt %d", g.CastAt)
	}
	if err := h.uc.CheckCast(ctx, me, "g1"); err != nil || len(h.agent.seatTurns("cast")) != 2 {
		t.Fatal("an up-to-date cast is not checked again")
	}
}
