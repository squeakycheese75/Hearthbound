package domain

import (
	"fmt"
	"strings"
)

// NPC is a non-player character the DM has put on the table: a person or a
// creature the party meets, like an innkeeper or a kobold guide. NPCs are
// story characters, not combatants; a fight spawns monsters separately.
type NPC struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Type is a person type (NPCTypes) or, when Monster is set, a monster key.
	Type     string `json:"type"`
	Monster  bool   `json:"monster,omitempty"`
	Role     string `json:"role"`
	Attitude string `json:"attitude"`
	Gender   Gender `json:"gender,omitempty"`
	Look     string `json:"look,omitempty"`
	// Follows means the NPC travels with the party from scene to scene.
	Follows bool `json:"follows,omitempty"`
	// Here means the NPC is in the current scene.
	Here bool `json:"here"`
}

// NPCTypes are the kinds of people with stock portraits.
var NPCTypes = []string{
	"commoner", "innkeeper", "merchant", "guard", "soldier", "noble", "priest", "sage",
	"farmer", "child", "elder", "thief", "sailor", "hunter", "blacksmith", "bard",
	"witch", "knight", "cultist", "beggar",
}

// Attitudes an NPC can hold towards the party.
var Attitudes = []string{"friendly", "neutral", "wary", "hostile"}

// NPCID derives an NPC's ID from its name.
func NPCID(name string) string {
	return "npc-" + artSlug(name)
}

// FindNPC returns the NPC with this name (case-insensitive), or nil.
func (g *Game) FindNPC(name string) *NPC {
	id := NPCID(name)
	for _, n := range g.NPCs {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// NPCsHere returns the NPCs in the current scene.
func (g *Game) NPCsHere() []*NPC {
	var out []*NPC
	for _, n := range g.NPCs {
		if n.Here {
			out = append(out, n)
		}
	}
	return out
}

// Describe is one line about an NPC for the DM's state digest.
func (n *NPC) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s, %s): %s", n.Name, n.Type, n.Attitude, n.Role)
	if n.Follows {
		b.WriteString("; travels with the party")
	}
	return b.String()
}

// StockNPCKey is the stock portrait for a person type.
func StockNPCKey(npcType string) string {
	return "npcs/" + artSlug(npcType)
}

// StockKeysForNPC is the stock art for an NPC, best match first: its
// monster's portrait, or its person type's (the gendered variant, then any).
func StockKeysForNPC(n *NPC) []string {
	if n.Monster {
		return []string{StockMonsterKey(n.Type)}
	}
	return gendered(StockNPCKey(n.Type), n.Gender)
}

// gendered lists a stock key's variant for a gender, then the key itself.
func gendered(key string, g Gender) []string {
	if g == "" {
		return []string{key}
	}
	return []string{key + g.suffix(), key}
}

// NPCArtKey is where a custom portrait of one game's NPC lives.
func NPCArtKey(gameID string, n *NPC) string {
	return "games/" + artSlug(gameID) + "/" + n.ID
}

// NPCPortrait describes a custom portrait of an NPC, billed to the leader.
func NPCPortrait(g *Game, n *NPC) ArtRequest {
	r := npcRequest(n.Type, n.Look, n.Monster, n.Gender, ArtStyle)
	r.Key, r.GameID, r.Payer = NPCArtKey(g.ID, n), g.ID, g.LeaderID
	return r
}

// npcRequest describes a portrait of a person type (or a creature) met on the road.
func npcRequest(npcType, look string, monster bool, g Gender, style string) ArtRequest {
	what := fmt.Sprintf("a %s, an ordinary person of a fantasy world", strings.TrimSpace(g.Word()+" "+strings.ReplaceAll(npcType, "_", " ")))
	if monster {
		what = fmt.Sprintf("a %s, a fantasy creature, shown as a character rather than a threat", strings.ReplaceAll(npcType, "_", " "))
	}
	if strings.TrimSpace(look) != "" {
		what += ": " + look
	}
	prompt := fmt.Sprintf("Head-and-shoulders portrait of %s. Expressive face, clothing and props that show who they are. "+
		"Facing the viewer, centred, softly blurred dark background. %s", what, style)
	return ArtRequest{Key: StockNPCKey(npcType) + g.suffix(), Kind: ArtNPC, Prompt: prompt, Size: PortraitSize}
}

// LastNarrationAt is the log position (counting trimmed events) of the latest
// narration, or -1 if there is none.
func (g *Game) LastNarrationAt() int {
	for i := len(g.Log) - 1; i >= 0; i-- {
		if g.Log[i].Type == EvNarration {
			return g.LogBase + i
		}
	}
	return -1
}

// CastStale reports whether the latest narration has not been checked for
// NPCs yet (scenes from before NPCs existed, or a failed check).
func (g *Game) CastStale() bool {
	at := g.LastNarrationAt()
	return g.Started && g.Mode == ModeExploration && at >= 0 && at+1 > g.CastAt
}

// PartyLevel is the highest level among living party members (at least 1).
func (g *Game) PartyLevel() int {
	level := 1
	for _, c := range g.PartyMembers() {
		if !c.Dead && c.Level > level {
			level = c.Level
		}
	}
	return level
}
