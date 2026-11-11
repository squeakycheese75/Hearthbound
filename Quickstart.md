# Quickstart

## Requirements

- Go 1.26+ and Node 20+ (Node only for building the UI)
- A Claude API key (`ANTHROPIC_API_KEY`, or a profile from `ant auth login`).
  Not needed for `make demo`.
- Optional: an OpenAI key (`OPENAI_API_KEY`) for painted portraits and scenes.
  Claude reads images but does not generate them.

## Build and run

```sh
make web     # build the browser UI into web/dist (embedded in the binary)
make build   # build bin/tabletop
export ANTHROPIC_API_KEY=sk-ant-...
make debug   # http://127.0.0.1:8080, with debug logging to data/tabletop.log
```

Rebuild with `make web && make build` after pulling changes: the UI is
embedded in the binary.

## Three ways to run

| Command | Who | Save file | Agents use |
|---|---|---|---|
| `make run` / `make debug` | just you (`-local`, signed in automatically as `local`) | `data/tabletop.db` | your keys from Settings, else `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` from the shell |
| `make serve` | you + invited friends (accounts, invites) | `data/tabletop.db` | each player's own keys from Settings |
| `make demo` | trying the UI for free | **`data/demo.db`** | nothing: the DM and companions are **scripted**, not AI |

The demo always uses its own save file and shows an orange **DEMO** badge.
Never point it at your real save with `-db`: its scripted turns would be
written into your real adventures.

## How a session plays

0. **Register your AI.** Add your Anthropic key in Settings (or, with
   `-local`, export `ANTHROPIC_API_KEY`). You can't start an adventure without
   one: your key runs the DM, the choice helper and your own characters' agents.
1. **Build your roster.** Each hero you create (class, ancestry, personality,
   backstory) lives on your account. They keep their XP, levels, gold and gear
   from one adventure to the next, have one portrait that follows them
   everywhere, and keep a chronicle of their adventures. A hero is **in town**,
   **adventuring** (one adventure at a time) or **fallen**. Survivors come home
   fully rested; the dead stay fallen. Retire heroes from their character page.
2. **Gather your party.** Pick the hero you'll play and up to three companions
   from your roster, the DM's style (Heroic, Grim, Comedic, Mystery) and a pace.
   Short on heroes? **Roll a random hero**, **Random companion** or **Roll a
   random party** creates level 1 heroes with a random ancestry, personality,
   name and backstory. Random companions get a class the party doesn't have
   yet. Every rolled hero joins your roster like any other.
   Deleting an adventure sends its party home with what they earned, marked
   "(abandoned)" in their chronicle.
   **The party can change in a tavern, town or village** (and before you set
   out), outside combat. The leader's **+ Recruit from the tavern** button (on
   the scene image) opens the tavern: **your heroes in town**, **other
   players' heroes looking for a party**, or **someone new** (roll a hero on
   the spot). Pick anyone to see their full sheet, then recruit (up to four in
   the party). Only heroes in town can join; anyone already in a party is
   unavailable. Newcomers are played by their agent and the DM introduces them
   next turn. Another player's hero runs on its owner's key, and the owner can
   take it over (*Play as …*) at any time. Set a hero's **Looking for
   adventure?** on their character page: *Not for hire*, *Looking for
   adventure* (anyone can recruit them), or *Ask me first* (listed, but joining
   waits for invites, which arrive with the notice board). **Send home**
   on one of your agent-played companions sends them back to your roster with
   what they earned, marked "(left the party)" in their chronicle. Play as
   someone else first to send home the character you're playing.
3. **Seats and pace.** Each party member is either **live** (its owner plays
   it) or **agent-played** (its personality plays it). You play one character
   at a time: your hero starts live and companions start as agents. Every
   character you own is playable. *Play as …* on a party card switches you to
   that character and hands your previous one to its agent. *Let my agent
   play* hands your current one over. If your character falls, you take over
   one of your living characters in the party.
   **The story only moves while the party's leader plays one of their
   characters,** because the leader's AI runs the Dungeon Master. If the leader
   hands everything to agents, loses their last hero, or leaves, the story
   pauses, and nobody plays on with someone else's DM. (An agent covering while
   the leader is away still counts: the seat is still theirs.) The leader picks
   *Play as …* to resume.
   **Every character in a party has its own agent, tied to that party.** When a
   hero dies, they're retired at the next quiet moment outside combat: they
   leave the party, their agent is detached, and they go to the **Hall of the
   Fallen** on the home page with "(fell in battle)" in their chronicle. A hero
   who leaves a party loses that party's agent too; recruited into a new party,
   they get a fresh one there. The pace sets how long the table
   waits for live players:

   | Pace | Exploration round | Combat turn |
   |---|---|---|
   | Daily (default) | 12 h | 2 h |
   | Relaxed | 24 h | 4 h |
   | Brisk | 1 h | 10 min |
   | Fast (testing) | 2 min | 1 min |
   | Always wait for me | never | never |

   When a deadline passes, absent players' characters are played by their
   agents for that round, on their owners' keys, and the seats stay theirs.
   That is how "my agent covers for me while I'm busy" works. Each party card
   carries a **Live** or **AI** badge, and a covered seat reads "Away · agent
   covering" until its player acts again. Turn off **My agent covers while I'm
   away** on your card and your character holds back instead (no tokens spent). The home page
   marks adventures where it's **your turn**.
4. **People you meet.** When someone who matters joins the scene (an
   innkeeper, a guide, a kobold prisoner), the DM puts them on the table. They
   appear on the scene with their portrait, and under **Here** with their role
   and attitude (friendly, neutral, wary, hostile). Changing scene leaves them
   behind unless they travel with you; earlier ones are listed as "Met
   earlier". The DM sees who is present every turn. If the DM narrates
   someone without putting them on the table, a small check after each
   narration (the `cast` role) adds them; opening an older adventure checks its
   latest scene once. NPCs are people you meet, not recruits.
5. **Exploration.** Type what your hero says or does (up to 500 characters), or click one of the DM's
   **quick choices**. Once every live player has acted, agent seats react in
   character and the DM resolves the round. When an outcome is uncertain, the
   DM calls the rules engine (checks, saves, damage, loot, XP) and the dice
   animate with the engine's real result. If the DM forgets to offer choices,
   a small separate call writes them from the latest narration.
6. **Combat.** When the DM starts a fight, the engine takes over: initiative,
   movement on the grid, attacks and spells, death saves. On your turn, pick
   an action and a target (from the list or the glowing tokens on the map).
   Monsters carry numbered badges that match the Foes list; the slain stay on
   the map, faded. Agent seats choose from their legal options, monsters use
   engine tactics, and the DM narrates each round.
   - **Potions of Healing** work in a fight. As your action, drink one (2d4+2
     HP) or pour one into a hurt or dying ally beside you, which brings them
     round. Companions use them too.
   - **Turned undead flee.** A creature turned by a cleric spends its turns
     dashing away from the party until it is hurt or the fight ends.
   - **Multiattack:** the owlbear and the bandit captain make two attacks with
     their action.
   - **Opportunity attacks.** Moving out of a foe's reach lets it strike with
     its reaction, once per round, using its melee attack. This applies to
     heroes, companions and monsters alike, including fleeing undead. A hero
     struck down on the way stops where they fell. **Disengage** (an action,
     offered beside a foe) falls back out of reach without provoking.
7. **Journal and rests.** The DM keeps the party's goals in a **Journal** under
   the party panel (`update_quest`): open quests first, then those done or
   failed, each with a short note. The DM sees the journal in every turn's
   state. Beside the quick choices, **Short rest** and **Long rest** send a rest
   as your action; the DM still decides whether it's safe, then calls the
   engine's `rest`.

Every turn saves the game and writes the party's sheets back to their characters.

## Your save data

Everything lives in `data/`:

| Path | What |
|---|---|
| `data/tabletop.db` (+ `-wal`, `-shm`) | accounts, characters, adventures (SQLite) |
| `data/demo.db` | the demo's separate save |
| `data/media/` | generated images (`characters/`, `monsters/`, `games/`) |
| `data/secret.key` | master key that encrypts stored API keys (unless `TABLETOP_SECRET_KEY` is set) |
| `data/backups/` | backups |
| `data/tabletop.log` | log written by `make debug` |

**Back up** at any time, even while the server runs (a consistent copy):

```sh
make backup          # data/backups/tabletop-<UTC time>.db
```

A server started with `-backup-dir` (or `TABLETOP_BACKUP_DIR`) also backs up by
itself: a minute after starting, then every `-backup-every` (default `24h`),
keeping the newest `-backup-keep` (default 14).

Keep `data/secret.key` with your backups: without it, stored API keys can't be
decrypted. `make clean` deletes `data/`, which means all your saves.

Heroes from adventures made before rosters existed were copied into the
roster once, portraits included. Those older adventures still play, but they
don't sync with the copies. Pip and Sister Maren were in two old adventures, so
they appear twice; retire the spares.

## Configuration

| Flag | Env | Default | |
|---|---|---|---|
| `-addr` | `TABLETOP_ADDR` | `127.0.0.1:8080` | listen address |
| `-db` | `TABLETOP_DB` | `data/tabletop.db` (`data/demo.db` with `-demo`) | SQLite save file |
| `-agents` | `TABLETOP_AGENTS` | built in | directory of agent definition files (see below) |
| `-model` | `TABLETOP_MODEL` | from the files | override the model of every agent role |
| `-dm-effort` | `TABLETOP_DM_EFFORT` | from the files (`medium`) | override the DM's effort (`low`…`max`) |
| `-companion-effort` | `TABLETOP_COMPANION_EFFORT` | from the files (`low`) | override character and choice-helper effort |
| `-art` | `TABLETOP_ART` | `stock` | illustrations: `stock` (shipped images, free), `paint` (custom, each player's OpenAI key), `off` |
| `-image-model` | `TABLETOP_IMAGE_MODEL` | `gpt-image-2` | OpenAI image model |
| `-image-quality` | `TABLETOP_IMAGE_QUALITY` | `medium` | `low` (cheapest) / `medium` / `high` |
| `-art-workers` | | `2` | images painted in parallel (also for `-paint-stock`) |
| `-stock-list` | | | list the stock images and which are painted, then exit |
| `-paint-stock` | | | paint missing stock images (`all`, or `scenes,monsters,portraits`) with `OPENAI_API_KEY`, then exit |
| `-stock-limit` | | `0` | paint at most this many stock images per run |
| `-art-pack` | `TABLETOP_ART_PACK` | `candlelit` | stock art pack (style) to serve and paint |
| `-stock-dir` | | `web/public/stock` | where art packs are written, one folder per pack |
| `-compact-at` | | `60000` | DM context size (tokens) that ends a chapter; companions at 5/12 of it |
| `-local` | `TABLETOP_LOCAL` | off (`make run`/`debug`/`demo` set it) | single-user mode |
| | `TABLETOP_ADMIN_USER`, `TABLETOP_ADMIN_PASSWORD` | | first admin, created on first start |
| | `TABLETOP_SECRET_KEY` | `data/secret.key` | master key for stored API keys |
| `-demo` | | off | scripted offline table on `data/demo.db` (implies `-local` unless `-local=false`) |
| `-debug` | `TABLETOP_DEBUG` | off | debug logs and the `/api/games/{id}/debug` transcript endpoint |
| `-log-file` | `TABLETOP_LOG_FILE` | | also append logs to this file |
| `-log-format` | `TABLETOP_LOG_FORMAT` | `text` | `text` or `json` |

Agents use prompt caching (frozen system prompts plus append-only transcripts)
and server-side refusal fallbacks. **Chapters keep long adventures affordable:**
when the DM's history passes `-compact-at` tokens (or a companion's passes
5/12 of it), the next quiet moment outside combat ends the chapter. The DM writes a
private "story so far" recap, every agent's history restarts from it (the DM
also keeps its last narration, companions the latest exchanges), and the log
shows "Chapter N ends." Look for `chapter compacted` in the log. The adventure header shows its token total;
Settings shows your spend today. Both weigh tokens the way they are billed:
re-reading the cached story counts a tenth and cache writes 1.25×. Click the
header total for **Tokens by agent**: one row per agent (DM, each character,
the choice helper, chapter recaps) with whose key pays, calls, its current
prompt against the chapter limit, cache hit rate and the raw token counts.

## Levels and class features

Heroes go from level 1 to 5 using the 5e XP table (300 XP for level 2, 900 for
3, 2,700 for 4, 6,500 for 5). A level-up raises maximum HP by the class average (half the hit die
+ 1, plus CON), brings new spell slots and spells, and unlocks class features
from the SRD 5.1. The owner sees a "what's new" pop-up, the story records a
level-up card, and the DM is told to mark the moment.

| Class | Level 1 | Level 2 | Level 3 | Level 5 |
|---|---|---|---|---|
| Fighter | Fighting Style (Defense, +1 AC in armour), Second Wind | Action Surge | Improved Critical (19–20) | Extra Attack |
| Barbarian | Rage (2 uses) | Reckless Attack, Danger Sense | Frenzy; Rage 3 uses | Extra Attack, Fast Movement |
| Rogue | Expertise, Sneak Attack 1d6 | Cunning Action (Hide, Dash) | Fast Hands; Sneak Attack 2d6 | Uncanny Dodge; Sneak Attack 3d6 |
| Ranger | Favored Enemy | Fighting Style (Archery, +2 ranged) | Colossus Slayer | Extra Attack |
| Wizard | Arcane Recovery | Sculpt Spells | level 2 spells | Fireball |
| Cleric | Disciple of Life | Channel Divinity (Turn Undead, Preserve Life) | level 2 spells | Destroy Undead, Mass Healing Word |
| Bard | Bardic Inspiration (d6) | Jack of All Trades, Song of Rest | Cutting Words, Expertise | Font of Inspiration (d8) |

Level 4 brings every class an **Ability Score Improvement**: +2 to one ability
or +1 to two, to at most 20. The hero's owner chooses it in the level-up
pop-up or on the character sheet (in the adventure, or on the character page
while the hero is in town); it waits until they do. A higher CON modifier adds
hit points for every level. At level 5, proficiency rises to +3 and damage
cantrips roll twice the dice.

**Spells** can be cast with a higher slot when their own are spent, and many
get stronger for it (Magic Missile gains a dart, Cure Wounds 1d8...). Shatter
and Fireball are areas that hit everyone near the target, allies included
(the option says how many of each), unless an evoker sculpts the spell.

**In combat, a turn has an action, a bonus action and free features.** The
action picker groups them. Your turn stays open after a bonus action or a free
feature, and after your action while you still have a bonus action left (use
it, or press **End turn**). Agent-played heroes use their features by rule of
thumb (rage at the start, Second Wind when hurt, hide after attacking, inspire
an ally) and only ask their agent for the main action, so this costs no extra
tokens. Short rests restore short-rest features (Second Wind, Action Surge,
Channel Divinity) and trigger Arcane Recovery and Song of Rest. Long rests and
time in town restore everything. Fast Hands, Favored Enemy and Sculpt Spells
only matter in the story; the DM sees them in the party's features.

## Items and loot

Every item is a real object: it has a type from the SRD equipment catalogue,
a rarity, rolled bonuses, a value, a weight and a history of where it's been.
Click anything in a backpack to inspect it.

- **Rarity:** common, uncommon (green), rare (blue), very rare (purple) and
  legendary (orange). Rarer items roll more bonuses: 1 for uncommon, 2 for
  rare, 3 for very rare.
- **Bonuses** come from affix tables per kind, such as *Keen* (+1 to hit),
  *Brutal* (+1 damage), *Sturdy* (+1 AC), *of the Bear* (+1 STR) or *of
  Vigour* (+3 max HP). They make names like *Keen Shortsword of the Bear*.
  They're kept to +1/+2 to suit levels 1–3.
- **Treasure is rolled by the engine,** not invented by the DM: the DM calls
  `roll_loot`, and rarity climbs with party level. Bosses and hidden hoards roll
  twice and keep the better result. At most two items per turn.
- **The DM's `give_item`** hands out ordinary equipment from the catalogue, or
  story keepsakes (a map, a key) that have no stats or value.
- Old inventories convert automatically: known gear becomes catalogue items,
  anything else a keepsake.
- **Equipment** has six slots: main hand, off hand (a shield or a light
  weapon), ranged (bows, crossbows, thrown weapons), armour and two trinkets.
  What's equipped drives your numbers:
  - the main-hand and ranged weapons are your attacks
  - armour and a shield set your AC (unarmoured: 10 + DEX, plus CON for
    barbarians)
  - bonuses apply: a weapon's to hit and damage on its own attacks; AC, max HP
    and abilities from anything equipped
  - Inspect an item and choose **Equip** or **Take off**. In an adventure that
    works outside combat; for heroes in town, on their character page.
  - Classes use their SRD proficiencies. Armour and shields need training. Any
    weapon works, but you only add your proficiency bonus with weapons you're
    trained in. A two-handed weapon frees the off hand.
  - New heroes come equipped with their class's gear. Existing heroes are
    equipped the first time they're loaded, and their numbers stay the same.
- **The market** (**Market** on the home page) is where heroes **in town**
  trade. Heroes on an adventure can't trade: their game holds their sheet.
  Pick which of your heroes is trading, then:
  - **For sale:** other players' listings. Inspect an item, then buy it with
    your hero's gold. The item and the gold move together, or not at all.
  - **Sell:** list an item at your own price (it's held by the market until it
    sells or you withdraw it), or sell it to the town shop for half its value.
    Equipped items and keepsakes can't be sold.
  - **Your listings:** withdraw unsold items into any of your heroes in town,
    or **collect** a sale's proceeds. 10% of every sale between players leaves
    the realm as a fee.
  - **Town shop:** every catalogue item at list price, with quantities for
    things sold by number (arrows, rations, potions).
  - Every trade is written into the item's history.
- **The realm economy** (Settings, admins only). Every coin is recorded in a
  gold ledger:
  - **In:** loot from the DM, new heroes' purses, the shop buying items.
  - **Out:** market fees, the shop selling items, in-story purchases the DM
    charges for.
  - **Transfers:** sales between players move gold without changing the total.
  
  The panel shows the gold in the realm, flows over the last day and week by
  source, and recent sale prices. Three controls steer the money: the **loot
  rate** (scales the DM's per-turn gold cap), the **market fee** and the
  **shop buy-back** rate.

## Safety at the table

Players' words reach the DM and everyone's agents, so the game treats them as
fiction, never as instructions:

- **Player text is marked as data.** Actions, speech and other players'
  backstories reach agents inside `<player_content>` blocks. The DM and
  characters are told that nothing in them can change the rules or their
  tools, and a player can't close the block early.
- **The engine caps what the DM can hand out,** however it's asked: XP per turn
  (100 × party level), gold per turn (50 × party level), item names (60
  characters), what one hero carries (40 items), and trap or healing dice
  outside combat (2 + 2 × level dice, d12 at most). The DM is told when it hits
  a cap and gives less.
- **Lengths are capped:** actions 500 characters, names 40, backstories 400.
- **Typed actions** (the leader's setting under the action box): *Everyone*,
  *Leader only*, or *Quick choices only*. Anyone who can't type picks from the
  quick choices instead. Useful at tables with people you don't know.

## Agents and models

Every agent is one of five roles, each defined by a file in
`internal/adapter/agents/roles/`:

| Role | Does | Pays | Default |
|---|---|---|---|
| `dm` | narrates, voices NPCs, runs the rules tools | leader | Opus 5.5, medium effort |
| `character` | plays one party member (or covers for an away player) | the character's owner | Opus 5.5, low effort |
| `choices` | writes quick choices when the DM forgets | leader | Opus 5.5, low effort |
| `cast` | after each narration, makes sure every NPC in the scene is shown | leader | Opus 5.5, low effort |
| `recap` | the DM's private notes at a chapter break | leader | Opus 5.5, medium effort |

Each file has frontmatter (`model`, `effort`, `tools`) and the prompt as a Go
`text/template`. To experiment without rebuilding, copy the folder, edit it and
start with `-agents path/to/roles`. The startup log lists each role's model and
effort (`agent role`).

**Each player picks their own characters' model** in Settings ("Your
characters' AI"): Opus 5.5, Sonnet 5.5 (about half the cost), Haiku 5.5 (far
cheaper), or the table default. It applies wherever their characters play, on
their own key. Tokens by agent shows the model each seat ran on. Haiku has no
server-side refusal fallback; if it declines, that character hesitates for the
round (or the engine picks its combat move).

## Playing with friends

A friend can play one of your companions in your adventure:

1. In your adventure, outside a fight, press **Offer seat** on an
   agent-played companion's party card. The seat appears on the **Tavern**
   notice board (in the front page's header, with a count of open seats).
2. Your friend opens the Tavern and presses **Take this seat**. From then on
   they play that companion live:
   - its turns wait for them, and "your turn" shows on their front page
   - its agent covers for them on *their* key when they're away
   - the bill for its AI is theirs
3. The character stays yours: its XP, gold and gear come back to your roster
   when the adventure ends. Their party card reads "Played by …".
4. They can **Give seat back** at any time, anywhere; the companion goes back
   to your agent.

Taking a seat needs an Anthropic key in Settings, or a sponsor (below). Each
friend plays one seat per adventure, and the leader can close an offer before
anyone takes it.

### Asking for heroes, and party-wanted posts

Heroes set to **Ask me first** (on the hero's page) can't be recruited
straight away:

- **Asking:** in the recruit dialog, the leader presses **Ask … if … may
  join**. The owner sees the request on their front page and presses **Send …**
  or **Decline**.
- **Party-wanted posts:** the recruit dialog also has **Post on the notice
  board**. The post shows on the Tavern page under *Parties looking for heroes*,
  where any other player can **Offer this hero** from their idle roster. The
  leader sees the offer above the action box and presses **Welcome …** or
  **No thanks**.

Either side can withdraw. Heroes join only at a tavern, town or village, the
same as recruiting, and asks and offers count toward the party's limit of four
while they wait. A hero who joins is played by their agent on their owner's
key, and the owner can take over at any time. The Tavern link's badge counts
open seats plus posts you haven't answered.

### Paying for a friend's AI

An admin can pay for friends who don't have an API key, on the admin's own
Anthropic key:

- **Invite: I'll pay for their AI** in Settings makes an invite whose account
  is sponsored from the start, up to 500,000 tokens a day.
- **Players in the realm** (Settings, admins only) lists every account with
  today's spend. **Pay for their AI** sponsors an existing player; change the
  daily amount, or **Stop paying**.

A sponsored player's own Anthropic key, if they add one, is always used first.
Their spend is capped by the lower of their own cap and what the sponsor pays
for, so raising their own cap doesn't cost the sponsor more. Sponsorship covers
Claude only: narration and paintings still need the player's own OpenAI key.

## Multiplayer server

```sh
export TABLETOP_ADMIN_USER=james TABLETOP_ADMIN_PASSWORD='a long passphrase'
export TABLETOP_SECRET_KEY=$(openssl rand -base64 32)   # or let it create data/secret.key
make serve                                              # = ./bin/tabletop (no -local)
```

1. On first start, the admin account is created from `TABLETOP_ADMIN_*` and
   any older games are assigned to it.
2. Sign in, open **Settings**, add your Anthropic key (and an optional OpenAI
   key), then click **Create invite link** for each friend. Each link works once.
3. Friends register through their link and add **their own** keys. Who pays:
   a character's agent bills its owner; the DM, the choice helper and scene and
   monster paintings bill the adventure's leader; portraits bill the
   character's owner. Everyone sets a daily token cap; when it is reached, that
   player's agents pause until midnight UTC.

Recruiting a friend's character, or handing a friend one of your agent-played
companions to play, arrives with the tavern (see [docs/ROADMAP.md](docs/ROADMAP.md)). Until then, friends play their own adventures.

**Security notes**
- API keys are encrypted with AES-256-GCM and never sent back to the browser.
- Sessions are HttpOnly, SameSite=Lax cookies, marked Secure behind HTTPS.
  Cross-site writes are refused, request bodies are capped at 1 MB, and
  players can only see and act in games they lead or have a character in.
- Run it behind HTTPS (a Caddy config arrives with the hosting step). Never
  expose `-local` or `-demo` to a network: they sign every visitor in.

## Sound and narration

The speaker button at the top of the play screen opens the sound panel. All of
these settings are saved in each player's own browser.

- **Effects and ambience** are off until you switch them on. The browser makes
  them itself (Web Audio), so there are no audio files and nothing to license.
  - **Effects:** dice, hits and misses, crits, coins, loot, level-ups, a fight
    starting, victory and defeat, people arriving and leaving, and a chime when
    it's your move.
  - **Ambience:** follows the scene key (tavern murmur and fire, cave drips,
    forest birds, temple drone...). Drums play during combat.
  - **Cues:** the engine tags its system messages with a `cue` (`domain.Cue`)
    and the UI decides what that sounds like. Only live events make a sound;
    loading the game's history stays quiet.
- **Read the story aloud** reads the DM's narration and, optionally,
  characters' lines. The music quietens while a voice reads. There are two
  voices:
  - **Browser voice:** free, built in, and robotic.
  - **AI voice:** OpenAI `gpt-4o-mini-tts` through `POST /api/speech`, paid
    with each listener's own OpenAI key (the same key used for illustrations;
    in `-local` mode, `OPENAI_API_KEY`). It costs about a cent per minute of
    speech. You pick the narrator's voice; each character keeps one of the
    other voices. If the key is missing or rejected, that line falls back to
    the browser voice and the panel says why.
- **Who says what.** The DM's narration is split up: narrator text uses the
  narrator's voice, and each quoted line goes to whoever says it. The speaker
  is found from a name next to the quote ("Run!" shouts Bryn), an NPC's type
  ("the innkeeper"), or the last person named in the paragraph. Voices follow
  each character's gender, and AI voices are told who is speaking (for
  example "a female dwarf cleric", or an NPC's role and attitude) so they read
  the line in character. You can also pick a voice for any character in the
  panel.
- **Keeping up.** While a voice reads, a bar at the bottom shows who is
  speaking, with **Skip ›** (next line) and **Catch up »** (drop everything
  queued). "Catch up when I act" (on by default) clears the queue whenever you
  take an action.

### Gender

Every hero has a gender (`m` or `f`), picked when they are created; random
heroes roll one and get a name to match. The DM gives every NPC a gender
through `show_npc`. Gender shapes portrait prompts and voices. Heroes made
before this have none yet: their character page asks once, and a hero who is
already on an adventure picks it up there. The stock pack has male and female
portrait variants (`portraits/<race>-<class>-m|f`, `npcs/<type>-m|f`). Until
those are painted (`make stock-paint KINDS=portraits,npcs`), the genderless
image is used.

## Illustrations

**By default the table uses stock art** (`-art stock`): an **art pack** of 129
images painted once in one style and built into the binary. It has scene
backdrops, top-down battle maps, monster portraits, a portrait for each race
and class, and NPC types (innkeeper, guard, priest…). It costs nothing during play. The default pack is `candlelit`
("Candlelit Oils"). Heroes keep any custom portrait they already have, and
anything missing shows the built-in drawings.

```sh
make stock-list                      # what's in the pack, and what's missing
make stock-paint KINDS=scenes LIMIT=2   # paint a sample (OPENAI_API_KEY or your saved key)
make stock-paint                     # paint everything still missing
make web && make build               # bundle the images into the binary
```

Generating packs, making your own style, using your own images and committing a
pack are covered in **[docs/ART_PACKS.md](docs/ART_PACKS.md)**.

### Custom paintings (`-art paint`)

With `-art paint` and an OpenAI key (in Settings, or `OPENAI_API_KEY` in
`-local` mode), custom images are painted in the background and never block a
turn. Stock art shows until they're ready. Each appears as soon as
it's ready; the SVG art is the fallback. The header shows "Painting N
illustrations…" while they work (30–90 s each).

- **Portraits:** painted when you create a hero, from race, class,
  personality and backstory. One per hero, reused in every adventure.
- **Monster portraits:** one per monster type, painted the first time it
  appears and shared by every game.
- **Scene backdrops:** painted when the DM changes scene, from the DM's own
  visual description.
- **Repaint:** *Repaint* on a character sheet, or *Repaint scene* on the backdrop.
  Only shown if you have an OpenAI key.

Opening an adventure re-requests anything missing, so images interrupted by a
restart are retried. Cost scales with new heroes, monster types and places,
not with turns; use `-image-quality low` while testing.

## Debugging

```sh
make debug                      # = ./bin/tabletop -local -debug -log-file data/tabletop.log
tail -f data/tabletop.log | grep -v "msg=http"
```

With `-debug`, the server logs metadata only, never story text:

| Log line | Meaning |
|---|---|
| `turn started` / `finished` / `failed` | each begin, act, fight or autopilot, with account and duration (logged without `-debug` too) |
| `agent call` | one per model call: seat (`dm`, a character ID, or `choices` for the quick-choice helper), `ledger` (`recap` for chapter recaps), `payer`, step, time, tools called, tokens, cache use, `prompt` size, `billed` and the seat's running `seat_billed` |
| `claude call` / `claude call failed` | the API view: model, effort, stop reason, API error |
| `tool call` | each rules-engine tool an agent used, and whether it worked |
| `combat action` / `combat over` | who did what to whom |
| `deadline sweep` | how many adventures agents moved on after a deadline |
| `art queued` / `ready` / `failed` | image generation, with the API error if any |

- **Full transcripts:** `http://127.0.0.1:8080/api/games/<id>/debug` returns
  the whole saved game, including what every agent was sent and replied (only
  with `-debug`). The ID is in the page URL (`/play/<id>`).
- **Browser:** add `?debug` to the page URL once to log every live event to
  the developer console (`?debug=0` turns it off).

## Everyday targets

```sh
make            # list targets
make check      # lint + race tests + 100% use-case coverage (definition of done)
make dev-web    # Vite dev server with hot reload; run `make debug` alongside it
make fmt
make clean      # removes bin/ and data/ (ALL your saves)
```

## Layout

```
cmd/tabletop/               wiring, flags, agent/painter bridges, deadline sweeper loop
internal/domain/            creatures, characters, game, seats/pace, art, accounts, events
internal/domain/rules/      the 5e engine: checks, attacks, spells, combat, death saves
internal/usecase/play       the table: DM tool loop, seats, rounds, autopilot, combat
internal/usecase/newgame    start an adventure from your roster
internal/usecase/characters rosters: create, view, hire policy, retire
internal/usecase/accounts   invites, sign-in, encrypted keys, daily caps
internal/usecase/artwork    background painting queue
internal/usecase/deadlines  sweeps overdue adventures
internal/usecase/games      list / view / delete adventures
internal/adapter/agents     agent definition files (roles/*.md) and their loader
internal/adapter/claude     Messages API agent (streaming, caching, fallbacks, per-key clients)
internal/adapter/openai     image generation
internal/adapter/sqlite     all persistence, character write-back, migrations
internal/adapter/srd        classes, races, monsters, personas, DM styles
internal/adapter/demo       scripted offline agent
internal/delivery/web       JSON API, SSE, auth
web/                        React UI (built into web/dist)
tools/                      lint rules, screenshot helper
```

## Current limits

- Levels 1–3 and an SRD subset. Class features (Sneak Attack, Rage, Second
  Wind) and monster multiattack aren't modelled yet.
- No shared parties yet (the tavern is next), no item ledger or trading, no human DM seat.
- To run a realm for other people, see **[docs/DEPLOY.md](docs/DEPLOY.md)**:
  Docker, Caddy for HTTPS, backups, and sign-in limits.
