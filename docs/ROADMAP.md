# Roadmap

## The vision

A persistent, multiplayer 5e fantasy realm. Every character is an agent that lives in
the realm. Its owner plays it live or lets its agent play it in their downtime.
Players recruit each other's characters, and take over each other's agent-played
seats, in the tavern. Each player's agents run
on **their own** API tokens; the platform only provides the ecosystem. DMs run
long stories as AI agents, with room for human DMs later. Items are unique and
tradeable through shops.

Decisions so far:
- **Audience:** a hosted, invite-only server.
- **Time model:** an asynchronous realm (play-by-post with deadlines and autopilot).
- **Item ownership:** a server-side ledger, not a blockchain.

## Status

| | Step | State |
|---|---|---|
| ✅ | Single-player game: DM and companion agents, 5e rules engine, combat, UI, dice | done |
| ✅ | Illustrations (OpenAI, background queue, per-character and per-monster) | done |
| ✅ | Debug logging, transcript endpoint, quick choices (with fallback) | done |
| ✅ | **1. Accounts + bring-your-own key:** invites, sessions, encrypted keys, payer routing, daily caps, `-local` | done |
| ✅ | **2. Persistent characters:** roster, write-back on every save, rest in town, chronicle, legacy import | done |
| ✅ | **3. Async seats:** live/agent seats, rounds wait for every live player, pace, deadlines, sweeper, autopilot, "your turn" | done |
| 🔨 | **4. Realm, tavern and chronicle:** slices 1–3 done (recruiting at a tavern; friends taking open seats; asking for heroes; party-wanted posts and offers; a front-page inbox; the realm paying for friends' AI). Next: recall and swaps, adoption, then the realm and its chronicle | in progress |
| ✅ | **5. Hosting:** Dockerfile, Caddy (auto-HTTPS), nightly backups, security pass (sign-in limits, headers), deploy guide ([DEPLOY.md](DEPLOY.md)); choosing a provider and a domain is next | done |
| 🔨 | **Items, market and exchange** (see below): slices 1–4 done (items, gear, market, gold ledger); next: direct trades, then a paid currency | in progress |
| ✅ | **Sound:** synthesised effects and per-scene ambience, narration with the browser voice or AI voices on the player's OpenAI key | done |
| ⏳ | Player-made art packs (see below) | |
| ✅ | **Class features (levels 1–3):** SRD features for every class, a bonus-action turn economy, rests that recharge, and level-up screens | done |
| ✅ | **Levels 4–5:** ability score improvements (chosen by the owner), Extra Attack, Uncanny Dodge, Fireball and other 3rd-level spells, upcasting, area spells | done |
| 🔨 | **Gameplay polish** (see below): quest journal, rest buttons, potions in fights, fleeing undead, Multiattack, opportunity attacks and Disengage done. Next: bonus-action spells, encounter variety, a player-facing recap | in progress |
| 🔨 | **New-player experience:** a one-click first adventure (tutorial DM), tips and plain-English rules terms | in review |
| ⏳ | Next phase: direct trades, travel between realms, human DM seat, email notifications | |

## Design rules that must keep holding

- **The engine owns the numbers.** Agents call tools (checks, saves, damage,
  `start_combat`…); they never invent results. All randomness goes through the
  injected `rules.Roller`.
- **Difficulty ramps with level.** `start_combat` refuses fights above the 5e
  "hard" budget (`rules.HardBudget` vs `rules.AdjustedXP`) and any monster above
  `rules.MaxCR` (CR 1/2 at level 1), whatever the DM names it.
- **Everyone fights in character.** Companion agents get their class's
  `ClassTemplate.Tactics` and options labelled melee/ranged/moves. Monsters
  follow `Creature.Tactic` (brute, skirmisher, pack, cunning, mindless) in
  `rules.MonsterChoice`, and dim ones (low INT) sometimes act at random.
- **Who pays.**
  - A character's agent bills whoever holds its seat (`Game.Payer(c)`): its player, else its owner. After step 4 a friend who takes over a seat takes over its compute too, including the agent that covers for them while they are away.
  - The DM, the quick-choice helper, and scene and monster art bill the leader (`Game.LeaderID`).
  - Portraits bill the character's owner.
- **Agents are defined by role files** (`internal/adapter/agents/roles/*.md`: model, effort, tools, prompt template).
  - Players choose the model for their own characters (`Account.AgentModel`), applied in `cmd/tabletop/agent.go`.
  - `SeatUsage.Model` records what answered.
  - Only the Anthropic provider is wired. Other providers are a later step: `AgentModel.Provider` is the hook. A seat's transcript replays the provider's raw turns, so changing provider mid-adventure needs a fresh history for that seat.
  - Keys are resolved per call in `cmd/tabletop/agent.go` through `accounts.Credential`, which enforces the daily cap.
  - `newgame` refuses to start an adventure until the leader has registered an Anthropic key (`newgame.Agents`).
  - `Game.Spend` ledgers every call by seat (`dm`, character IDs, `choices`, `recap`) with its payer; it sums to `Game.Usage`. The UI's "Tokens by agent" panel reads it.
- **Transcripts are append-only** (prompt caching and preserved thinking).
  - Assistant turns replay byte-for-byte from `AgentMessage.Raw`. Consecutive user turns are merged by the Claude adapter.
  - Never edit or delete earlier messages. The exceptions: the throwaway `choices` seat, which starts fresh each time, and **chapter breaks** (`usecase/play/chapters.go`), which replace every history with a single recap message. That form is valid under preserved thinking because nothing earlier is replayed. `Game.Context` tracks each seat's prompt size; `Game.Recap` is the latest story so far, which is useful for the step 4 chronicle.
- **Characters are the source of truth.**
  - `sqlite.Store.Save` writes party sheets back to `characters` in the same transaction (`domain.Character.Settle`).
  - Party creature IDs equal character IDs when `Game.Roster` is true. Legacy (non-roster) games keep `pc1`-style IDs and don't sync.
- **Owner vs player.** `OwnerID` is whose roster a character lives on; playing it never changes that. From step 4, `PlayerID` is who holds its seat in this adventure (empty = the owner). Ownership only moves through an explicit adoption at the tavern.
- **Seats.** `Creature.Controller` decides who plays: `human` = live, `ai` = agent.
  - `Game.Pending` holds this round's live actions. `Game.Deadline` and `Game.Pace` drive autopilot.
  - `play.Expire` (called by the `deadlines` sweeper) lets agents act for absent players. Those seats go into `Game.Covering` (shown as "agent covering") until the owner acts again.
  - `play.Cover` lets an owner refuse that: a seat in `Game.Hold` holds back when its owner is away, and no agent is called for it.
  - One live character per player. `Autopilot(off)` switches seats, and a fallen live character passes its player to a spare (`reseat`).
  - **No leader, no story.** `Game.Playing()` is true only while the leader (whose key runs the DM) plays one of their own living characters; a covered seat counts. Otherwise `Act`, `proceed` and `runCombat` pause and `schedule` sets no deadline, so agents never play on alone and nobody uses someone else's DM.
  - **One agent per character per party.** Transcripts are keyed by character within a game. `Game.Leave` deletes them, so a hero recruited elsewhere starts a fresh agent.
  - **The fallen retire.** `retireFallen` (in `schedule`, outside combat) takes dead members out of the party while someone lives. The store settles them as fallen, with "(fell in battle)" in their chronicle.
- **Equipment drives the numbers.** `Creature.Recalculate` derives abilities, max HP, AC and weapon attacks from `BaseAbilities`, `BaseMaxHP`, the class and equipped items (`Item.Slot`). It's safe to call repeatedly; never edit those numbers directly. `Outfit` runs once per hero, adding class weapons as items and equipping their gear. Level-ups set the base and recalculate.
- **Trades are atomic and only in town.** `usecase/market` does all its moves through `Store.Atomically` (a SQLite transaction via `cmd/tabletop`'s `marketStore`), and only for idle heroes, because an adventuring hero's sheet is written back from their game. Listings hold the item in escrow; proceeds wait on the listing until collected.
- **Every coin is ledgered.** Gold changes go through `domain.LedgerEntry` (append-only `gold_ledger`). Faucets: loot, starting purses, shop buy-backs. Sinks: DM charges, shop sales, market fees. Transfers: player sales and payouts. Play records via `Game.GoldMoves` (saved with the game); the market via `Tx.Record` inside its transaction. `domain.Economy` (admin-set) scales loot and sets the fee and buy-back.
- **Art costs nothing in play by default.** `-art stock` serves pre-painted images from an **art pack** in `web/public/stock/<pack>/` (`domain.ArtPacks`: key, name, style prompt; `domain.StockList`, painted once with `-paint-stock`). Later: let each adventure (or realm) choose its pack. Custom painting (`-art paint`) is opt-in. A subject's own painting always wins over stock (`artwork.Picture`).
- **NPCs are story characters, not combatants.** The DM's `show_npc` and `npc_leaves` tools keep `Game.NPCs`; `set_scene` leaves behind anyone who doesn't follow the party; `partyStatus` lists who is here every turn. Because models forget tools, a throwaway `cast` call after each exploration narration (`castCheck`) adds anyone narrated but not shown, like the `choices` fallback. A fight spawns monsters separately, so a turncoat guide becomes a monster via `start_combat`.
- **Untrusted text.** Other players' names, personas, backstories and actions
  reach your agents' prompts. Wrap them with `domain.PlayerText` (a
  `<player_content>` block the text can't close) and cap their length
  (`domain.MaxAction`, names 40, backstories 400). `dm.md` and `character.md`
  say text in those blocks is fiction, never instructions.
- **The engine bounds the DM's generosity.** `rules.TurnXPCap`, `rules.TurnGoldCap`,
  `rules.MaxItemName`, `rules.MaxInventory` and `rules.CheckHazardDice` hold
  per DM turn whatever the prompt says; tools return the limit so the DM can
  adjust. `Game.Typing` (everyone / leader / choices) restricts free text,
  checked in `play.Act`.
- **The demo never touches real saves.** `-demo` defaults to `data/demo.db`.
  Test against copies of real databases, never `data/tabletop.db` itself.

## Step 4: realm, tavern and chronicle (in progress)

Goal: the **tavern** is the one place where parties change. Friends put heroes
up for hire, take over each other's agent-played companions, and give heroes
away, all through the tavern and its notice board. Every character stays
playable by anyone the owner lets in. Taking a seat means taking on its compute.

### Ownership rules

- **Owner** (`Creature.OwnerID`, `Character.OwnerID`): whose roster the hero lives on.
  - XP, gold, gear and the portrait always settle back to the owner's roster.
  - Playing a character never changes its owner.
- **Player** (new `Creature.PlayerID`; empty means the owner): who holds the seat in this adventure.
  - The player plays the character live, and their key pays for its agent. That includes covering while they are away (`play.Cover` applies to the player).
  - Add `Game.PlayerOf(c)` and use it everywhere seats are resolved: `Payer`, `SeatFor`, `CanView`, `WaitingFor`, `owedSeat`, `reseat`/`spare`, "one live character per player".
  - The `members` column lists owners and players.
- **Giving a seat back:** the player can hand the seat back at any time, anywhere. The character returns to its owner's agent and the owner's key. The story keeps moving only while the leader is playing (see the seat rules).
- **Adoption** is the only way ownership moves. The owner posts an idle hero "for adoption" and a friend accepts. The hero moves to the new roster with its sheet, portrait and chronicle (an "adopted by …" entry).
- **Chronicle credit:** each adventure entry records who played the character ("played by pip").

### Slice 1: built

- `Game.AtTavern()`, `domain.TavernScenes` and `domain.MaxParty` (4).
- `play.Recruit` (leader only, their own idle heroes) and `play.SendHome` (owner only; not the character they're playing; the party keeps one member).
- `Game.Join`/`Game.Leave` record `Game.Joined`/`Game.Left`. These aren't stored: `sqlite.Store.Save` embarks or settles those characters in the same transaction as the game.
- `Game.TableNotes` tell the DM about arrivals and departures at the top of its next message, with no extra model call.
- The UI: **+ Recruit from the tavern** on the scene image (in a tavern, town or village), with a picker of heroes in town, and **Send home** on party cards.
- **The tavern listing:** `GET /api/tavern` (`characters.ForHire`) lists other players' heroes in town whose hire policy is `ask` or `auto`, with owner names.
  - `play.Recruit` takes the leader's own heroes and other players' `auto` heroes, which run on their owner's key. `ask` heroes wait for invites (slice 2).
  - Only idle heroes can join; one in another party is refused.
  - The recruit dialog shows the full sheet and can roll a new hero; the character page sets the hire policy.
  - NPCs are deliberately not recruitable.
- **Older scenes:** `Game.CastAt` tracks the last narration checked for NPCs, and the browser asks `POST /cast` once when `castStale` is set.

### Slice 2: built (open seats)

- `Creature.PlayerID` and `domain.PlayerOf(c)`: a friend can hold a seat. Turns, "your turn", `SeatFor`, `CanView`, `WaitingFor`, `Payer`, covering and `members` follow the player; the sheet still settles to the owner.
- `Game.OpenSeats`: `play.OfferSeat` (leader only, an agent-played living companion, outside combat), `play.TakeSeat` (open, not your own party, one seat per friend) and `play.GiveSeatBack` (any time). Each tells the table and the DM.
  - Offering a seat doesn't need a tavern: the party doesn't change, only who plays a companion.
- `usecase/tavern`: `Board` lists other people's open seats (from the `open_seats` column); `Take` needs a ready AI key, since whoever plays a seat pays for it.
- UI: **Offer seat** / **Give seat back** on party cards, "Played by …", a **Tavern** page with the board and heroes for hire, and a count of open seats in the header.

### Slice 3: built (asks, party-wanted posts, sponsors)

- `Game.Calls []domain.Call{Kind ask|offer, CharacterID, OwnerID, Name, Summary}`. Calls are stored on the game, so play owns every party change.
  - `play.AskForHero` (leader, an `ask` hero), `play.OfferHero` (an owner answering a post) and `play.AnswerCall` (the owner answers an ask, the leader an offer, and either side withdraws). Yes goes through the same `welcome` as `Recruit`, at a tavern.
  - Calls count toward the party limit while they wait.
- `Game.Wanted` (`play.PostWanted`, at most 200 characters, shown to players only, never to agents).
- An `on_board` column replaces the open-seats filter: `GamesOnTheBoard`.
- `tavern.Notices` returns posts (from other leaders), inbox (calls you answer) and waiting (calls you made). `tavern.Offer` needs a ready key.
- **Realm pays:** `Account.SponsoredBy/SponsorCap`, sponsored invites, and `accounts.Players/Sponsor`. `accounts.Credential` falls back to the sponsor's Anthropic key within `min(own cap, sponsor cap)`.
- UI: asking and posting in the recruit dialog, offers and asks above the action box, a front-page inbox, *Parties looking for heroes* on the Tavern page, and a players list in Settings.

### The tavern rule

The party can only change while it **can reach a tavern**, meaning it is in a tavern, town or village:
- `Game.AtTavern()` is true when the game has not started, or when the mode is exploration and `Scene.Key` is one of `domain.TavernScenes` (`tavern`, `town`, `village`).
- Every adventure opens in a tavern, so the leader can recruit before setting out and whenever the DM brings the party back to a settlement.
- The DM's system prompt says new members can only join in a tavern, town or village, so if players ask, the story heads for one. A newcomer is introduced as arriving from the local tavern.
- Recruiting, swapping and recall need `AtTavern()`. Giving a seat back and taking control of your own characters don't.

### Built so far, against the original plan

The plan was a separate `tavern_posts` table. What shipped keeps every
party change in `play`, with the state stored on the game:

| Planned | Built as |
|---|---|
| `seat` posts, TakeSeat | `Game.OpenSeats`, `play.OfferSeat/TakeSeat/GiveSeatBack` (slice 2) |
| `hire` posts, Invite/Respond | `Character.Hire` (`auto` recruits at once, `ask` asks first) and `Game.Calls` of kind `ask` (slice 3) |
| `wanted` posts | `Game.Wanted` and calls of kind `offer` (slice 3) |
| Board, incoming invites | `tavern.Board` (seats) and `tavern.Notices` (posts, inbox, waiting) |
| Light notifications | the front-page inbox and the Tavern badge |
| A key needed to take a seat | still true, unless an admin sponsors you (`Account.SponsoredBy`) |
| Capping and wrapping post text | posts are capped at 200 characters and never shown to agents |

### To build

1. **Recall and swaps** (in `play`, at a tavern):
   - **Recall(owner, gameID, characterID):** the owner takes their hero out of someone else's party. It settles home, and any friend playing it loses the seat. Show "Recall" on your heroes in other parties.
   - **Swap(leader, gameID, outID, inID):** replace an agent-played companion with a recruit, ask or offer in one step. The outgoing character settles home.
2. **Adoption:** the owner marks an idle hero "for adoption" on the character page, it is listed on the Tavern page, and a friend accepts. Ownership, sheet, portrait and chronicle move to the new roster, with an "adopted by …" entry. This is the only way ownership changes.
3. **Party cards:** show both names when they differ ("Played by pip · Aria's owner: james").
4. **Realm.**
   - `domain.Realm{ID, Name, DMStyle, Chronicle []Entry, OwnerID}`, a `realms` table, and a default realm per server, created on first start.
   - Adventures belong to a realm (`Game.RealmID`).
   - The DM's system prompt includes the realm chronicle (the last N entries, summarised, wrapped as data). Keep it stable within a game so the cache holds.
5. **Chronicle.**
   - A DM tool `record_chronicle({summary})`, prompted when an adventure ends (`end_adventure`, or the leader closing it).
   - It appends to the realm chronicle and to each character's history, with player credit ("played by pip").
   - The realm chronicle on the home page.
6. **Notifications, later:** email or push when a call waits on you or it's your turn.

### Tests to add

- Recall: allowed only to the owner and only at a tavern; the seat's player is unseated, and the sheet settles home.
- Swap: one step, the outgoing hero settles home, and the DM hears about both.
- Adoption moves the character, portrait and chronicle to the new owner.
- The chronicle carries into the next adventure's DM prompt, with player credit.

## Gameplay polish

**Built:**
- **Journal:** a DM tool `update_quest` (open, done or failed, with a note). It is shown under the party panel and listed in each `[State]`. At most 10 quests; the oldest finished one is dropped when the journal is full.
- **Rest buttons:** short and long rests are sent as the player's action, and the DM still rules on them.
- **Potions in fights:** drink one, or give one to a hurt or dying ally within reach (an action).
- **Turned undead flee:** they Dash away from the party instead of skipping their turns.
- **Multiattack:** the owlbear and bandit captain make two attacks.
- **Opportunity attacks and Disengage:** leaving a foe's reach provokes its reaction attack. Movement is resolved start to end, not square by square.

**Next:**
- **Bonus-action spells:** Healing Word and Mass Healing Word should cost a bonus action (they're actions today). Add others, such as Misty Step and Spiritual Weapon, as the spell lists grow. Also Cunning Action: Disengage for rogues.
- **Encounter variety:** more SRD monsters, monster traits (Pack Tactics, Nimble Escape as bonus actions), mixed groups, and simple terrain on the battle map.
- **A player-facing recap:** "the story so far" for returning players. The DM's chapter recap is private notes and may hold secrets, so this needs its own public summary, written from the log and the journal.

## Items, the market and the exchange

Goal: items are a core part of the game. Loot is random enough that every drop
is worth inspecting, good finds are rare, and players trade freely in a
market whose money supply the realm owner controls. Our own take on random
loot (rarity tiers and rolled affixes), kept small enough for 5e's bounded
accuracy at levels 1-3.

Decisions: real money only ever flows **in** (buying currency or cosmetics,
model A or C); no cashing out. Gold is earned in play; the gold ledger is
built so a paid currency can plug in later.

**Slice 1: real items** (built)
- `domain.Item`: an ID, base type, name, kind, rarity, affixes, value, weight,
  quantity and history. `Creature.Items` replaces the free-text `Inventory`;
  the store converts old inventories on load.
- An SRD equipment catalogue (`srd` adapter): weapons, armour, shields, potions,
  gear and packs, with damage, AC, properties, value, weight and description.
- Loot is rolled by the engine (`rules.RollItem`): rarity by party level
  (common, uncommon, rare, very rare; legendary is reserved for set pieces),
  then 0-3 affixes from tables per kind, a name like "Keen Longsword of the
  Bear", and a value.
- The DM's tools: `roll_loot` (the engine rolls a random item, capped per
  turn) and `give_item` (catalogue items, or story keepsakes with no stats).
- Inspect any item: stats, affixes, value, weight and history.

**Slice 2: gear that matters** (built)
- Equipment slots (main hand, off hand, armour, two trinkets).
- The equipped weapon drives attacks, armour and shield drive AC, and affixes
  apply (to hit, damage, AC, max HP, abilities). Class templates only provide
  proficiencies.

**Slice 3: the market** (built: listings, buying, withdrawing, collecting and the town shop; direct item-for-item trades are still to do)
- Fixed-price listings (item held in escrow), direct trades (items and/or
  gold, both accept), and town shops that buy and sell catalogue items.
- Only in town. Sale fees take gold out of the realm.
- Every move of items and gold is atomic and logged in the item's history.

**Slice 4: the gold ledger and the exchange** (built: the ledger, the report and the controls; a paid currency is still to come)
- Every coin entering the realm (loot, rewards) or leaving it (fees, shops) is
  a ledger entry, so the money supply is always known.
- Admin controls for loot rates and fees, and dashboards for supply, flows and
  prices.
- Later: a paid currency or cosmetics through a payment provider (money in only).

## Player-made art packs

Goal: players design an art style and generate a pack for it on their own
OpenAI key, then use it at their table and share it with the server. How
packs work today is in [ART_PACKS.md](ART_PACKS.md).

**Already in place**
- Packs are folders of stock images, with the style as one prompt (`domain.ArtPacks`).
- `usecase/stockart` lists a pack's images and paints only the missing ones, in parallel. It is resumable and counts failures.
- The fallback order: custom painting, then the pack, then built-in drawings (`artwork.Picture`).

**To build**
1. **Packs as data, not code.**
   - Each pack folder gets a `pack.json` (key, name, author, style prompt, created).
   - Packs live under `data/packs/` as well as the built-in `web/public/stock/`, and `domain.ArtPacks` becomes the built-in default only.
   - A `packs` store lists both.
2. **A pack per adventure.** `Game.ArtPack` is chosen at party setup, defaulting to the server's `-art-pack`. Later, a realm's pack.
3. **A pack studio page.**
   - Name the style and write (or pick a starting) style prompt.
   - Paint a preview set (a bright scene, a dark scene, a battle map, a monster, a portrait) on the player's own OpenAI key, with a cost estimate first.
   - Tweak and repaint the preview, then "Paint the whole pack" as a background job with progress (it reuses `stockart` with the player as payer).
   - Repaint or upload single images, with a size check.
4. **Sharing.** A pack can be private, or shared with the server. Shared packs appear in everyone's setup picker with their author's name, and the server admin can hide one.
5. **Safety and limits.**
   - Style prompts are untrusted text: cap their length, and keep the subject half of every prompt fixed.
   - Respect the daily caps.
   - Moderation failures show per image with a retry.
   - Uploaded images are re-encoded to WebP and size-checked.

**Tests to add**
- The pack store: built-in and data packs, a bad `pack.json`, a missing pack falling back to the default.
- Painting on the player's key: billing, caps, preview then full run, resume after restart.
- Per-adventure packs: the game view serves the adventure's pack.

## Step 5: hosting (built)

See [DEPLOY.md](DEPLOY.md). What remains is choosing a provider and a domain, then inviting a friend to play one async round.

- A multi-stage `Dockerfile` (node build, then go build, then a distroless image) and a `docker-compose.yml` with Caddy (auto-HTTPS).
- Nightly SQLite `.backup` plus `secret.key`.
- Security pass: per-account and login rate limits, security headers, and a log scrub check.
- A deploy guide in Quickstart. Verify by inviting a friend and playing one async round.
