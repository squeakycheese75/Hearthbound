package play_test

import (
	"dnd/internal/domain"
	"errors"
	"strings"
	"testing"
)

func TestAskingAnOwnerForTheirHero(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	if err := h.uc.AskForHero(ctx, me, "g1", "asking"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	c, ok := g.CallFor("asking")
	if !ok || c.Kind != domain.CallAsk || c.Answerer(me) != friend || c.Summary != "level 1 Elf Wizard" || !g.OnTheBoard() {
		t.Fatalf("the ask waits for the owner: %+v", g.Calls)
	}
	if err := h.uc.AnswerCall(ctx, me, "g1", "asking", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("the leader can't answer their own ask: %v", err)
	}
	if err := h.uc.AnswerCall(ctx, "stranger", "g1", "asking", false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("strangers can't answer: %v", err)
	}
	if err := h.uc.AnswerCall(ctx, friend, "g1", "asking", true); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if _, err := g.Creature("asking"); err != nil || len(g.Calls) != 0 || !h.pub.has(domain.EvSystem, "Asking joins the party.") {
		t.Fatalf("the owner said yes: %+v %v", g.Calls, err)
	}
}

func TestDecliningAndWithdrawingCalls(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	_ = h.uc.AskForHero(ctx, me, "g1", "asking")
	if err := h.uc.AnswerCall(ctx, friend, "g1", "asking", false); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); len(g.Calls) != 0 || len(g.Party) != 2 || !h.pub.has(domain.EvSystem, "No Asking this time") {
		t.Fatalf("declined: %+v", g.Calls)
	}
	_ = h.uc.AskForHero(ctx, me, "g1", "asking")
	if err := h.uc.AnswerCall(ctx, me, "g1", "asking", false); err != nil || len(h.store.game(t).Calls) != 0 {
		t.Fatalf("the leader withdraws: %v", err)
	}
	if err := h.uc.AnswerCall(ctx, friend, "g1", "asking", true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("nothing to answer: %v", err)
	}
}

func TestAskRules(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	_ = h.uc.AskForHero(ctx, me, "g1", "asking")
	cases := []struct {
		name    string
		account string
		hero    string
		want    error
	}{
		{"only the leader asks", friend, "asking", domain.ErrNotFound},
		{"once", me, "asking", domain.ErrInvalid},
		{"auto heroes are recruited directly", me, "hired", domain.ErrInvalid},
		{"your own are recruited directly", me, "nim", domain.ErrInvalid},
		{"private heroes stay hidden", me, "theirs", domain.ErrNotFound},
		{"free heroes only", me, "busy", domain.ErrInvalid},
		{"real heroes only", me, "ghost", domain.ErrNotFound},
	}
	for _, tc := range cases {
		if err := h.uc.AskForHero(ctx, tc.account, "g1", tc.hero); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	g := h.store.game(t)
	g.Calls = append(g.Calls, domain.Call{Kind: domain.CallAsk, CharacterID: "x", OwnerID: friend})
	_ = h.store.Save(ctx, g)
	if err := h.uc.AskForHero(ctx, me, "g1", "hired"); !errors.Is(err, domain.ErrInvalid) || !strings.Contains(err.Error(), "full") {
		t.Fatalf("asks count toward a full party: %v", err)
	}
	if err := h.uc.AnswerCall(ctx, friend, "g1", "x", true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a hero gone from the roster: %v", err)
	}
	g = h.store.game(t)
	g.Scene.Key = "crypt"
	_ = h.store.Save(ctx, g)
	if err := h.uc.AskForHero(ctx, me, "g1", "hired"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("only at the tavern: %v", err)
	}
	if err := h.uc.AnswerCall(ctx, friend, "g1", "asking", true); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("joining waits for the tavern: %v", err)
	}
}

func TestPartyWantedPostsAndOffers(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	if err := h.uc.OfferHero(ctx, friend, "g1", "theirs"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no post, no offers: %v", err)
	}
	if err := h.uc.PostWanted(ctx, friend, "g1", "anyone"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("strangers can't post for the party: %v", err)
	}
	if err := h.uc.PostWanted(ctx, me, "g1", strings.Repeat("x", domain.MaxWanted+1)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("keep it short: %v", err)
	}
	if err := h.uc.PostWanted(ctx, me, "g1", "  Looking for   a healer\nfor the crypt. "); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Wanted != "Looking for a healer for the crypt." || !g.OnTheBoard() {
		t.Fatalf("posted: %q", g.Wanted)
	}
	if err := h.uc.OfferHero(ctx, me, "g1", "nim"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("the party's own players recruit instead: %v", err)
	}
	if err := h.uc.OfferHero(ctx, friend, "g1", "nim"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("only your own heroes: %v", err)
	}
	if err := h.uc.OfferHero(ctx, friend, "g1", "ghost"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("real heroes only: %v", err)
	}
	if err := h.uc.OfferHero(ctx, friend, "g1", "theirs"); err != nil {
		t.Fatal(err)
	}
	if !h.pub.has(domain.EvSystem, "Theirs, a level 1 Elf Wizard, would like to join") {
		t.Fatal("the leader hears of the offer")
	}
	if err := h.uc.AnswerCall(ctx, friend, "g1", "theirs", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("the owner can't accept their own offer: %v", err)
	}
	if err := h.uc.AnswerCall(ctx, me, "g1", "theirs", true); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if c, err := g.Creature("theirs"); err != nil || c.OwnerID != friend || len(g.Calls) != 0 {
		t.Fatalf("the leader welcomed them: %v", err)
	}
	if err := h.uc.PostWanted(ctx, me, "g1", ""); err != nil || h.store.game(t).OnTheBoard() {
		t.Fatalf("the post comes down: %v", err)
	}
}

func TestOnlyTheLeaderPostsAndAsks(t *testing.T) {
	t.Parallel()
	h := recruitTable(t)
	if err := h.uc.Recruit(ctx, me, "g1", "hired"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.PostWanted(ctx, friend, "g1", "Anyone?"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a friend at the table can't post: %v", err)
	}
	if err := h.uc.AskForHero(ctx, friend, "g1", "asking"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a friend at the table can't ask: %v", err)
	}
}
