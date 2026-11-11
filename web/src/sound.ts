// Sound for the table: effects, ambience and narration, all made in the
// browser. Effects and ambience are synthesised with Web Audio (no audio files,
// nothing to license); narration uses the browser's own speech voices.
import { useEffect, useState } from "react";
import type { GameEvent } from "./api";
import type { CastMember, Line } from "./voices";
import { aiVoice, pitch, splitLines, splitSpeech } from "./voices";

export interface SoundSettings {
  /** Master switch for effects and ambience. */
  on: boolean;
  /** Volumes, 0 to 1. */
  effects: number;
  ambience: number;
  /** Read the story aloud. */
  narrate: boolean;
  /** Who reads: the browser's built-in voices (free) or an AI voice on your OpenAI key. */
  engine: "browser" | "ai";
  /** Browser voice name; empty means the browser default. */
  voice: string;
  /** AI narrator voice; characters get the other AI voices. */
  aiVoice: string;
  /** AI voices the player picked for particular characters, by cast ID. */
  castVoices: Record<string, string>;
  /** Drop unread lines when you act, so the reading never falls behind play. */
  catchUp: boolean;
  rate: number;
}

/** What the narrator is doing, for the "now reading" bar. */
export interface Reading {
  active: boolean;
  who: string;
  waiting: number;
}

interface Job {
  text: string;
  speaker: string;
  clip?: Promise<string>;
}

const defaults: SoundSettings = { on: false, effects: 0.7, ambience: 0.35, narrate: false, engine: "browser", voice: "", aiVoice: "fable", castVoices: {}, catchUp: true, rate: 1 };
const storeKey = "hearthbound.sound";

function load(): SoundSettings {
  try {
    return { ...defaults, ...(JSON.parse(localStorage.getItem(storeKey) ?? "{}") as Partial<SoundSettings>) };
  } catch {
    return defaults;
  }
}

type Listener = (s: SoundSettings) => void;

/** Effects the UI can play. Server cues (domain.Cue) are a subset. */
export type Effect =
  | "dice" | "hit" | "miss" | "crit" | "damage" | "turn" | "scene"
  | "combat" | "victory" | "defeat" | "fall" | "coins" | "loot" | "level"
  | "xp" | "rest" | "arrive" | "leave" | "chapter";

/** Ambience beds: one per scene key, plus combat. */
type Bed = { stop: (fade: number) => void };

class Engine {
  settings = load();
  private listeners = new Set<Listener>();
  private ctx: AudioContext | null = null;
  private fx!: GainNode;
  private amb!: GainNode;
  private white!: AudioBuffer;
  private brown!: AudioBuffer;
  private bed: Bed | null = null;
  private bedKey = "";
  private wanted = "";
  private speaking = 0;
  /** Lines waiting to be read, in order. AI clips are fetched as they arrive. */
  private jobs: Job[] = [];
  private current: { speaker: string; stop: () => void } | null = null;
  private pumping = false;
  private readers = new Set<(r: Reading) => void>();
  private gameId = "";
  private cast: CastMember[] = [];
  /** The AI voices on offer (from /api/me). */
  aiVoices: string[] = [];
  /** Why the last AI line fell back to the browser voice, if it did. */
  aiError = "";

  subscribe(fn: Listener) {
    this.listeners.add(fn);
    return () => void this.listeners.delete(fn);
  }

  update(patch: Partial<SoundSettings>) {
    this.settings = { ...this.settings, ...patch };
    localStorage.setItem(storeKey, JSON.stringify(this.settings));
    this.listeners.forEach((fn) => fn(this.settings));
    if (this.settings.on) this.unlock();
    this.levels();
    if (!this.settings.narrate) this.hush();
    this.ambience(this.wanted);
  }

  /** Browsers only allow audio after a click; call this from one. */
  unlock() {
    if (!this.settings.on) return;
    if (!this.ctx) {
      const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      if (!Ctx) return;
      const ctx = new Ctx();
      this.ctx = ctx;
      this.fx = ctx.createGain();
      this.amb = ctx.createGain();
      this.fx.connect(ctx.destination);
      this.amb.connect(ctx.destination);
      this.white = this.noise(ctx, false);
      this.brown = this.noise(ctx, true);
      this.levels();
    }
    if (this.ctx.state === "suspended") void this.ctx.resume();
    this.ambience(this.wanted);
  }

  private noise(ctx: AudioContext, brown: boolean) {
    const buf = ctx.createBuffer(1, ctx.sampleRate * 3, ctx.sampleRate);
    const d = buf.getChannelData(0);
    let last = 0;
    for (let i = 0; i < d.length; i++) {
      const w = Math.random() * 2 - 1;
      if (!brown) {
        d[i] = w;
        continue;
      }
      last = (last + 0.02 * w) / 1.02;
      d[i] = last * 3.5;
    }
    return buf;
  }

  private levels() {
    if (!this.ctx) return;
    const s = this.settings;
    const t = this.ctx.currentTime;
    this.fx.gain.setTargetAtTime(s.on ? s.effects : 0, t, 0.05);
    // Duck the ambience while a voice is reading.
    const duck = this.speaking > 0 ? 0.45 : 1;
    this.amb.gain.setTargetAtTime(s.on ? s.ambience * duck : 0, t, 0.4);
  }

  private ready() {
    return this.settings.on && this.ctx && this.ctx.state === "running" ? this.ctx : null;
  }

  // ---- building blocks ----

  /** A tone with a quick attack and exponential decay. */
  private tone(at: number, freq: number, dur: number, vol: number, type: OscillatorType = "sine", to?: number, dest?: AudioNode) {
    const ctx = this.ctx!;
    const o = ctx.createOscillator();
    const g = ctx.createGain();
    o.type = type;
    o.frequency.setValueAtTime(freq, at);
    if (to) o.frequency.exponentialRampToValueAtTime(to, at + dur);
    g.gain.setValueAtTime(0.0001, at);
    g.gain.exponentialRampToValueAtTime(vol, at + 0.008);
    g.gain.exponentialRampToValueAtTime(0.0001, at + dur);
    o.connect(g).connect(dest ?? this.fx);
    o.start(at);
    o.stop(at + dur + 0.05);
  }

  /** A burst of filtered noise. */
  private hiss(at: number, dur: number, vol: number, filter: BiquadFilterType, freq: number, q = 1, to?: number, dest?: AudioNode, attack = 0.005) {
    const ctx = this.ctx!;
    const src = ctx.createBufferSource();
    src.buffer = this.white;
    const f = ctx.createBiquadFilter();
    f.type = filter;
    f.frequency.setValueAtTime(freq, at);
    if (to) f.frequency.exponentialRampToValueAtTime(to, at + dur);
    f.Q.value = q;
    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, at);
    g.gain.exponentialRampToValueAtTime(vol, at + attack);
    g.gain.exponentialRampToValueAtTime(0.0001, at + dur);
    src.connect(f).connect(g).connect(dest ?? this.fx);
    src.start(at, Math.random() * 2);
    src.stop(at + dur + 0.05);
  }

  /** A struck-metal ring: inharmonic partials. */
  private metal(at: number, base: number, dur: number, vol: number, dest?: AudioNode) {
    for (const [ratio, v] of [[1, 1], [2.76, 0.6], [5.4, 0.35], [8.93, 0.2]]) this.tone(at, base * ratio, dur / ratio ** 0.3, vol * v, "sine", undefined, dest);
  }

  private notes(at: number, freqs: number[], step: number, dur: number, vol: number, type: OscillatorType = "triangle") {
    freqs.forEach((f, i) => this.tone(at + i * step, f, dur, vol, type));
  }

  private drum(at: number, freq: number, vol: number, dest?: AudioNode) {
    this.tone(at, freq, 0.35, vol, "sine", freq * 0.5, dest);
    this.hiss(at, 0.08, vol * 0.4, "lowpass", 900, 1, undefined, dest);
  }

  // ---- effects ----

  play(effect: Effect) {
    const ctx = this.ready();
    if (!ctx) return;
    const t = ctx.currentTime + 0.01;
    const r = Math.random;
    switch (effect) {
      case "dice":
        for (let i = 0; i < 7; i++) this.hiss(t + i * (0.05 + r() * 0.06), 0.04, 0.5 - i * 0.05, "bandpass", 2500 + r() * 2500, 4);
        break;
      case "hit":
        this.hiss(t, 0.12, 0.6, "lowpass", 1800);
        this.metal(t, 520 + r() * 80, 0.6, 0.18);
        break;
      case "crit":
        this.hiss(t, 0.15, 0.7, "lowpass", 2400);
        this.metal(t, 600, 0.9, 0.22);
        this.notes(t + 0.12, [523, 659, 784, 1047], 0.07, 0.5, 0.12);
        break;
      case "miss":
        this.hiss(t, 0.28, 0.35, "bandpass", 500, 2, 3000, undefined, 0.08);
        break;
      case "damage":
        this.drum(t, 110, 0.6);
        break;
      case "turn":
        this.notes(t, [784, 1175], 0.12, 0.6, 0.12, "sine");
        break;
      case "scene":
        this.hiss(t, 0.5, 0.25, "bandpass", 3000, 1, 900, undefined, 0.15);
        this.hiss(t + 0.18, 0.35, 0.2, "bandpass", 2400, 1, 700, undefined, 0.1);
        break;
      case "combat":
        [0, 0.22, 0.44, 0.88].forEach((d, i) => this.drum(t + d, i === 3 ? 70 : 95, 0.7));
        this.tone(t + 0.88, 147, 1.2, 0.1, "sawtooth");
        break;
      case "victory":
        this.notes(t, [392, 523, 659], 0.12, 0.4, 0.15);
        [523, 659, 784].forEach((f) => this.tone(t + 0.4, f, 1.4, 0.1, "triangle"));
        break;
      case "defeat":
        this.notes(t, [330, 294, 247, 196], 0.35, 1.2, 0.12, "sine");
        this.drum(t + 1.1, 55, 0.7);
        break;
      case "fall":
        this.drum(t, 60, 0.8);
        this.tone(t + 0.1, 220, 1.6, 0.08, "sine", 110);
        break;
      case "coins":
        for (let i = 0; i < 5; i++) this.metal(t + i * (0.04 + r() * 0.05), 2600 + r() * 900, 0.25, 0.07);
        break;
      case "loot":
        this.hiss(t, 0.18, 0.2, "lowpass", 1200);
        this.notes(t + 0.1, [659, 880, 1319], 0.08, 0.5, 0.11, "sine");
        break;
      case "level":
        this.notes(t, [523, 659, 784, 1047, 1319], 0.09, 0.8, 0.13);
        this.metal(t + 0.45, 1047, 1.4, 0.06);
        break;
      case "xp":
        this.notes(t, [880, 1109], 0.1, 0.5, 0.09, "sine");
        break;
      case "rest":
        [262, 330, 392].forEach((f) => this.tone(t, f, 2.2, 0.06, "sine"));
        break;
      case "arrive":
        this.hiss(t, 0.09, 0.4, "lowpass", 600); // a door
        this.drum(t + 0.02, 140, 0.25);
        this.notes(t + 0.12, [587, 784], 0.1, 0.4, 0.07, "sine");
        break;
      case "leave":
        this.notes(t, [784, 587], 0.12, 0.45, 0.07, "sine");
        this.hiss(t + 0.25, 0.09, 0.35, "lowpass", 500);
        break;
      case "chapter":
        this.hiss(t, 0.6, 0.25, "bandpass", 2800, 1, 800, undefined, 0.2);
        [196, 294, 392].forEach((f) => this.tone(t + 0.3, f, 2, 0.07, "triangle"));
        break;
    }
  }

  // ---- ambience ----

  /** Switch the ambient bed: a scene key, "combat", or "" for silence. */
  ambience(key: string) {
    this.wanted = key;
    const ctx = this.ready();
    const target = ctx && this.settings.ambience > 0 ? key : "";
    if (target === this.bedKey) return;
    this.bed?.stop(2.5);
    this.bed = null;
    this.bedKey = target;
    if (ctx && target) this.bed = this.makeBed(ctx, target);
  }

  private makeBed(ctx: AudioContext, key: string): Bed {
    const out = ctx.createGain();
    out.gain.setValueAtTime(0.0001, ctx.currentTime);
    out.gain.exponentialRampToValueAtTime(1, ctx.currentTime + 3);
    out.connect(this.amb);
    const nodes: AudioScheduledSourceNode[] = [];
    const timers: number[] = [];

    /** A looping noise layer through a filter, optionally swept by a slow LFO. */
    const layer = (brown: boolean, filter: BiquadFilterType, freq: number, vol: number, sway = 0, rate = 0.07) => {
      const src = ctx.createBufferSource();
      src.buffer = brown ? this.brown : this.white;
      src.loop = true;
      const f = ctx.createBiquadFilter();
      f.type = filter;
      f.frequency.value = freq;
      const g = ctx.createGain();
      g.gain.value = vol;
      src.connect(f).connect(g).connect(out);
      if (sway) {
        const lfo = ctx.createOscillator();
        const depth = ctx.createGain();
        lfo.frequency.value = rate;
        depth.gain.value = sway;
        lfo.connect(depth).connect(f.frequency);
        lfo.start();
        nodes.push(lfo);
      }
      src.start(0, Math.random() * 2);
      nodes.push(src);
    };
    /** A sustained drone of sines. */
    const drone = (freqs: number[], vol: number) => {
      for (const fr of freqs) {
        const o = ctx.createOscillator();
        o.frequency.value = fr;
        const g = ctx.createGain();
        g.gain.value = vol;
        const vib = ctx.createOscillator();
        const vd = ctx.createGain();
        vib.frequency.value = 0.15 + Math.random() * 0.2;
        vd.gain.value = fr * 0.004;
        vib.connect(vd).connect(o.frequency);
        o.connect(g).connect(out);
        o.start();
        vib.start();
        nodes.push(o, vib);
      }
    };
    /** Something that happens now and then: every min..max seconds. */
    const every = (min: number, max: number, fn: (t: number) => void) => {
      const tick = () => {
        fn(ctx.currentTime + 0.05);
        timers.push(window.setTimeout(tick, (min + Math.random() * (max - min)) * 1000));
      };
      timers.push(window.setTimeout(tick, Math.random() * max * 1000));
    };
    const r = Math.random;
    const crackle = (t: number) => {
      for (let i = 0; i < 1 + r() * 3; i++) this.hiss(t + r() * 0.3, 0.02 + r() * 0.03, 0.15 + r() * 0.2, "highpass", 1500 + r() * 2000, 1, undefined, out);
    };
    const bird = (t: number) => {
      const f = 2200 + r() * 1800;
      for (let i = 0; i < 2 + r() * 4; i++) this.tone(t + i * 0.11, f, 0.09, 0.05, "sine", f * (1.2 + r() * 0.4), out);
    };
    const drip = (t: number) => this.tone(t, 1400 + r() * 900, 0.18, 0.08, "sine", 500, out);
    const wind = (vol: number, low = 400) => layer(false, "bandpass", low, vol, low * 0.6, 0.05 + r() * 0.05);
    const crickets = (t: number) => {
      for (let i = 0; i < 6; i++) this.tone(t + i * 0.045, 4400, 0.03, 0.025, "sine", undefined, out);
    };

    switch (key) {
      case "tavern":
        layer(true, "lowpass", 500, 0.5, 150, 0.2); // murmur
        every(0.2, 0.9, crackle);
        every(5, 14, (t) => { this.metal(t, 1900 + r() * 400, 0.3, 0.03, out); this.metal(t + 0.08, 2100 + r() * 400, 0.3, 0.025, out); });
        break;
      case "village":
      case "town":
        layer(true, "lowpass", 650, key === "town" ? 0.4 : 0.22, 120, 0.15);
        every(3, 9, bird);
        if (key === "town") every(25, 50, (t) => this.metal(t, 330, 3, 0.05, out));
        break;
      case "forest":
        wind(0.15, 600);
        every(1.5, 5, bird);
        break;
      case "road":
        wind(0.22, 500);
        every(5, 12, bird);
        break;
      case "mountains":
        wind(0.4, 350);
        wind(0.15, 900);
        break;
      case "swamp":
        layer(true, "lowpass", 300, 0.3);
        every(0.8, 3, (t) => { for (let i = 0; i < 2 + r() * 2; i++) this.tone(t + i * 0.16, 95 + r() * 30, 0.12, 0.12, "square", 70, out); });
        every(2, 6, (t) => this.tone(t, 200 + r() * 200, 0.12, 0.06, "sine", 700, out));
        break;
      case "cave":
      case "dungeon":
      case "crypt":
        drone(key === "crypt" ? [41, 41.4, 61.7] : [55, 55.6], 0.06);
        layer(true, "lowpass", 180, 0.25);
        every(1.5, key === "cave" ? 3.5 : 6, drip);
        if (key === "crypt") every(14, 30, (t) => this.hiss(t, 3, 0.12, "bandpass", 300, 6, 180, out, 1.2));
        break;
      case "castle":
        wind(0.18, 300);
        drone([65.4, 98], 0.035);
        every(12, 28, (t) => this.metal(t, 180 + r() * 60, 1.2, 0.04, out));
        break;
      case "camp":
        every(0.15, 0.6, crackle);
        every(1, 3, crickets);
        wind(0.08, 500);
        break;
      case "ruins":
        wind(0.3, 450);
        every(10, 25, (t) => this.hiss(t, 0.4, 0.2, "lowpass", 220, 1, undefined, out));
        break;
      case "temple":
        drone([110, 164.8, 220, 277.2], 0.03);
        layer(true, "lowpass", 250, 0.12);
        break;
      case "combat":
        layer(true, "lowpass", 300, 0.25);
        every(1.9, 1.9, (t) => [0, 0.32, 0.63, 0.79, 1.27].forEach((d, i) => this.drum(t + d, i % 2 ? 82 : 62, 0.3, out)));
        break;
      default:
        wind(0.12, 500);
    }

    return {
      stop: (fade) => {
        timers.forEach((id) => window.clearTimeout(id));
        const now = ctx.currentTime;
        out.gain.cancelScheduledValues(now);
        out.gain.setValueAtTime(out.gain.value || 0.0001, now);
        out.gain.exponentialRampToValueAtTime(0.0001, now + fade);
        nodes.forEach((n) => n.stop(now + fade + 0.1));
        window.setTimeout(() => out.disconnect(), (fade + 0.3) * 1000);
      },
    };
  }

  // ---- narration ----

  /** The game being read, and who might speak in it. */
  setCast(gameId: string, cast: CastMember[]) {
    this.gameId = gameId;
    this.cast = cast;
  }

  /** Stops the line being read; the next one starts. */
  skip() {
    this.current?.stop();
  }

  /** Drops everything waiting to be read and stops the current line. */
  skipAll() {
    if (this.jobs.length) debug("dropped", this.jobs.length, "unread line(s)");
    for (const job of this.jobs) job.clip?.then(URL.revokeObjectURL).catch(() => undefined);
    this.jobs = [];
    this.current?.stop();
    this.status();
  }

  private hush() {
    this.skipAll();
  }

  /** What is being read: the speaker's name and how many lines wait. */
  reading(): Reading {
    const who = this.current?.speaker;
    const name = who === undefined ? "" : who === "" ? "Narrator" : (this.cast.find((c) => c.id === who)?.name ?? "A voice");
    return { active: !!this.current, who: name, waiting: this.jobs.length };
  }

  private status() {
    const r = this.reading();
    this.readers.forEach((fn) => fn(r));
  }

  onReading(fn: (r: Reading) => void) {
    this.readers.add(fn);
    return () => void this.readers.delete(fn);
  }

  /** Queues narration (split into narrator and dialogue) or a character's line. */
  speak(text: string, actor?: string) {
    const s = this.settings;
    if (!s.narrate) return;
    const lines: Line[] = !actor || actor === "dm" ? splitLines(text, this.cast) : splitSpeech(text, actor);
    for (const line of lines) {
      const clean = line.text.replace(/[*_#>`~]/g, "").replace(/\s+/g, " ").trim().slice(0, 4000);
      if (!/[\p{L}\p{N}]/u.test(clean)) continue;
      const job: Job = { text: clean, speaker: line.speaker };
      debug("queue", job.speaker || "narrator", clean.slice(0, 40));
      if (s.engine === "ai") job.clip = this.fetchClip(job);
      this.jobs.push(job);
    }
    this.status();
    void this.pump();
  }

  private async pump() {
    if (this.current || this.pumping) return;
    this.pumping = true;
    while (this.jobs.length) {
      const job = this.jobs.shift()!;
      await this.perform(job);
    }
    this.pumping = false;
    this.current = null;
    this.status();
  }

  /** Starts fetching an AI clip now, so it is ready when its turn comes. */
  private fetchClip(job: Job) {
    const voice = aiVoice(job.speaker, this.cast, this.settings.aiVoice || "fable", this.settings.castVoices, this.aiVoices);
    const character = job.speaker !== "";
    const body = { text: job.text, voice, style: character ? "character" : "narrator", gameId: this.gameId, speaker: character && job.speaker !== "stranger" ? job.speaker : "" };
    return fetch("/api/speech", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify(body),
    }).then(async (res) => {
      if (!res.ok) throw new Error((await res.text()) || `HTTP ${res.status}`);
      return URL.createObjectURL(await res.blob());
    });
  }

  private async perform(job: Job) {
    if (!job.clip) return this.say(job, () => this.speakBrowser(job));
    let url = "";
    try {
      url = await job.clip;
    } catch (e) {
      this.aiError = aiProblem((e as Error).message);
      this.listeners.forEach((fn) => fn(this.settings));
      return this.say(job, () => this.speakBrowser(job));
    }
    this.aiError = "";
    await this.say(job, () => this.playClip(url));
  }

  /** Runs one line, ducking the ambience and making it skippable. */
  private async say(job: Job, run: () => { done: Promise<void>; stop: () => void }) {
    const { done, stop } = run();
    this.current = { speaker: job.speaker, stop };
    this.speaking = 1;
    this.levels();
    this.status();
    await done;
    this.current = null;
    this.speaking = 0;
    this.levels();
  }

  private playClip(url: string) {
    const el = new Audio(url);
    el.playbackRate = this.settings.rate;
    const done = new Promise<void>((resolve) => {
      const end = () => {
        URL.revokeObjectURL(url);
        resolve();
      };
      el.onended = end;
      el.onerror = end;
      el.onpause = end;
      el.play().catch(end);
    });
    return { done, stop: () => el.pause() };
  }

  private speakBrowser(job: Job) {
    if (!("speechSynthesis" in window)) return { done: Promise.resolve(), stop: () => undefined };
    const s = this.settings;
    const u = new SpeechSynthesisUtterance(job.text);
    const voice = window.speechSynthesis.getVoices().find((v) => v.name === s.voice);
    if (voice) u.voice = voice;
    u.rate = s.rate;
    u.pitch = pitch(job.speaker, this.cast);
    const done = new Promise<void>((resolve) => {
      u.onend = () => resolve();
      u.onerror = () => resolve();
    });
    window.speechSynthesis.speak(u);
    return { done, stop: () => window.speechSynthesis.cancel() };
  }

  /** Reacts to a live event from the table (never to history on load). */
  event(e: GameEvent) {
    switch (e.type) {
      case "roll": {
        this.play("dice");
        const roll = e.roll;
        if (!roll) return;
        window.setTimeout(() => this.play(rollResult(roll)), 650);
        return;
      }
      case "scene":
        this.play("scene");
        return;
      case "system":
      case "level_up":
        if (e.cue) this.play(e.cue as Effect);
        return;
      case "narration":
        this.speak(e.text ?? "", "dm");
        return;
      case "speech":
        this.speak(e.text ?? "", e.actor);
        return;
    }
  }
}

function rollResult(roll: NonNullable<GameEvent["roll"]>): Effect {
  if (roll.critical) return "crit";
  if (roll.success === undefined) return "damage";
  return roll.success ? "hit" : "miss";
}

/** Logs narration decisions to the console when ?debug is on. */
function debug(...args: unknown[]) {
  try {
    if (localStorage.getItem("tabletop.debug") === "1") console.debug("[narration]", ...args);
  } catch {
    // no storage, no logging
  }
}

/** A short, readable reason an AI line could not be read. */
function aiProblem(msg: string) {
  if (/no API key/i.test(msg)) return "Add an OpenAI key in Settings to use AI voices.";
  if (/invalid_api_key|401/.test(msg)) return "Your OpenAI key was rejected.";
  return "The AI voice failed; using the browser voice.";
}

/** The one sound engine for the page. */
export const sound = new Engine();

// The first click anywhere lets the browser play audio.
if (typeof window !== "undefined") {
  const first = () => sound.unlock();
  window.addEventListener("pointerdown", first);
  window.addEventListener("keydown", first);
}

/** Live sound settings for components. */
export function useSoundSettings() {
  const [s, set] = useState(sound.settings);
  useEffect(() => sound.subscribe(set), []);
  return s;
}

/** What is being read right now. */
export function useReading() {
  const [r, set] = useState<Reading>(sound.reading());
  useEffect(() => sound.onReading(set), []);
  return r;
}

/** The browser's speech voices (they load asynchronously). */
export function useVoices() {
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([]);
  useEffect(() => {
    if (!("speechSynthesis" in window)) return;
    const read = () => setVoices(window.speechSynthesis.getVoices().filter((v) => v.lang.startsWith("en")));
    read();
    window.speechSynthesis.addEventListener("voiceschanged", read);
    return () => window.speechSynthesis.removeEventListener("voiceschanged", read);
  }, []);
  return voices;
}
