// Types mirror the Go domain's JSON. Keep in sync with internal/domain.

export type Ability = "STR" | "DEX" | "CON" | "INT" | "WIS" | "CHA";

export interface Dice {
  count: number;
  sides: number;
  bonus: number;
}

export interface Action {
  id: string;
  name: string;
  kind: "attack" | "save" | "auto_hit" | "heal" | "dodge";
  toHit?: number;
  saveDc?: number;
  saveAbility?: Ability;
  dice: Dice;
  damageType?: string;
  rangeFt: number;
  slotLevel?: number;
  description?: string;
}

export interface Persona {
  archetype: string;
  personality: string;
  backstory: string;
  voice: string;
  bravery: number;
  chattiness: number;
  loyalty: number;
}

/** A class feature, with uses (max 0 means always on). */
export interface Feature {
  key: string;
  name: string;
  description: string;
  level: number;
  max?: number;
  used?: number;
  recharge?: "short" | "long";
}

export interface Creature {
  /** A friend playing this character's seat (empty: its owner plays it). */
  playerId?: string;
  features?: Feature[];
  /** A Bardic Inspiration die waiting to be used. */
  inspiration?: number;
  /** Ability Score Improvements earned but not yet chosen. */
  asi?: number;
  /** Ability scores before equipment bonuses. */
  baseAbilities?: Partial<Record<Ability, number>>;
  id: string;
  name: string;
  kind: "pc" | "monster";
  controller: "human" | "ai" | "dm";
  race?: string;
  gender?: Gender;
  class?: string;
  level: number;
  xp: number;
  abilities: Partial<Record<Ability, number>>;
  maxHp: number;
  hp: number;
  ac: number;
  speed: number;
  skillProfs?: string[];
  actions: Action[];
  spellSlots?: Record<string, number>;
  slotsUsed?: Record<string, number>;
  conditions?: string[];
  deathSaves: { successes: number; failures: number };
  stable?: boolean;
  dead?: boolean;
  pos: { x: number; y: number };
  persona?: Persona;
  cr?: string;
  portrait?: string;
  /** Old free-text inventory (converted to items by the server). */
  inventory?: string[];
  items?: Item[];
  gold: number;
}

export type Rarity = "common" | "uncommon" | "rare" | "very rare" | "legendary";

export interface Affix {
  key: string;
  name: string;
  prefix?: boolean;
  stat: string;
  value: number;
}

/** One specific, owned item. */
export interface Item {
  id: string;
  base?: string;
  name: string;
  kind: "weapon" | "armour" | "shield" | "trinket" | "potion" | "gear" | "pack" | "keepsake";
  rarity: Rarity;
  affixes?: Affix[];
  value: number;
  weight: number;
  qty?: number;
  desc?: string;
  damage?: string;
  ac?: number;
  props?: string[];
  history?: string[];
  /** Where it's equipped ("" or absent: carried in the pack). */
  slot?: Slot;
}

export type Slot = "main" | "off" | "ranged" | "armour" | "trinket1" | "trinket2";

export interface Roll {
  label: string;
  dice: string;
  rolls: number[];
  kept: number;
  modifier: number;
  total: number;
  target?: number;
  success?: boolean;
  advantage?: string;
  critical?: boolean;
}

export interface Scene {
  key: string;
  title: string;
  mood: string;
}

export type EventType =
  | "narration_delta"
  | "narration"
  | "speech"
  | "player"
  | "roll"
  | "scene"
  | "system"
  | "state"
  | "awaiting"
  | "thinking"
  | "error"
  | "image"
  | "level_up";

export interface GameEvent {
  type: EventType;
  actor?: string;
  text?: string;
  roll?: Roll;
  scene?: Scene;
  stream?: string;
  /** A sound effect for the moment (system messages). */
  cue?: string;
  /** What a hero gained on levelling up (level_up events). */
  levelUp?: { level: number; gains: string[] };
}

export interface Encounter {
  round: number;
  order: string[];
  initiative: Record<string, number>;
  turn: number;
  gridW: number;
  gridH: number;
}

export interface Option {
  actionId: string;
  actionName: string;
  /** What it uses: the action (empty), the bonus action, or nothing. */
  cost?: "" | "bonus" | "free";
  targetId?: string;
  targetName?: string;
  summary: string;
}

export interface Game {
  id: string;
  title: string;
  dmStyle: string;
  mode: "exploration" | "combat" | "over";
  scene: Scene;
  creatures: Creature[];
  party: string[];
  encounter?: Encounter;
  log: GameEvent[] | null;
  usage: Usage;
  /** Token spend per agent seat: "dm", a character ID, "choices" or "recap". */
  spend?: Record<string, SeatUsage>;
  /** Live characters whose agent is standing in for an away owner. */
  covering?: Record<string, boolean>;
  /** Live characters whose owner has not let their agent cover for them. */
  hold?: Record<string, boolean>;
  leaderId: string;
  /** A new player's guided first adventure. */
  tutorial?: boolean;
  /** Companions the leader has offered to friends on the tavern board. */
  openSeats?: string[];
  /** The leader's post on the tavern's notice board. */
  wanted?: string;
  calls?: Call[];
  /** The party's journal, kept by the DM. */
  quests?: Quest[];
  /** Friends playing seats here, by character ID. */
  players?: Record<string, string>;
  /** The viewer's account ID. */
  you: string;
  /** The party can take on or send home members here (a tavern, town or village). */
  atTavern: boolean;
  partyLimit: number;
  /** Who may type free-text actions: everyone (absent), the leader, or nobody. */
  typing?: "leader" | "choices";
  /** False when the viewer plays from the quick choices only. */
  mayType: boolean;
  /** The longest action the viewer may type. */
  maxAction: number;
  /** The latest narration hasn't been checked for NPCs yet. */
  castStale: boolean;
  /** Prompt size (tokens) at which each kind of agent closes a chapter. */
  chapterAt: { dm: number; companion: number };
  /** Each seat's latest prompt size in tokens. */
  context?: Record<string, number>;
  chapter?: number;
  awaiting: boolean;
  started: boolean;
  options: Option[] | null;
  /** A turn is being resolved on the server right now. */
  busy: boolean;
  /** People and creatures the DM has put on the table. */
  npcs?: NPC[];
  /** The DM's suggested next actions. */
  choices?: string[];
  /** When absent players' agents step in (ISO time). */
  deadline?: string;
  pace?: Pace;
  /** This round's submitted actions, by character ID. */
  pending?: Record<string, string>;
  /** Events trimmed from the front of the log (for stable keys). */
  logBase?: number;
  /** The viewer's characters in this party. */
  mine: string[];
  yourTurn: boolean;
  waitingOn: string[];
  /** Every character is left to its agent, so the story waits for a player. */
  paused: boolean;
  /** Finished illustration URLs by creature ID, plus "scene". */
  art: Record<string, string>;
  /** Creature IDs (and "scene") whose art is being painted. */
  painting: string[];
  artEnabled: boolean;
}

export interface NPC {
  id: string;
  name: string;
  /** A person type, or a monster key when monster is set. */
  type: string;
  monster?: boolean;
  role: string;
  attitude: "friendly" | "neutral" | "wary" | "hostile";
  gender?: Gender;
  look?: string;
  follows?: boolean;
  /** In the current scene. */
  here: boolean;
}

export interface Usage {
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
}

export interface SeatUsage {
  payer: string;
  /** The model the seat's latest call ran on. */
  model?: string;
  calls: number;
  total: Usage;
  last: Usage;
}

/** Tokens weighted the way they are billed, in input-token terms (matches the daily cap). */
export function billed(u: Usage): number {
  return u.input + u.output + Math.floor((u.cacheWrite * 5) / 4) + Math.floor(u.cacheRead / 10);
}

export interface Pace {
  key: string;
  name: string;
  /** Nanoseconds (Go time.Duration). */
  explore: number;
  combat: number;
}

export interface Summary {
  yourTurn: boolean;
  deadline?: string;
  id: string;
  title: string;
  party: string;
  mode: string;
  scene: string;
  sceneKey: string;
  updatedAt: string;
}

/** An adventure in the list, with its current scene's painting. */
export interface SummaryView extends Summary {
  art?: string;
}

export interface ClassTemplate {
  name: string;
  summary: string;
  hitDie: number;
  priority: Ability[];
}

export interface RaceTemplate {
  name: string;
  summary: string;
  speed: number;
}

export interface DMStyle {
  key: string;
  name: string;
  description: string;
}

export interface SetupOptions {
  classes: ClassTemplate[];
  races: RaceTemplate[];
  archetypes: Persona[];
  dmStyles: DMStyle[];
  paces: Pace[];
}

export interface NewGameInput {
  title: string;
  dmStyle: string;
  pace: string;
  heroId: string;
  companionIds: string[];
  /** A new player's guided first adventure. */
  tutorial?: boolean;
}

export interface Character {
  id: string;
  ownerId: string;
  sheet: Creature;
  status: "idle" | "adventuring" | "fallen";
  adventureId?: string;
  hire: "no" | "ask" | "auto";
  history?: string[];
  portrait?: string;
  painting: boolean;
  /** XP needed for each level (index 0 = level 1), up to the max level. */
  xpLevels: number[];
}

export interface Listing {
  id: string;
  sellerId: string;
  sellerCharacter: string;
  item: Item;
  price: number;
  status: "open" | "sold" | "withdrawn";
  createdAt: string;
  proceeds?: number;
  collected?: boolean;
  sellerName: string;
  mine: boolean;
}

export interface ShopItem {
  key: string;
  name: string;
  kind: Item["kind"];
  price: number;
  desc: string;
  damage?: string;
  ac?: number;
  props?: string[];
  weight: number;
  stack?: boolean;
}

export interface Economy {
  lootPercent: number;
  marketFeePercent: number;
  shopBuyBackPercent: number;
}

export interface GoldFlows {
  in: number;
  out: number;
  net: number;
  by: Record<string, number>;
}

export interface EconomyReport {
  economy: Economy;
  supply: number;
  heroes: number;
  day: GoldFlows;
  week: GoldFlows;
  sales: { item: string; rarity: Rarity; price: number; at: string }[];
}

/** Another player's hero looking for adventure. */
export interface TavernHero extends Character {
  ownerName: string;
}

export type Gender = "m" | "f";

export interface NewCharacter {
  name: string;
  class: string;
  race: string;
  gender: Gender | "";
  archetype: string;
  backstory: string;
}

export interface Account {
  id: string;
  username: string;
  admin: boolean;
  dailyTokenCap: number;
  /** Model the player's characters' agents run on; absent = the table's default. */
  agentModel?: string;
  /** Set when an admin pays for this player's AI. */
  sponsoredBy?: string;
  sponsorCap?: number;
}

export interface AgentRole {
  name: string;
  description: string;
  model: string;
  effort: string;
  tools?: string[];
}

export interface AgentModel {
  id: string;
  name: string;
  provider: string;
  note: string;
}

/** A short display name for a model ID ("claude-haiku-5-5" -> "Haiku 5.5"). */
export function modelName(id: string): string {
  const m = /^claude-([a-z]+)-(\d+)(?:-(\d+))?/.exec(id);
  if (!m) return id;
  return `${m[1][0].toUpperCase()}${m[1].slice(1)} ${m[2]}${m[3] ? `.${m[3]}` : ""}`;
}

export interface KeyInfo {
  kind: "anthropic" | "openai";
  set: boolean;
  last4?: string;
}

export interface Me {
  account: Account;
  keys: KeyInfo[];
  spend: { used: number; cap: number; sponsorCap?: number };
  local: boolean;
  demo: boolean;
  roles: AgentRole[];
  models: AgentModel[];
  /** AI narration voices (read on your own OpenAI key). */
  voices: string[];
  /** The art pack's tavern painting, for the front page. */
  backdrop?: string;
}

/** A seat on the tavern board: a companion in someone's adventure, waiting for a player. */
export interface OpenSeat {
  gameId: string;
  title: string;
  leader: string;
  scene: string;
  character: Creature;
  party: string[];
  portrait?: string;
}

export interface Invite {
  code: string;
  usedBy?: string;
  sponsored?: boolean;
  createdAt: string;
}

export interface Quest {
  title: string;
  status: "open" | "done" | "failed";
  note?: string;
}

/** An open question between a party and a hero's owner. */
export interface Call {
  kind: "ask" | "offer";
  characterId: string;
  ownerId: string;
  name: string;
  summary: string;
}

/** A party's post on the tavern's notice board. */
export interface WantedPost {
  gameId: string;
  title: string;
  leader: string;
  scene: string;
  text: string;
  party: string[];
  offered?: string;
}

export interface Notice {
  gameId: string;
  title: string;
  call: Call;
  from?: string;
}

export interface Notices {
  posts: WantedPost[];
  inbox: Notice[];
  waiting: Notice[];
}

/** A player as the realm's admin sees them. */
export interface RealmPlayer {
  id: string;
  username: string;
  admin: boolean;
  ownKey: boolean;
  sponsored: boolean;
  sponsorCap: number;
  usedToday: number;
}

/** Thrown when the session is missing or expired. */
export class SignedOut extends Error {}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 401 && !path.startsWith("/api/auth/")) {
    window.dispatchEvent(new Event("tabletop:signed-out"));
    throw new SignedOut("Please sign in");
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error ?? res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  me: () => call<Me>("GET", "/api/me"),
  roster: () => call<Character[]>("GET", "/api/characters"),
  tavern: () => call<{ forHire: TavernHero[] }>("GET", "/api/tavern"),
  market: () => call<{ open: Listing[]; mine: Listing[]; feePercent: number }>("GET", "/api/market"),
  listItem: (characterId: string, itemId: string, price: number) => call<Listing>("POST", "/api/market/listings", { characterId, itemId, price }),
  listingAction: (listingId: string, action: "buy" | "withdraw" | "collect", characterId: string) =>
    call<void>("POST", `/api/market/listings/${listingId}/${action}`, { characterId }),
  economy: () => call<EconomyReport>("GET", "/api/admin/economy"),
  setEconomy: (e: Economy) => call<void>("PUT", "/api/admin/economy", e),
  shop: () => call<{ items: ShopItem[]; buyBackPercent: number }>("GET", "/api/shop"),
  shopBuy: (characterId: string, key: string, qty: number) => call<void>("POST", "/api/shop/buy", { characterId, key, qty }),
  shopSell: (characterId: string, itemId: string) => call<{ paid: number }>("POST", "/api/shop/sell", { characterId, itemId }),
  character: (id: string) => call<Character>("GET", `/api/characters/${id}`),
  createCharacter: (c: NewCharacter) => call<Character>("POST", "/api/characters", c),
  randomCharacter: (avoid: string[]) => call<Character>("POST", "/api/characters/random", { avoid }),
  offerSeat: (id: string, characterId: string, open: boolean) => call<void>("POST", `/api/games/${id}/seats`, { characterId, open }),
  giveSeatBack: (id: string, characterId: string) => call<void>("POST", `/api/games/${id}/give-back`, { characterId }),
  notices: () => call<Notices>("GET", "/api/tavern/notices"),
  offerHero: (gameId: string, characterId: string) => call<void>("POST", "/api/tavern/offer", { gameId, characterId }),
  postWanted: (id: string, text: string) => call<void>("POST", `/api/games/${id}/wanted`, { text }),
  askForHero: (id: string, characterId: string) => call<void>("POST", `/api/games/${id}/ask`, { characterId }),
  answerCall: (id: string, characterId: string, yes: boolean) => call<void>("POST", `/api/games/${id}/answer`, { characterId, yes }),
  tavernSeats: () => call<OpenSeat[]>("GET", "/api/tavern/seats"),
  takeSeat: (gameId: string, characterId: string) => call<void>("POST", "/api/tavern/seats/take", { gameId, characterId }),
  improveInTown: (characterId: string, increases: Partial<Record<Ability, number>>) =>
    call<void>("PUT", `/api/characters/${characterId}/improve`, { increases }),
  improveInGame: (id: string, characterId: string, increases: Partial<Record<Ability, number>>) =>
    call<void>("POST", `/api/games/${id}/improve`, { characterId, increases }),
  equipInTown: (characterId: string, itemId: string, slot: Slot | "") =>
    call<void>("PUT", `/api/characters/${characterId}/equip`, { itemId, slot }),
  equipInGame: (id: string, characterId: string, itemId: string, slot: Slot | "") =>
    call<void>("POST", `/api/games/${id}/equip`, { characterId, itemId, slot }),
  setHire: (id: string, hire: Character["hire"]) => call<void>("PUT", `/api/characters/${id}/hire`, { hire }),
  setGender: (id: string, gender: Gender) => call<void>("PUT", `/api/characters/${id}/gender`, { gender }),
  retire: (id: string) => call<void>("DELETE", `/api/characters/${id}`),
  login: (username: string, password: string) => call<void>("POST", "/api/auth/login", { username, password }),
  register: (invite: string, username: string, password: string) =>
    call<void>("POST", "/api/auth/register", { invite, username, password }),
  logout: () => call<void>("POST", "/api/auth/logout"),
  setKey: (kind: string, key: string) => call<void>("PUT", `/api/me/keys/${kind}`, { key }),
  setCap: (cap: number) => call<void>("PUT", "/api/me/cap", { cap }),
  setAgentModel: (model: string) => call<void>("PUT", "/api/me/agent-model", { model }),
  invites: () => call<Invite[]>("GET", "/api/admin/invites"),
  createInvite: (sponsored: boolean) => call<Invite>("POST", "/api/admin/invites", { sponsored }),
  players: () => call<RealmPlayer[]>("GET", "/api/admin/players"),
  sponsor: (accountId: string, tokenCap: number) => call<void>("PUT", `/api/admin/players/${accountId}/sponsor`, { tokenCap }),
  options: () => call<SetupOptions>("GET", "/api/options"),
  list: () => call<SummaryView[]>("GET", "/api/games"),
  create: (input: NewGameInput) => call<{ gameId: string }>("POST", "/api/games", input),
  get: (id: string) => call<Game>("GET", `/api/games/${id}`),
  remove: (id: string) => call<void>("DELETE", `/api/games/${id}`),
  begin: (id: string) => call<void>("POST", `/api/games/${id}/begin`),
  act: (id: string, text: string) => call<void>("POST", `/api/games/${id}/act`, { text }),
  fight: (id: string, option: Option) => call<void>("POST", `/api/games/${id}/fight`, option),
  autopilot: (id: string, characterId: string, on: boolean) => call<void>("POST", `/api/games/${id}/autopilot`, { characterId, on }),
  recruit: (id: string, characterId: string) => call<void>("POST", `/api/games/${id}/recruit`, { characterId }),
  checkCast: (id: string) => call<void>("POST", `/api/games/${id}/cast`),
  setTyping: (id: string, mode: "everyone" | "leader" | "choices") => call<void>("PUT", `/api/games/${id}/typing`, { mode }),
  sendHome: (id: string, characterId: string) => call<void>("POST", `/api/games/${id}/send-home`, { characterId }),
  cover: (id: string, characterId: string, allow: boolean) => call<void>("POST", `/api/games/${id}/cover`, { characterId, allow }),
  repaint: (id: string, subject: string) => call<void>("POST", `/api/games/${id}/art/${subject}`),
};

export function mod(score = 10): number {
  return Math.floor((score - 10) / 2);
}

export function signed(n: number): string {
  return n >= 0 ? `+${n}` : `${n}`;
}

/** "in 1h 42m", "in 3m", "now" for a future ISO time. */
export function untilText(iso: string | undefined, now = Date.now()): string | null {
  if (!iso) return null;
  const ms = new Date(iso).getTime() - now;
  if (ms <= 0) return "any moment now";
  const m = Math.ceil(ms / 60000);
  if (m < 60) return `in ${m}m`;
  const h = Math.floor(m / 60);
  if (h < 48) return `in ${h}h ${m % 60}m`;
  return `in ${Math.round(h / 24)} days`;
}

/** Human length of a Go duration in nanoseconds. */
export function durationText(ns: number): string {
  const m = Math.round(ns / 6e10);
  if (m <= 0) return "never";
  if (m < 60) return `${m} min`;
  return `${Math.round(m / 60)} h`;
}
