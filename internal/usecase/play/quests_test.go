package play_test

import (
	"dnd/internal/domain"
	"strings"
	"testing"
)

func TestTheDMKeepsTheJournal(t *testing.T) {
	t.Parallel()
	g := newGame(t)
	for i := range domain.MaxQuests - 1 {
		g.Quests = append(g.Quests, domain.Quest{Title: "Old errand " + string(rune('A'+i)), Status: domain.QuestOpen})
	}
	g.Quests[0].Status = domain.QuestDone
	h := setup(t, g)
	h.agent.on(domain.DMSeat, calls(
		"update_quest", `{"title":"Find Elsie","status":"open","note":"Last seen by the mill."}`,
		"update_quest", `{"title":"find elsie","status":"open"}`,
		"update_quest", `{"title":"Rescue the miller","status":"open"}`,
		"update_quest", `{"title":"Burn the hive","status":"done"}`,
		"update_quest", `{"title":"","status":"open"}`,
		"update_quest", `{"title":"x","status":"lost"}`,
		"update_quest", `{"title":"x","status":"open","note":"`+strings.Repeat("n", domain.MaxQuestNote+1)+`"}`,
		"update_quest", `{"title":"Find Elsie","status":"failed"}`,
		"update_quest", `{"title":"Old errand B","status":"done"}`,
		"update_quest", `{"title":"Rescue the miller","status":"done","note":"Freed from the cellar."}`,
	), say("The hunt goes on."))
	if err := h.uc.Act(ctx, me, "g1", "We ask about Elsie."); err != nil {
		t.Fatal(err)
	}
	g = h.store.game(t)
	if len(g.Quests) != domain.MaxQuests {
		t.Fatalf("the journal stays at %d: %+v", domain.MaxQuests, g.Quests)
	}
	byTitle := map[string]domain.Quest{}
	for _, q := range g.Quests {
		byTitle[q.Title] = q
	}
	if q := byTitle["Find Elsie"]; q.Status != domain.QuestFailed || q.Note != "Last seen by the mill." {
		t.Fatalf("one quest per title (any case), notes kept: %+v", q)
	}
	if _, ok := byTitle["Old errand A"]; ok {
		t.Fatal("a full journal forgets the oldest finished quest")
	}
	if _, ok := byTitle["Burn the hive"]; ok {
		t.Fatal("…and a full journal of open quests refuses new ones")
	}
	for _, want := range []string{"New quest: Find Elsie.", "Quest failed: Find Elsie.", "Journal updated: find elsie.", "Quest complete: Rescue the miller."} {
		if !h.pub.has(domain.EvSystem, want) {
			t.Errorf("the table sees %q", want)
		}
	}
	h.agent.on(domain.DMSeat, say("Onward."))
	if err := h.uc.Act(ctx, me, "g1", "We rest a moment."); err != nil {
		t.Fatal(err)
	}
	if msg := lastDMMessage(t, h); !strings.Contains(msg, "Journal:\n") || !strings.Contains(msg, "- [failed] Find Elsie: Last seen by the mill.\n") {
		t.Fatalf("the DM sees the journal in [State]: %s", msg)
	}
}
