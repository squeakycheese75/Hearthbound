// Package demo is an offline stand-in for the Claude agents. It plays a short
// canned scene through the real rules-engine tools so the UI and game loop can
// be tried without an API key. It is not an AI: the narration is scripted.
package demo

import (
	"context"
	"dnd/internal/adapter/claude"
	"dnd/internal/domain"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// Agent answers like a very predictable table.
type Agent struct {
	delay time.Duration
	seq   atomic.Int64
}

// New returns a demo agent that streams text with the given per-word delay.
func New(delay time.Duration) *Agent { return &Agent{delay: delay} }

// Complete produces the next scripted reply for the transcript.
func (a *Agent) Complete(ctx context.Context, req claude.Request) (claude.Response, error) {
	last := req.History[len(req.History)-1]
	var msg domain.AgentMessage
	switch {
	case req.Seat == domain.RoleCast:
		msg = domain.AgentMessage{Text: "none"} // the script always shows its NPCs itself
	case hasTool(req.Tools, "choose_action"):
		msg = a.companion(req.System, last)
	default:
		msg = a.dm(req.History, last)
	}
	if err := a.stream(ctx, msg.Text, req.OnText); err != nil {
		return claude.Response{}, err
	}
	msg.Role = domain.RoleAssistant
	msg.Raw = json.RawMessage(`{"role":"assistant","content":[]}`)
	return claude.Response{Message: msg}, nil
}

func (a *Agent) stream(ctx context.Context, text string, onText func(string)) error {
	if onText == nil || text == "" {
		return nil
	}
	for _, w := range strings.SplitAfter(text, " ") {
		select {
		case <-ctx.Done():
			return fmt.Errorf("demo stream: %w", ctx.Err())
		case <-time.After(a.delay):
		}
		onText(w)
	}
	return nil
}

func hasTool(tools []domain.ToolSpec, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func (a *Agent) call(name, input string) domain.AgentMessage {
	id := fmt.Sprintf("demo_%d", a.seq.Add(1))
	return domain.AgentMessage{ToolCalls: []domain.ToolCall{{ID: id, Name: name, Input: json.RawMessage(input)}}}
}

// withChoices hands the turn back to the party with quick-choice buttons.
func (a *Agent) withChoices(text string, choices ...string) domain.AgentMessage {
	b, _ := json.Marshal(map[string][]string{"choices": choices})
	msg := a.call("offer_choices", string(b))
	msg.Text = text
	return msg
}

// afterFight are the choices once the goblins are beaten.
var afterFight = []string{"Study the goblins' map", "Rest by the fire until dawn", "Set out for the crypt", "Search the bodies for more clues"}

func (a *Agent) dm(history []domain.AgentMessage, last domain.AgentMessage) domain.AgentMessage {
	if len(last.ToolResults) > 0 {
		return a.afterTool(last.ToolResults[0].Content)
	}
	// Match on what the party did, not on the [State] digest (gear names like
	// "Priest's Pack" would otherwise read as "rest").
	text, _, _ := strings.Cut(strings.ToLower(last.Text), "[state]")
	switch {
	case len(history) == 1:
		return a.call("set_scene", `{"scene_key":"tavern","title":"The Gilded Flagon","mood":"Rain lashes the shutters; the hearth roars.","visual":"A crooked timber-framed tavern common room, low smoke-blackened beams, a roaring stone hearth, rain streaking the leaded windows."}`)
	case strings.Contains(text, "whole party is down"):
		return a.call("end_adventure", `{"epilogue":"The goblins drag their prizes into the rain. In the Gilded Flagon they still tell of the heroes who fell by the hearth."}`)
	case strings.Contains(text, "combat is over") && firstHero(last.Text) != "":
		return a.call("roll_loot", fmt.Sprintf(`{"character_id":%q,"quality":"good"}`, firstHero(last.Text)))
	case strings.Contains(text, "combat is over"):
		return a.withChoices("Silence returns, broken only by the rain. Among the goblins' rags you find a crude map, scrawled in charcoal, pointing toward the old crypt beyond the hill.\n\nWhat do you do?", afterFight...)
	case strings.HasPrefix(text, "round "):
		return domain.AgentMessage{Text: "Steel rings against rusted iron as the brawl spills across the tavern floor, tables overturned and tankards flying."}
	case containsAny(text, "rest", "heal", "sleep", "camp", "bandage"):
		return a.call("rest", `{"kind":"long"}`)
	case foughtAlready(history):
		return a.afterwards(text)
	}
	return a.beforeFight(text)
}

// beforeFight plays the tavern scene until the goblins are unmasked.
func (a *Agent) beforeFight(text string) domain.AgentMessage {
	switch {
	case containsAny(text, "attack", "fight", "charge", "draw my", "strike"):
		return a.call("start_combat", `{"monsters":[{"key":"goblin","count":2}]}`)
	case containsAny(text, "look", "search", "inspect", "listen"):
		return a.call("ability_check", `{"character_id":"pc1","skill":"perception","dc":12,"reason":"scanning the common room"}`)
	case containsAny(text, "innkeeper", "ask"):
		return a.call("show_npc", `{"name":"Bryn","type":"innkeeper","role":"innkeeper of the Gilded Flagon","attitude":"friendly","gender":"f","look":"A broad, flour-dusted woman with a grey braid and a wary smile."}`)
	}
	return a.withChoices("The innkeeper leans in, lowering her voice. \"Two of those hooded folk by the door have been watching you since you came in. Goblins, if you ask me, wearing borrowed cloaks.\"\n\nThe hooded figures shift, hands drifting beneath their cloaks. What do you do?",
		"Look around carefully", "Attack the hooded figures", "Buy them a drink and listen", "Warn my companions quietly")
}

// afterwards follows the trail to the crypt once the goblins are beaten.
func (a *Agent) afterwards(text string) domain.AgentMessage {
	switch {
	case containsAny(text, "crypt", "set out", "travel", "head", "follow the map"):
		return a.call("set_scene", `{"scene_key":"crypt","title":"The Old Crypt","mood":"Cold air breathes from the dark.","visual":"A moss-choked stone crypt entrance on a rain-swept hill at night, a broken iron gate, faint green light deep inside."}`)
	case containsAny(text, "bodies", "search"):
		return a.call("show_npc", `{"name":"Snik","type":"kobold","role":"the goblins' kobold prisoner","attitude":"wary","gender":"m","look":"A scrawny rust-scaled kobold with rope-burned wrists.","follows":true}`)
	case containsAny(text, "map", "study", "clue"):
		return a.call("update_quest", `{"title":"Find the crypt's master","status":"open","note":"A goblin map marks a hill-crypt an hour north, with MASTER scrawled by the door."}`)
	}
	return a.withChoices("The tavern settles back into an uneasy quiet. Through the rain-streaked window, the hill to the north is a black shape against the clouds.\n\nWhat do you do?", afterFight...)
}

var heroLine = regexp.MustCompile(`(?m)^- (\S+) [^:]+: L\d`)

// firstHero finds the first party member's ID in a [State] digest.
func firstHero(text string) string {
	if m := heroLine.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// foughtAlready reports whether the goblin fight has happened in this story.
func foughtAlready(history []domain.AgentMessage) bool {
	for _, m := range history {
		if strings.Contains(strings.ToLower(m.Text), "combat is over") {
			return true
		}
	}
	return false
}

// afterFind narrates meeting an NPC or finding loot.
func (a *Agent) afterFind(result string) (domain.AgentMessage, bool) {
	switch {
	case strings.HasPrefix(result, "Bryn is on the table"):
		return a.withChoices("Bryn the innkeeper leans in, lowering her voice. \"Two of those hooded folk by the door have been watching you since you came in. Goblins, if you ask me, wearing borrowed cloaks.\"\n\nWhat do you do?",
			"Look around carefully", "Attack the hooded figures", "Buy them a drink and listen"), true
	case strings.HasPrefix(result, "Snik is on the table"):
		return a.withChoices("Behind the overturned table, a kobold in rope bonds cringes from you. \"Snik not goblin! Snik knows the crypt. Snik shows the way, yes, if big folk cut the rope.\"\n\nWhat do you do?",
			"Cut Snik free and set out for the crypt", "Study the goblins' map", "Rest by the fire until dawn"), true
	case strings.Contains(result, " finds "):
		return a.withChoices("Silence returns, broken only by the rain. Among the goblins' rags you find a crude map, scrawled in charcoal, pointing toward the old crypt beyond the hill, and something worth keeping.\n\nWhat do you do?", afterFight...), true
	}
	return domain.AgentMessage{}, false
}

// afterTool narrates the result of the tool the script just called.
func (a *Agent) afterTool(result string) domain.AgentMessage {
	if msg, ok := a.afterFind(result); ok {
		return msg
	}
	switch {
	case strings.HasPrefix(result, "Shown to the players"):
		return domain.AgentMessage{}
	case strings.Contains(result, "too deadly"):
		return a.call("start_combat", `{"monsters":[{"key":"goblin","count":1}]}`) // small party: one goblin
	case strings.HasPrefix(result, "Combat started"):
		return domain.AgentMessage{Text: "The cloaks fall away. A goblin leaps onto the table, blade bared and cackling!"}
	case strings.HasPrefix(result, "New quest"), strings.HasPrefix(result, "Journal updated"):
		return a.withChoices("The charcoal map marks a hill-crypt an hour's walk north, with a crude skull drawn over the door and the word \"MASTER\" scrawled beside it. One goblin also carries a bone key, still warm.\n\nWhat do you do?", afterFight...)
	case strings.HasPrefix(result, "The party takes a"):
		return a.withChoices("You bank the fire and sleep in shifts while the rain beats on. By dawn, wounds have closed and spirits have lifted.\n\nWhat do you do?", afterFight...)
	case strings.Contains(result, "The Old Crypt"):
		return a.withChoices("The crypt door hangs open on one hinge. Somewhere below, something drags itself across stone.\n\n(This is where the demo's script ends. Run `make debug` with an Anthropic key for a real AI Dungeon Master.)",
			"Light a torch and go in", "Listen at the door", "Turn back to town")
	case strings.Contains(result, "SUCCESS"):
		return a.withChoices("Your eyes catch the glint of a blade beneath a hooded figure's cloak by the door, and a tell-tale green ear. Goblins, in disguise.", "Attack the hooded figures", "Warn my companions quietly", "Buy them a drink and listen")
	case strings.Contains(result, "FAILURE"):
		return a.withChoices("Smoke and shadow hide the room's corners; nothing seems amiss, though the back of your neck prickles.", "Attack the hooded figures", "Ask the innkeeper about them", "Look around carefully")
	}
	return a.withChoices("Rain drums on the roof of the Gilded Flagon. A traveller's tale of a haunted crypt has the whole room whispering, and three strangers' eyes keep finding yours.\n\nWhat do you do?",
		"Confront the hooded figures", "Ask the innkeeper about them", "Slip out the back to follow them", "Draw my sword and attack")
}

// demoLines gives each personality archetype its own scripted reply.
var demoLines = map[string]string{
	"Gallant Knight":         `"Stand behind me, friends. By my oath, no harm comes to you tonight." *steps toward the strangers, chin high*`,
	"Wisecracking Scoundrel": `"Hooded, brooding, staring... either cultists or very bad poets." *palms a coin from the next table*`,
	"Bookish Scholar":        `"Fascinating. Those cloak clasps match a sigil in Varrow's Bestiary, third edition." *flips through a battered notebook*`,
	"Grumpy Veteran":         `"Trouble. Always trouble." *finishes the ale in one swallow and loosens a blade*`,
	"Chaotic Free Spirit":    `"Ooh, mysterious strangers! I'm going to say hello." *already halfway across the room, waving*`,
	"Quiet Wanderer":         `"Two by the door. One at the bar." *shifts to watch the back exit*`,
	"Pious Zealot":           `"I smell the grave on them. The light will judge." *grips a holy symbol until the knuckles whiten*`,
	"Reluctant Hero":         `"Could we... perhaps not get stabbed tonight?" *sighs, then quietly moves to cover your back*`,
}

func (a *Agent) companion(system string, last domain.AgentMessage) domain.AgentMessage {
	if len(last.ToolResults) > 0 {
		return domain.AgentMessage{}
	}
	if strings.Contains(last.Text, "Your options:") {
		return a.call("choose_action", `{"option":0,"line":"Have at you!"}`)
	}
	for archetype, line := range demoLines {
		if strings.Contains(system, "Your persona: "+archetype) {
			return domain.AgentMessage{Text: line}
		}
	}
	return domain.AgentMessage{Text: `"I've got your back." *loosens a blade in its sheath and watches the door*`}
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}
