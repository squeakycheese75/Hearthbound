package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// ArtKind says what an illustration depicts.
type ArtKind string

// Kinds of illustration.
const (
	ArtPortrait ArtKind = "portrait"
	ArtMonster  ArtKind = "monster"
	ArtScene    ArtKind = "scene"
	// ArtBattlemap is a top-down floor for the combat grid (stock only).
	ArtBattlemap ArtKind = "battlemap"
	// ArtNPC is a portrait of a non-player character.
	ArtNPC ArtKind = "npc"
)

// SceneSubject is the subject ID used to reroll a game's current scene art.
const SceneSubject = "scene"

// BattlemapSubject is the art subject for the combat grid's floor.
const BattlemapSubject = "battlemap"

// Illustration sizes.
const (
	PortraitSize = "1024x1024"
	SceneSize    = "1536x1024"
)

// ArtStyle is appended to every prompt so the whole game shares one look. It
// is the style of the default art pack.
const ArtStyle = "Style: painted dark-fantasy tabletop RPG illustration, rich oil-painting brushwork, " +
	"dramatic warm candlelight and deep shadows, muted earthy palette with touches of gold, detailed and cinematic. " +
	"No text, lettering, captions, watermark, border or frame."

// ArtPack is a set of stock illustrations painted in one style. Its images
// live in web/public/stock/<Key>/ and every prompt ends with Style.
type ArtPack struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Style string `json:"style"`
}

// ArtPacks are the known styles. Add one, paint it with -paint-stock, and
// select it with -art-pack.
var ArtPacks = []ArtPack{
	{Key: "candlelit", Name: "Candlelit Oils", Style: ArtStyle},
}

// ArtPackFor finds a pack by key.
func ArtPackFor(key string) (ArtPack, error) {
	for _, p := range ArtPacks {
		if p.Key == key {
			return p, nil
		}
	}
	return ArtPack{}, fmt.Errorf("%w: art pack %q", ErrUnknown, key)
}

// ArtRequest asks the illustrator for one image. Key is where it is stored and
// doubles as its identity: the same key is never painted twice unless rerolled.
type ArtRequest struct {
	Key    string
	GameID string
	// Payer is the account whose image key pays for this painting.
	Payer  string
	Kind   ArtKind
	Prompt string
	Size   string
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func artSlug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// PortraitKey is where a party member's portrait lives.
func PortraitKey(gameID, creatureID string) string {
	return "games/" + artSlug(gameID) + "/" + artSlug(creatureID)
}

// MonsterKey is where a monster type's portrait lives; it is shared by every game.
func MonsterKey(kind string) string {
	return "monsters/" + artSlug(kind)
}

// SceneArtKey is where a game's backdrop for a scene lives.
func SceneArtKey(gameID string, s Scene) string {
	return "games/" + artSlug(gameID) + "/scene-" + artSlug(s.Key+" "+s.Title)
}

// PartyArtKey is where a party member's portrait lives: with the character for
// roster adventures, or with the game for older adventures.
func PartyArtKey(g *Game, c *Creature) string {
	if g.Roster {
		return CharacterArtKey(c.ID)
	}
	return PortraitKey(g.ID, c.ID)
}

// CreatureArtKey returns the art key for any creature: party members get their
// own portrait, monsters share one per type.
func CreatureArtKey(g *Game, c *Creature) string {
	if c.Kind == KindMonster {
		return MonsterKey(c.Portrait)
	}
	return PartyArtKey(g, c)
}

// CharacterPortrait describes a roster character's portrait, billed to its owner.
func CharacterPortrait(ch *Character) ArtRequest {
	r := PortraitRequest("", ch.Sheet)
	r.Key, r.Payer = CharacterArtKey(ch.ID), ch.OwnerID
	return r
}

var classLook = map[string]string{
	"fighter":   "wearing battered chain mail, a longsword at the hip and a shield on the back",
	"barbarian": "bare-armed and scarred, wearing furs, a great axe over one shoulder",
	"rogue":     "in dark fitted leathers and a hood, a rapier at the hip",
	"ranger":    "in a weathered green cloak and scale mail, a longbow and quiver on the back",
	"wizard":    "in travel-worn robes, holding a staff, a spellbook on a strap",
	"cleric":    "in scale mail with a shield, a glowing holy symbol on a chain",
	"bard":      "in colourful layered clothes, a lute slung across the back",
}

// PortraitFor describes a party member's portrait, billed to its owner or else the leader.
func PortraitFor(g *Game, c *Creature) ArtRequest {
	r := PortraitRequest(g.ID, c)
	r.Key, r.Payer = PartyArtKey(g, c), g.Payer(c)
	return r
}

// MonsterFor describes a monster portrait, billed to the game's leader.
func MonsterFor(g *Game, c *Creature) ArtRequest {
	r := MonsterRequest(g.ID, c)
	r.Payer = g.LeaderID
	return r
}

// SceneFor describes the game's current backdrop, billed to the leader.
func SceneFor(g *Game) ArtRequest {
	r := SceneRequest(g.ID, g.Scene)
	r.Payer = g.LeaderID
	return r
}

// PortraitRequest describes a head-and-shoulders portrait of a party member.
func PortraitRequest(gameID string, c *Creature) ArtRequest {
	return portraitRequest(gameID, c, ArtStyle)
}

func portraitRequest(gameID string, c *Creature, style string) ArtRequest {
	var b strings.Builder
	fmt.Fprintf(&b, "Head-and-shoulders portrait of %s, a %s adventurer", c.Name, strings.TrimSpace(c.Gender.Word()+" "+strings.ToLower(c.Race)+" "+strings.ToLower(c.Class)))
	if look := classLook[strings.ToLower(c.Class)]; look != "" {
		fmt.Fprintf(&b, ", %s", look)
	}
	b.WriteString(". ")
	if c.Persona != nil {
		fmt.Fprintf(&b, "Personality: %s ", c.Persona.Personality)
		if c.Persona.Backstory != "" {
			fmt.Fprintf(&b, "Backstory: %s ", c.Persona.Backstory)
		}
		b.WriteString("Let their expression and posture show this personality. ")
	}
	if c.Persona == nil {
		b.WriteString("Determined, heroic expression. ")
	}
	b.WriteString("Facing the viewer, centred, softly blurred dark background. ")
	b.WriteString(style)
	return ArtRequest{Key: PortraitKey(gameID, c.ID), GameID: gameID, Payer: c.OwnerID, Kind: ArtPortrait, Prompt: b.String(), Size: PortraitSize}
}

// MonsterRequest describes a portrait of a monster type, shared across games.
func MonsterRequest(gameID string, c *Creature) ArtRequest {
	return monsterRequest(gameID, c, ArtStyle)
}

func monsterRequest(gameID string, c *Creature, style string) ArtRequest {
	kind := strings.ReplaceAll(c.Portrait, "_", " ")
	look := monsterLooks[c.Portrait]
	if look == "" {
		look = "a classic fantasy creature"
	}
	prompt := fmt.Sprintf("Close-up character portrait of %s %s from a fantasy tabletop game: %s. "+
		"Fierce, watchful expression, stepping out of shadow toward the viewer, centred, dark smoky background. "+
		"Storybook fantasy, not gory. %s", article(kind), kind, look, style)
	return ArtRequest{Key: MonsterKey(c.Portrait), GameID: gameID, Kind: ArtMonster, Prompt: prompt, Size: PortraitSize}
}

// monsterLooks describe each SRD monster type for its portrait. Specific,
// calm descriptions paint better and pass image-safety filters more reliably
// than words like "menacing" or "threatening".
var monsterLooks = map[string]string{
	"kobold":         "a small, scaly, dog-like reptilian creature with rust-red scales, a short snout and bright eyes, wearing scraps of leather",
	"giant_rat":      "a rat the size of a dog with matted brown fur, long yellow incisors and a bald, curling tail",
	"bandit":         "a weathered human outlaw in a hooded cloak and patched leathers, a scarf over the lower face",
	"cultist":        "a robed figure in a deep crimson hood with a carved bone amulet, face half in shadow",
	"goblin":         "a small green-skinned goblin with long pointed ears, sharp yellow eyes and ragged leather armour",
	"skeleton":       "an animated human skeleton in rusted chain mail, a faint blue light in its empty eye sockets",
	"zombie":         "a shambling undead figure with grey skin and tattered burial clothes, eyes clouded white",
	"wolf":           "a grey timber wolf with a thick winter coat, ears forward and amber eyes",
	"orc":            "a tall, muscular orc warrior with grey-green skin, small tusks, a braided black topknot and crude fur-trimmed hide armour",
	"hobgoblin":      "a disciplined hobgoblin soldier with dark orange skin, a flat nose and polished red-lacquered armour",
	"gnoll":          "a lanky hyena-headed humanoid with spotted fur, a bristling mane and patchwork armour",
	"ghoul":          "a gaunt undead creature with stretched grey skin, long nails and pale hungry eyes",
	"bugbear":        "a big, shaggy, bear-like goblinoid with coarse brown fur and a heavy brow, in hide armour",
	"giant_spider":   "a huge black spider with glossy hairy legs and a cluster of glinting eyes, webs behind it",
	"dire_wolf":      "a massive wolf with a shaggy black coat and pale eyes, larger than a horse",
	"bandit_captain": "a confident outlaw leader with a scarred jaw, a feathered hat and a fine but worn coat",
	"ogre":           "a huge, hulking ogre with a heavy jaw, small eyes and a patchwork hide tunic",
	"owlbear":        "a great beast with the body of a bear and the feathered head and hooked beak of an owl",
}

// article picks "a" or "an" for a word.
func article(word string) string {
	if word != "" && strings.ContainsRune("aeiou", rune(word[0])) {
		return "an"
	}
	return "a"
}

// SceneRequest describes a wide establishing shot of a location.
func SceneRequest(gameID string, s Scene) ArtRequest {
	return sceneRequest(gameID, s, ArtStyle)
}

func sceneRequest(gameID string, s Scene, style string) ArtRequest {
	looks := s.Visual
	if strings.TrimSpace(looks) == "" {
		looks = "A " + s.Key + " setting."
	}
	prompt := fmt.Sprintf("Wide establishing shot of a fantasy location: %s. %s Mood: %s. "+
		"Environment only, no people in the foreground; leave the lower third darker and calm so text can sit over it. %s",
		s.Title, looks, s.Mood, style)
	return ArtRequest{Key: SceneArtKey(gameID, s), GameID: gameID, Kind: ArtScene, Prompt: prompt, Size: SceneSize}
}

// Stock art is painted once per art pack and shipped with the server: one
// backdrop per scene key, one portrait per monster type, and one per race and
// class. It shows wherever no custom painting exists, so play needs no image
// calls. Stock keys are relative to the pack's folder.

// StockSceneKey is the stock backdrop for a scene key.
func StockSceneKey(sceneKey string) string {
	return "scenes/" + artSlug(sceneKey)
}

// StockBattlemapKey is the stock top-down battle map for a scene key.
func StockBattlemapKey(sceneKey string) string {
	return "battlemaps/" + artSlug(sceneKey)
}

// StockMonsterKey is the stock portrait for a monster type.
func StockMonsterKey(kind string) string {
	return "monsters/" + artSlug(kind)
}

// StockPortraitKey is the stock portrait for a race and class.
func StockPortraitKey(race, class string) string {
	return "portraits/" + artSlug(race) + "-" + artSlug(class)
}

// StockKeysFor is the stock art for any creature, best match first: a
// monster's portrait, or a hero's race and class (gendered variant, then any).
func StockKeysFor(c *Creature) []string {
	if c.Kind == KindMonster {
		return []string{StockMonsterKey(c.Portrait)}
	}
	return gendered(StockPortraitKey(c.Race, c.Class), c.Gender)
}

// stockScenes describes each scene key's stock backdrop.
var stockScenes = map[string]string{
	"tavern":    "A crowded low-beamed tavern, a roaring hearth, tankards on scarred oak tables, lanterns and pipe smoke",
	"village":   "A small farming village of thatched cottages around a well, a muddy lane, smoke from chimneys at dusk",
	"town":      "A walled market town, a cobbled square with stalls and a stone fountain, timber-framed houses",
	"road":      "A lonely dirt road winding over moorland hills under a heavy sky, a weathered milestone",
	"forest":    "An ancient forest of huge mossy oaks, shafts of light through mist, a faint path between roots",
	"swamp":     "A fog-bound swamp of black water, drowned trees and reeds, will-o'-the-wisp lights in the distance",
	"mountains": "A narrow mountain pass between jagged snow-capped peaks, a precipice and a cold wind",
	"cave":      "A cave mouth opening into darkness, dripping stalactites, a pale glow deep inside",
	"crypt":     "A vaulted underground crypt, stone sarcophagi with carved lids, cobwebs, a single guttering torch",
	"dungeon":   "A dungeon corridor of rough-hewn stone, iron-barred cells, chains on the walls, torchlight",
	"castle":    "A grim castle on a crag, high walls and towers against storm clouds, a drawbridge",
	"camp":      "A wilderness campsite at night, a campfire, bedrolls and packs, dark trees all around",
	"ruins":     "Overgrown ruins of an ancient temple, broken columns and toppled statues swallowed by vines",
	"temple":    "The candlelit interior of a stone temple, an altar under a stained-glass window, pews",
}

// stockBattlemaps describes each scene key's floor as seen from above. The
// rules engine has no walls or obstacles yet, so scenery stays at the edges.
var stockBattlemaps = map[string]string{
	"tavern":    "a tavern's plank floor: the bar counter along the top edge, round tables and stools in the corners, a stone hearth at one side, rushes and spilled ale",
	"village":   "a village green: a stone well, a muddy cart track, cottage walls and fences along the edges, patches of grass",
	"town":      "a cobbled market square: a fountain off to one side, market stalls and crates along the edges, drains between the stones",
	"road":      "a dirt road crossing heathland: wheel ruts, a milestone, gorse and boulders along the verges",
	"forest":    "a forest clearing: mossy ground and leaf litter, huge tree trunks and roots around the edges, a fallen log",
	"swamp":     "a swamp island of soggy ground: black pools and reeds around the edges, drowned stumps, stepping stones",
	"mountains": "a rocky mountain ledge: grey stone and scree, a cliff edge along one side, patches of snow",
	"cave":      "a cave floor of uneven rock: stalagmites around the walls, a shallow underground stream along one edge, scattered bones",
	"crypt":     "a crypt floor of worn flagstones: stone sarcophagi along the walls, rubble, candles in alcoves",
	"dungeon":   "a dungeon chamber of rough flagstones: iron-barred cells along the walls, chains, a drain in the floor, straw",
	"castle":    "a castle courtyard: worn paving, a well, wooden training dummies and weapon racks along the walls",
	"camp":      "a wilderness campsite: a campfire ring off-centre, bedrolls and packs around it, a trampled grass clearing, trees at the edges",
	"ruins":     "ruined temple flagstones: toppled columns and broken statues around the edges, vines and grass through the cracks",
	"temple":    "a temple floor of polished stone tiles: an altar on a dais along the top edge, rows of benches along the sides, candle stands",
}

// battlemapRequest describes a top-down battle map for a scene key, sized to
// the 12x8 combat grid (3:2).
func battlemapRequest(sceneKey, style string) ArtRequest {
	looks := stockBattlemaps[sceneKey]
	if looks == "" {
		looks = "a " + sceneKey + " area"
	}
	prompt := fmt.Sprintf("Top-down battle map for a tabletop role-playing game, seen straight down from directly above (orthographic, no perspective): %s. "+
		"Keep the middle open, flat and walkable, with scenery around the edges. No grid lines, no people, creatures or tokens, no labels. "+
		"Evenly lit so the whole floor reads clearly. %s", looks, style)
	return ArtRequest{Key: StockBattlemapKey(sceneKey), Kind: ArtBattlemap, Prompt: prompt, Size: SceneSize}
}

// StockList is every stock image in a pack with the prompt that paints it,
// for the given races, classes and monster types.
func StockList(pack ArtPack, races, classes, monsters []string) []ArtRequest {
	var out []ArtRequest
	for _, key := range SceneKeys {
		r := sceneRequest("", Scene{Key: key, Title: key, Mood: "atmospheric", Visual: stockScenes[key]}, pack.Style)
		r.Key = StockSceneKey(key)
		out = append(out, r)
	}
	for _, key := range SceneKeys {
		out = append(out, battlemapRequest(key, pack.Style))
	}
	for _, t := range NPCTypes {
		for _, g := range append([]Gender{""}, Genders...) {
			out = append(out, npcRequest(t, "", false, g, pack.Style))
		}
	}
	for _, m := range monsters {
		r := monsterRequest("", &Creature{Portrait: m}, pack.Style)
		r.Key = StockMonsterKey(m)
		out = append(out, r)
	}
	for _, race := range races {
		for _, class := range classes {
			for _, g := range append([]Gender{""}, Genders...) {
				r := portraitRequest("", &Creature{Name: "a hero", Race: race, Class: class, Gender: g}, pack.Style)
				r.Key = StockPortraitKey(race, class) + g.suffix()
				out = append(out, r)
			}
		}
	}
	return out
}
