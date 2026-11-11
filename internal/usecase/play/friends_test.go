package play_test

import (
	"dnd/internal/domain"
	"errors"
	"slices"
	"testing"
)

func TestAFriendTakesASeatAndGivesItBack(t *testing.T) {
	t.Parallel()
	h := setup(t, newGame(t))
	h.agent.on(domain.DMSeat, say("The road calls."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.OfferSeat(ctx, me, "g1", "pc2", true); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if !g.SeatOpen("pc2") || !h.pub.has(domain.EvSystem, "Mae is looking for a player") {
		t.Fatalf("seat offered: %v", g.OpenSeats)
	}
	if err := h.uc.TakeSeat(ctx, friend, "Pip", "g1", "pc2"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	cleric, _ := g.Creature("pc2")
	if cleric.PlayerID != friend || !domain.Live(cleric) || g.SeatOpen("pc2") || !g.CanView(friend) {
		t.Fatalf("seated: %+v open %v", cleric, g.OpenSeats)
	}
	if g.Payer(cleric) != friend || cleric.OwnerID != me {
		t.Fatal("the friend pays for the seat; the owner keeps the character")
	}
	if !slices.Contains(g.WaitingFor(), friend) || !h.pub.has(domain.EvSystem, "Pip joins the table to play Mae") {
		t.Fatalf("the table waits for the friend: %v", g.WaitingFor())
	}
	if err := h.uc.Act(ctx, friend, "g1", "I bless the party."); err != nil {
		t.Fatalf("the friend acts for their seat: %v", err)
	}
	if err := h.uc.GiveSeatBack(ctx, friend, "g1", "pc2"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	cleric, _ = g.Creature("pc2")
	if cleric.PlayerID != "" || domain.Live(cleric) || g.CanView(friend) {
		t.Fatalf("back with its owner's agent: %+v", cleric)
	}
}

func TestSeatRules(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Creatures[1].Dead = true
	h := setup(t, g)
	cases := []struct {
		name string
		do   func() error
		want error
	}{
		{"only the leader offers", func() error { return h.uc.OfferSeat(ctx, friend, "g1", "pc2", true) }, domain.ErrNotFound},
		{"not the fallen", func() error { return h.uc.OfferSeat(ctx, me, "g1", "pc2", true) }, domain.ErrInvalid},
		{"a party member (the fallen are laid to rest)", func() error { return h.uc.OfferSeat(ctx, me, "g1", "pc2", true) }, domain.ErrNotFound},
		{"not someone being played", func() error { return h.uc.OfferSeat(ctx, me, "g1", "pc1", true) }, domain.ErrInvalid},
		{"closing an offer is fine", func() error { return h.uc.OfferSeat(ctx, me, "g1", "pc1", false) }, nil},
		{"a seat must be open", func() error { return h.uc.TakeSeat(ctx, friend, "Pip", "g1", "pc2") }, domain.ErrNotFound},
		{"giving back needs a seat", func() error { return h.uc.GiveSeatBack(ctx, me, "g1", "pc1") }, domain.ErrInvalid},
	}
	for _, tc := range cases {
		if err := tc.do(); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

func TestSeatOffersAndTakers(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	extra := build(t, "pc3", "Lark", "Bard", "Elf", domain.ControlAI, g.Creatures[1].Persona)
	extra.OwnerID = me
	g.Creatures = append(g.Creatures, extra)
	g.Party = append(g.Party, "pc3")
	g.OpenSeats = []string{"pc2", "pc3"}
	h := setup(t, g)
	if err := h.uc.TakeSeat(ctx, me, "James", "g1", "pc2"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("the leader plays their own: %v", err)
	}
	if err := h.uc.TakeSeat(ctx, friend, "Pip", "g1", "pc2"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.TakeSeat(ctx, friend, "Pip", "g1", "pc3"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("one seat per friend: %v", err)
	}
	if err := h.uc.OfferSeat(ctx, me, "g1", "pc2", true); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a seat a friend holds can't be offered: %v", err)
	}
	if err := h.uc.OfferSeat(ctx, friend, "g1", "pc3", false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a friend at the table can't offer seats: %v", err)
	}
	if err := h.uc.GiveSeatBack(ctx, me, "g1", "pc2"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("only the friend holding a seat gives it back: %v", err)
	}
	if err := h.uc.OfferSeat(ctx, me, "g1", "pc3", false); err != nil || h.store.game(t).SeatOpen("pc3") {
		t.Fatalf("the leader closes an offer: %v", err)
	}
	g = h.store.game(t)
	g.Mode = domain.ModeCombat
	_ = h.store.Save(ctx, g)
	if err := h.uc.OfferSeat(ctx, me, "g1", "pc3", true); !errors.Is(err, domain.ErrWrongMode) {
		t.Fatalf("not during a fight: %v", err)
	}
	g.Creatures = g.Creatures[:2]
	g.OpenSeats = []string{"ghost"}
	_ = h.store.Save(ctx, g)
	if err := h.uc.TakeSeat(ctx, "other", "Sam", "g1", "ghost"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a seat whose character is gone: %v", err)
	}
}
