package play_test

import (
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"dnd/internal/usecase/play"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

const friend = "acct-friend"

// twoPlayers is a party where the hero is played by me and the cleric by a friend.
func twoPlayers(t *testing.T, pace string) *domain.Game {
	t.Helper()
	g := newGame(t)
	cleric, _ := g.Creature("pc2")
	cleric.OwnerID, cleric.Controller = friend, domain.ControlHuman
	g.Pace = domain.PaceFor(pace)
	return g
}

func lastDMMessage(t *testing.T, h *harness) string {
	t.Helper()
	turns := h.agent.seatTurns(domain.DMSeat)
	if len(turns) == 0 {
		t.Fatal("the DM was never called")
	}
	hist := turns[len(turns)-1].History
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Text != "" {
			return hist[i].Text
		}
	}
	return ""
}

func TestRoundWaitsForEveryLivePlayer(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "off"))
	if err := h.uc.Act(ctx, me, "g1", "I open the door."); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if len(h.agent.seatTurns(domain.DMSeat)) != 0 || g.Pending["pc1"] != "I open the door." {
		t.Fatal("the round must wait for the second player")
	}
	if w := g.WaitingFor(); len(w) != 1 || w[0] != friend {
		t.Fatalf("waiting for %v", w)
	}
	if !h.pub.has(domain.EvSystem, "Waiting for Mae") {
		t.Fatal("players should be told who the table is waiting for")
	}
	if err := h.uc.Act(ctx, me, "g1", "Again!"); !errors.Is(err, domain.ErrNotYourTurn) {
		t.Fatalf("acting twice in a round: %v", err)
	}
	if err := h.uc.Act(ctx, friend, "g1", "I follow, mace raised."); err != nil {
		t.Fatal(err)
	}
	msg := lastDMMessage(t, h)
	if !strings.Contains(msg, "I open the door.") || !strings.Contains(msg, "I follow, mace raised.") || strings.Count(msg, "played by its owner") != 2 {
		t.Fatalf("the DM should get both players' actions: %s", msg)
	}
	if g := h.store.game(t); len(g.Pending) != 0 || !g.Deadline.IsZero() {
		t.Fatal("a resolved round clears its actions; pace off sets no deadline")
	}
}

func TestDeadlineLetsAgentsActForAbsentPlayers(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "fast"))
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if len(h.agent.turns) != 0 {
		t.Fatal("nothing is due before the adventure begins")
	}
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if want := h.clock.Now().Add(2 * time.Minute); !g.Deadline.Equal(want) {
		t.Fatalf("deadline %v, want %v", g.Deadline, want)
	}
	if err := h.uc.Act(ctx, me, "g1", "I search the cellar."); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if n := len(h.agent.seatTurns(domain.DMSeat)); n != 1 {
		t.Fatalf("too early: the DM must not resolve yet (%d calls)", n)
	}
	h.clock.advance(3 * time.Minute)
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	msg := lastDMMessage(t, h)
	if !strings.Contains(msg, "I search the cellar.") || !strings.Contains(msg, "while its owner is away") {
		t.Fatalf("the absent player's agent should act: %s", msg)
	}
	if comp := h.agent.seatTurns("pc2"); len(comp) != 1 || comp[0].Payer != friend {
		t.Fatal("the stand-in agent is billed to the absent player")
	}
	if !h.pub.has(domain.EvSystem, "Time is up") {
		t.Fatal("players should see why the round moved on")
	}
	if g := h.store.game(t); !g.Deadline.After(h.clock.Now()) {
		t.Fatal("a fresh deadline opens the next round")
	}
}

func TestCombatDeadlineLetsAgentTakeTheTurn(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Pace = domain.PaceFor("fast")
	h := setup(t, g)
	h.cat.monster = dummy(500, 0)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if g.Encounter.Current() != "pc1" || !g.Deadline.Equal(h.clock.Now().Add(time.Minute)) {
		t.Fatalf("the hero's turn should carry the combat deadline: %v", g.Deadline)
	}
	h.clock.advance(2 * time.Minute)
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if !h.pub.has(domain.EvRoll, "Aria attacks") {
		t.Fatal("the hero's agent should have attacked")
	}
	if !strings.Contains(h.agent.seatTurns("pc1")[0].System, "Every companion is played by an agent") {
		t.Fatal("the stand-in agent's prompt should say no one else is playing live")
	}
	g = h.store.game(t)
	if g.Encounter.Current() != "pc1" || g.Encounter.Round != 2 || !domain.Live(g.Creatures[0]) {
		t.Fatal("next round the table waits for the player again")
	}
	deadline := g.Deadline
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", false); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); !g.Deadline.Equal(deadline) {
		t.Fatal("taking control of the character already on the clock changes nothing")
	}
}

func TestAwayPlayersAreCoveredOrHeldBack(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "fast"))
	round := func() {
		t.Helper()
		if err := h.uc.Act(ctx, me, "g1", "I search."); err != nil {
			t.Fatal(err)
		}
		h.clock.advance(3 * time.Minute)
		if err := h.uc.Expire(ctx, "g1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	round()
	if g := h.store.game(t); !g.Covering["pc2"] || g.Covering["pc1"] || !h.pub.has(domain.EvSystem, "Mae's agent covers for their player") {
		t.Fatalf("the away player's character is marked as agent-covered: %v", g.Covering)
	}
	if err := h.uc.Act(ctx, friend, "g1", "I'm back."); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Covering["pc2"] {
		t.Fatal("acting again ends the cover")
	}
	if err := h.uc.Cover(ctx, friend, "g1", "pc2", false); err != nil {
		t.Fatal(err)
	}
	before := len(h.agent.seatTurns("pc2"))
	round()
	if g := h.store.game(t); !g.Hold["pc2"] || g.Covering["pc2"] || len(h.agent.seatTurns("pc2")) != before {
		t.Fatal("without permission the agent must not stand in")
	}
	if !strings.Contains(lastDMMessage(t, h), "Mae (pc2, owner away): holds back") || !h.pub.has(domain.EvSystem, "Mae holds back") {
		t.Fatal("the DM and table hear that the character holds back")
	}
	if err := h.uc.Cover(ctx, friend, "g1", "pc2", true); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Hold["pc2"] {
		t.Fatal("allowing cover clears the hold")
	}
	if err := h.uc.Cover(ctx, "stranger", "g1", "pc2", true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("strangers cannot change seats: %v", err)
	}
	if err := h.uc.Cover(ctx, me, "g1", "pc2", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("only the owner sets cover: %v", err)
	}
}

func TestCombatTurnHeldBackWhenCoverIsOff(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Pace = domain.PaceFor("fast")
	h := setup(t, g)
	h.cat.monster = dummy(500, 0)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Cover(ctx, me, "g1", "pc1", false); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(2 * time.Minute)
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if h.pub.has(domain.EvRoll, "Aria attacks") || !h.pub.has(domain.EvSystem, "Aria holds back") || len(h.agent.seatTurns("pc1")) != 0 {
		t.Fatal("a held character skips its turn instead of being played by its agent")
	}
	if g := h.store.game(t); g.Encounter.Round != 2 || g.Encounter.Current() != "pc1" {
		t.Fatal("the fight moves on and waits for the player next round")
	}
}

func TestSpendIsLedgeredPerSeat(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "fast"))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Act(ctx, me, "g1", "I search."); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(3 * time.Minute)
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	dm, mae, choices := g.Spend[domain.DMSeat], g.Spend["pc2"], g.Spend["choices"]
	if dm.Payer != me || dm.Calls != 2 || dm.Total.Input != 20 || dm.Last.Output != 5 {
		t.Fatalf("the DM's ledger bills the leader: %+v", dm)
	}
	if mae.Payer != friend || mae.Calls != 1 {
		t.Fatalf("the covering agent bills its owner: %+v", mae)
	}
	if choices.Payer != me || choices.Calls != 2 {
		t.Fatalf("the choice helper has its own line: %+v", choices)
	}
	sum := domain.TokenUsage{}
	for _, s := range g.Spend {
		sum.Add(s.Total)
	}
	if sum != g.Usage {
		t.Fatalf("the ledger adds up to the game total: %+v vs %+v", sum, g.Usage)
	}
	if dmMax, comp := h.uc.Limits(); dmMax != 60_000 || comp != 25_000 {
		t.Fatalf("chapter limits %d/%d", dmMax, comp)
	}
}

func TestExpireOnFinishedGameDoesNothing(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Mode, g.Started, g.Deadline = domain.ModeOver, true, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	h := setup(t, g)
	if err := h.uc.Expire(ctx, "g1"); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); !g.Deadline.IsZero() || len(h.agent.turns) != 0 {
		t.Fatal("finished games just drop their deadline")
	}
}

func TestAutopilotHandsSeatsToAgents(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "fast"))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Autopilot(ctx, friend, "g1", "pc2", true); err != nil {
		t.Fatal(err)
	}
	if len(h.agent.seatTurns("pc2")) != 0 || !h.pub.has(domain.EvSystem, "Mae's agent takes over") {
		t.Fatal("handing over waits for the remaining live player")
	}
	if err := h.uc.Act(ctx, me, "g1", "Onward."); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lastDMMessage(t, h), "Mae (pc2, played by its agent)") {
		t.Fatal("the cleric should now be played by her agent")
	}
	dmCalls := len(h.agent.seatTurns(domain.DMSeat))
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", true); err != nil {
		t.Fatal(err)
	}
	g := h.store.game(t)
	if len(h.agent.seatTurns(domain.DMSeat)) != dmCalls || len(h.agent.seatTurns("pc1")) != 0 || !g.Deadline.IsZero() || g.Playing() {
		t.Fatal("with nobody playing, the story pauses: no agent turns and no deadline")
	}
	if !h.pub.has(domain.EvSystem, "the story pauses") {
		t.Fatal("players should be told the story is paused")
	}
	h.clock.advance(time.Hour)
	if err := h.uc.Expire(ctx, "g1"); err != nil || len(h.agent.seatTurns(domain.DMSeat)) != dmCalls {
		t.Fatalf("a paused story ignores the clock: %v", err)
	}
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", false); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if !domain.Live(g.Creatures[0]) || !h.pub.has(domain.EvSystem, "Aria's player takes control") || g.Deadline.IsZero() {
		t.Fatal("taking back control resumes the story and starts the clock again")
	}
	if err := h.uc.Autopilot(ctx, me, "g1", "pc9", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("someone else's or a missing seat: %v", err)
	}
	if err := h.uc.Autopilot(ctx, friend, "g1", "pc1", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a friend cannot autopilot my hero: %v", err)
	}
}

func TestSwitchingCharacters(t *testing.T) {
	t.Parallel()
	g := newGame(t) // I own both: Aria (live) and Mae (agent)
	h := setup(t, g)
	if err := h.uc.Autopilot(ctx, me, "g1", "pc2", false); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	aria, _ := g.Creature("pc1")
	mae, _ := g.Creature("pc2")
	if domain.Live(aria) || !domain.Live(mae) || !h.pub.has(domain.EvSystem, "Aria's agent takes over") {
		t.Fatal("taking control of Mae hands Aria to her agent: one character per player")
	}
	if err := h.uc.Act(ctx, me, "g1", "I bless the party."); err != nil {
		t.Fatal(err)
	}
	if msg := lastDMMessage(t, h); !strings.Contains(msg, "Mae (pc2, played by its owner)") || !strings.Contains(msg, "Aria (pc1, played by its agent)") {
		t.Fatalf("the player now acts as Mae: %s", msg)
	}
	g = h.store.game(t)
	aria, _ = g.Creature("pc1")
	aria.Dead = true
	_ = h.store.Save(ctx, g)
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", false); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("the fallen cannot be played: %v", err)
	}
}

func TestFallenHeroHandsThePlayerAnotherCharacter(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Started = true
	hero, _ := g.Hero()
	hero.Dead, hero.HP = true, 0
	h := setup(t, g)
	if err := h.uc.Autopilot(ctx, me, "g1", "pc2", true); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if mae, _ := g.Creature("pc2"); !domain.Live(mae) || !h.pub.has(domain.EvSystem, "Aria has fallen. Their player now plays Mae.") {
		t.Fatal("a player whose hero falls takes over one of their living characters")
	}
}

func TestAutopilotInCombat(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "off"))
	h.cat.monster = dummy(500, 0)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Encounter.Current() != "pc1" {
		t.Fatalf("expected the hero to start, got %s", g.Encounter.Current())
	}
	if err := h.uc.Autopilot(ctx, friend, "g1", "pc2", true); err != nil {
		t.Fatal(err)
	}
	if h.pub.has(domain.EvSystem, "the story pauses") {
		t.Fatal("a friend handing over is fine: the leader is still playing")
	}
	if err := h.uc.Fight(ctx, friend, "g1", rules.Option{}); !errors.Is(err, domain.ErrNotYourTurn) {
		t.Fatalf("fighting on someone else's turn: %v", err)
	}
	opts, _ := h.uc.Options(ctx, me, "g1")
	if err := h.uc.Fight(ctx, me, "g1", opts[0]); err != nil {
		t.Fatal(err)
	}
	if !h.pub.has(domain.EvRoll, "Aria attacks") || !h.pub.has(domain.EvRoll, "Mae attacks") {
		t.Fatal("Aria fights, then Mae's agent takes her turn")
	}
	if g := h.store.game(t); g.Encounter.Current() != "pc1" {
		t.Fatalf("the fight waits for the leader again, current %s", g.Encounter.Current())
	}
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", true); err != nil {
		t.Fatal(err)
	}
	if !h.pub.has(domain.EvSystem, "the story pauses") || h.store.game(t).Playing() {
		t.Fatal("the leader handing over pauses the fight: nobody plays on with their DM")
	}
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", false); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); !g.Playing() || g.Encounter.Current() != "pc1" {
		t.Fatal("taking control resumes the fight on the leader's turn")
	}
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", false); err != nil {
		t.Fatalf("taking control twice is harmless: %v", err)
	}
	g := h.store.game(t)
	g.Mode = domain.ModeOver
	_ = h.store.Save(ctx, g)
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", false); err != nil {
		t.Fatalf("switching seats after the end is harmless: %v", err)
	}
}

func TestOnlyTheLeaderKeepsTheStoryGoing(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "fast"))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Autopilot(ctx, me, "g1", "pc1", true); err != nil {
		t.Fatal(err)
	}
	dm := len(h.agent.seatTurns(domain.DMSeat))
	if err := h.uc.Act(ctx, friend, "g1", "I carry on alone."); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(time.Hour)
	_ = h.uc.Expire(ctx, "g1")
	if g := h.store.game(t); g.Playing() || len(h.agent.seatTurns(domain.DMSeat)) != dm || !g.Deadline.IsZero() {
		t.Fatal("a friend can't keep playing on the leader's DM while the leader isn't playing")
	}
}

func TestFightPausesWhenTheLastPlayerFalls(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	hero, _ := g.Hero()
	hero.HP, hero.MaxHP = 1, 1
	cleric, _ := g.Creature("pc2")
	cleric.OwnerID = friend // agent-played, and not mine to take over
	h := setup(t, g)
	h.cat.monster = func(key, id string) (*domain.Creature, error) {
		m, _ := dummy(500, 0)(key, id)
		m.Abilities[domain.DEX], m.Actions[0].ToHit, m.Actions[0].Dice = 30, 20, domain.MustDice("1d4+10")
		return m, nil
	}
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Go."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if aria, _ := g.Creature("pc1"); !aria.Dead || g.Playing() || g.Mode != domain.ModeCombat {
		t.Fatalf("expected a slain hero and a paused fight: dead=%v mode=%s", aria.Dead, g.Mode)
	}
	if !h.pub.has(domain.EvSystem, "the story pauses") || h.pub.has(domain.EvRoll, "Mae attacks") {
		t.Fatal("the agents must not finish the fight alone")
	}
}

func TestHandingOverLastOwedSeatResolvesTheRound(t *testing.T) {
	t.Parallel()
	h := setup(t, twoPlayers(t, "off"))
	if err := h.uc.Act(ctx, me, "g1", "I light a torch."); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Autopilot(ctx, friend, "g1", "pc2", true); err != nil {
		t.Fatal(err)
	}
	if msg := lastDMMessage(t, h); !strings.Contains(msg, "I light a torch.") || !strings.Contains(msg, "Mae (pc2, played by its agent)") {
		t.Fatalf("once the only player left has acted, the round resolves: %s", msg)
	}
}

func TestDownedHeroesComeRoundAfterVictory(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	mae, _ := g.Creature("pc2")
	mae.HP, mae.Stable = 0, true
	mae.AddCondition(domain.Unconscious)
	h := setup(t, g)
	h.cat.monster = dummy(1, 100)
	h.agent.on(domain.DMSeat, call("start_combat", `{"monsters":[{"key":"dummy","count":1}]}`), say("Fight!"), say("You win."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	opts, err := h.uc.Options(ctx, me, "g1")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.uc.Fight(ctx, me, "g1", findOption(t, opts, "longsword")); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	mae, _ = g.Creature("pc2")
	if g.Mode != domain.ModeExploration || mae.HP != 1 || mae.Has(domain.Unconscious) {
		t.Fatalf("Mae should come round after the fight: mode=%s hp=%d", g.Mode, mae.HP)
	}
	if !strings.Contains(lastDMMessage(t, h), "Mae comes round") {
		t.Fatal("the DM should hear who came round before narrating the aftermath")
	}
}

func compactingTable(t *testing.T, g *domain.Game) *harness {
	t.Helper()
	h := setup(t, g)
	h.uc = play.New(h.store, h.agent, h.roles, h.pub, h.dice, h.cat, h.art, h.clock, slog.New(slog.DiscardHandler), play.Config{CompactAt: 30}) // companions at 12
	return h
}

func TestChapterBreakCompactsEveryHistory(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	g.Log = []domain.Event{{Type: domain.EvSpeech, Actor: "ghost", Text: "Boo."}, {Type: domain.EvPlayer, Actor: "pc1", Text: "Hello."}}
	h := compactingTable(t, g)
	big := say("Rain lashes the mill. What do you do?")
	big.Usage = domain.TokenUsage{Input: 4, CacheRead: 26} // the DM's prompt has reached the limit
	recapTry := call("set_scene", `{"scene_key":"crypt","title":"x","mood":"y","visual":"z"}`)
	h.agent.on(domain.DMSeat, big, recapTry, say("Elsie is missing; the miller lies. The party stands at the mill."), say("The door creaks."))
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if g.Chapter != 1 || !strings.HasPrefix(g.Recap, "Elsie is missing") || g.Scene.Key == "crypt" || len(g.Context) != 0 {
		t.Fatalf("chapter %d recap %q scene %s context %v", g.Chapter, g.Recap, g.Scene.Key, g.Context)
	}
	if r := g.Spend["recap"]; r.Calls != 2 || r.Payer != me {
		t.Fatalf("recaps have their own ledger line: %+v", r)
	}
	dm := g.Transcripts[domain.DMSeat]
	if len(dm) != 1 || !strings.Contains(dm[0].Text, "end of chapter 1") || !strings.Contains(dm[0].Text, "Rain lashes the mill") {
		t.Fatalf("the DM restarts from its notes and its last narration: %+v", dm)
	}
	if mae := g.Transcripts["pc2"]; len(mae) != 1 || !strings.Contains(mae[0].Text, "Elsie is missing") || !strings.Contains(mae[0].Text, "ghost: <player_content>Boo.</player_content>\nAria: <player_content>Hello.</player_content>\nDM: Rain lashes") {
		t.Fatalf("companions restart from the recap and recent events: %+v", mae)
	}
	if h.pub.has(domain.EvNarration, "Elsie is missing") || h.pub.has(domain.EvThinking, "Elsie is missing") || !h.pub.has(domain.EvSystem, "Chapter 1 ends.") {
		t.Fatal("the recap is private; players only see the chapter break")
	}
	if err := h.uc.Act(ctx, me, "g1", "I knock."); err != nil {
		t.Fatal(err)
	}
	turns := h.agent.seatTurns(domain.DMSeat)
	if next := turns[len(turns)-1].History; len(next) != 2 || !strings.Contains(next[0].Text, "end of chapter 1") {
		t.Fatalf("the next DM call carries only the notes and the new round: %d messages", len(next))
	}
}

func TestChapterBreakWaitsAndSurvivesFailures(t *testing.T) {
	t.Parallel()
	small := say("All is calm.")
	h := compactingTable(t, newGame(t))
	h.agent.on(domain.DMSeat, small)
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Chapter != 0 || len(g.Transcripts[domain.DMSeat]) != 2 {
		t.Fatal("no chapter break below the limit")
	}

	h = compactingTable(t, newGame(t))
	big := say("A storm rolls in.")
	big.Usage = domain.TokenUsage{CacheRead: 30}
	h.agent.on(domain.DMSeat, big, say("")) // the DM writes no recap
	if err := h.uc.Begin(ctx, me, "g1"); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Chapter != 0 || len(g.Transcripts[domain.DMSeat]) < 2 {
		t.Fatal("a failed recap keeps the full histories")
	}

	g := newGame(t)
	g.Started = true
	h = compactingTable(t, g)
	h.agent.on(domain.DMSeat, say("Mae has heard it all before."), say("The night deepens."))
	chatty := say("Onward.")
	chatty.Usage = domain.TokenUsage{CacheRead: 12}
	h.agent.on("pc2", chatty)
	if err := h.uc.Act(ctx, me, "g1", "We press on."); err != nil {
		t.Fatal(err)
	}
	if g := h.store.game(t); g.Chapter != 1 {
		t.Fatal("a companion's long history also closes the chapter (its limit is 5/12 of the DM's)")
	}
}
