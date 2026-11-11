// Who says what: splits the DM's narration into narrator text and quoted
// dialogue, works out who speaks each quote, and casts a voice for each speaker.
import type { Game, Gender } from "./api";

export interface CastMember {
  id: string;
  name: string;
  kind: "hero" | "monster" | "npc";
  gender?: Gender;
  /** Another way the narration may refer to them ("the innkeeper", "the kobold"). */
  alias?: string;
}

export interface Line {
  text: string;
  /** Cast ID of the speaker; "" for the narrator, "stranger" for an unknown voice. */
  speaker: string;
}

/** Everyone who might speak in a game. */
export function castFrom(game: Game): CastMember[] {
  const out: CastMember[] = game.creatures.map((c) => ({
    id: c.id,
    name: c.name,
    kind: c.kind === "monster" ? "monster" : "hero",
    gender: c.gender,
  }));
  for (const n of game.npcs ?? []) {
    out.push({ id: n.id, name: n.name, kind: n.monster ? "monster" : "npc", gender: n.gender, alias: n.type.replace(/_/g, " ") });
  }
  return out;
}

const quote = /“([^”]+)”|"([^"]+)"/g;

/** Splits narration into narrator text and quoted lines with their speakers. */
export function splitLines(text: string, cast: CastMember[]): Line[] {
  const out: Line[] = [];
  let last = 0;
  let previous = "";
  for (const m of text.matchAll(quote)) {
    const start = m.index ?? 0;
    const end = start + m[0].length;
    const before = text.slice(last, start);
    if (before.trim()) out.push({ text: before, speaker: "" });
    const said = m[1] ?? m[2] ?? "";
    const speaker = attribute(text, said, start, end, last, cast) || previous || "stranger";
    out.push({ text: said, speaker });
    previous = speaker;
    last = end;
  }
  const rest = text.slice(last);
  if (rest.trim()) out.push({ text: rest, speaker: "" });
  return out;
}

/**
 * Splits a character's own line: what they say (in quotes) is theirs; the
 * stage directions around it (*draws her sword*) go to the narrator, unless
 * written in the first person ("*I pull the scale from my pack*"). A line with
 * no quotes is all theirs.
 */
export function splitSpeech(text: string, speaker: string): Line[] {
  if (!new RegExp(quote.source).test(text)) return [{ text, speaker }];
  const out: Line[] = [];
  let last = 0;
  const aside = (s: string) => {
    if (!s.trim()) return;
    out.push({ text: s, speaker: /(^|[\s*])(I|I'm|I've|my|me)\b/.test(s) ? speaker : "" });
  };
  for (const m of text.matchAll(quote)) {
    const start = m.index ?? 0;
    aside(text.slice(last, start));
    out.push({ text: m[1] ?? m[2] ?? "", speaker });
    last = start + m[0].length;
  }
  aside(text.slice(last));
  return out;
}

/**
 * Finds who speaks a quote: a name in the short tag right after it
 * ("Run!" shouts Bryn) wins; otherwise the last name mentioned before it in
 * the same paragraph (Bryn leans in. "Two of them..."); failing that, someone
 * in the sentence after it ("No." Maren turns away).
 */
function attribute(text: string, said: string, start: number, end: number, from: number, cast: CastMember[]): string {
  const tail = /^[^.!?\n"“]{0,60}/.exec(text.slice(end))?.[0] ?? "";
  // A tag reads as one when the quote breaks off with a comma, the tag starts
  // in lower case, or it has a speaking verb: "Run!" shouts Bryn.
  const tag = /,\s*$/.test(said) || /^\s*[a-z]/.test(tail) || speaking.test(tail);
  const after = tag ? mentions(tail, cast) : [];
  if (after.length) return after[0].id;
  const paragraph = text.slice(from, start).split(/\n\s*\n/).pop() ?? "";
  const before = mentions(paragraph.slice(-240), cast);
  if (before.length) return before[before.length - 1].id;
  // An action beat after the line: "I will not." Maren turns away.
  const beat = tag ? [] : mentions(tail, cast);
  return beat.length ? beat[0].id : "";
}

const speaking = /\b(say|said|says|ask|asks|asked|repl(y|ies|ied)|shout(s|ed)?|growl(s|ed)?|snarl(s|ed)?|whisper(s|ed)?|mutter(s|ed)?|call(s|ed)?|cr(y|ies|ied)|hiss(es|ed)?|squeal(s|ed)?|bark(s|ed)?|add(s|ed)?|answer(s|ed)?|laugh(s|ed)?|snap(s|ped)?|murmur(s|ed)?|yell(s|ed)?|roar(s|ed)?|sigh(s|ed)?|grumble(s|d)?|declare(s|d)?|demand(s|ed)?|warn(s|ed)?|plead(s|ed)?|bellow(s|ed)?|croak(s|ed)?|rasp(s|ed)?)\b/i;

/** Cast members named in a stretch of text, in order of mention. */
function mentions(s: string, cast: CastMember[]) {
  const found: { id: string; at: number }[] = [];
  for (const c of cast) {
    const at = lastIndexOfName(s, c);
    if (at >= 0) found.push({ id: c.id, at });
  }
  return found.sort((a, b) => a.at - b.at);
}

function lastIndexOfName(s: string, c: CastMember) {
  // Monsters are often "the goblin"; people go by their first name.
  const base = c.name.replace(/\s*\d+$/, "").trim();
  const first = base.split(/\s+/)[0];
  const names = c.kind === "monster" ? [base] : [base, first].filter((n) => n.length >= 3);
  let best = -1;
  const all = names.map((n) => ({ n, flags: c.kind === "monster" ? "gi" : "g" }));
  if (c.alias) all.push({ n: `the ${c.alias}`, flags: "gi" });
  for (const { n, flags } of all) {
    const re = new RegExp(`\\b${escape(n)}\\b`, flags);
    for (const m of s.matchAll(re)) best = Math.max(best, m.index ?? -1);
  }
  return best;
}

function escape(s: string) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// OpenAI voices by how they tend to sound.
const female = ["coral", "nova", "shimmer", "sage", "marin"];
const male = ["ash", "ballad", "echo", "onyx", "verse", "cedar", "fable"];
const gruff = ["onyx", "ash"];


/** The AI voice for a speaker: a player's pick, else a steady voice that suits them. */
export function aiVoice(speaker: string, cast: CastMember[], narrator: string, picks: Record<string, string>, all: string[]) {
  if (!speaker) return narrator;
  if (picks[speaker]) return picks[speaker];
  if (speaker === "stranger") return narrator === "alloy" ? "sage" : "alloy";
  const c = cast.find((m) => m.id === speaker);
  const g = c?.gender;
  let pool = c?.kind === "monster" ? gruff : g === "f" ? female : g === "m" ? male : all;
  pool = pool.filter((v) => v !== narrator && all.includes(v));
  if (!pool.length) pool = all.filter((v) => v !== narrator);
  return pool.length ? pool[hash(speaker) % pool.length] : narrator;
}

/** A browser-voice pitch for a speaker: higher for women, lower for monsters. */
export function pitch(speaker: string, cast: CastMember[]) {
  if (!speaker) return 1;
  const c = cast.find((m) => m.id === speaker);
  const spread = (hash(speaker) % 15) / 100;
  if (c?.kind === "monster") return 0.6 + spread;
  const g = c?.gender;
  if (g === "f") return 1.2 + spread;
  if (g === "m") return 0.8 + spread;
  return 0.9 + spread * 2;
}

export function hash(s: string) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h;
}
