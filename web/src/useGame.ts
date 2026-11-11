import { useCallback, useEffect, useRef, useState } from "react";
import type { Game, GameEvent, Roll } from "./api";
import { api } from "./api";
import { sound } from "./sound";

export interface LiveState {
  game: Game | null;
  /** Events received since the last full load, appended to game.log for display. */
  tail: GameEvent[];
  /** Text streaming in right now, keyed by seat. */
  streams: Record<string, string>;
  /** Seat currently composing a reply. */
  thinking: string | null;
  rolls: { id: number; roll: Roll }[];
  error: string | null;
  connected: boolean;
}

/** Turn on with `?debug` in the URL (remembered) and off with `?debug=0`. */
function debugEnabled(): boolean {
  try {
    const q = new URLSearchParams(window.location.search).get("debug");
    if (q !== null) localStorage.setItem("tabletop.debug", q === "0" ? "" : "1");
    return localStorage.getItem("tabletop.debug") === "1";
  } catch {
    return false;
  }
}

export function useGame(id: string) {
  const [state, setState] = useState<LiveState>({
    game: null,
    tail: [],
    streams: {},
    thinking: null,
    rolls: [],
    error: null,
    connected: false,
  });
  const rollSeq = useRef(0);

  const reload = useCallback(async () => {
    try {
      const game = await api.get(id);
      setState((s) => ({ ...s, game, tail: [], streams: {}, error: null }));
      return game;
    } catch (e) {
      setState((s) => ({ ...s, error: (e as Error).message }));
      return null;
    }
  }, [id]);

  useEffect(() => {
    void reload();
    const es = new EventSource(`/api/games/${id}/events`);
    let dropped = false;
    es.onopen = () => {
      setState((s) => ({ ...s, connected: true }));
      // Events sent while we were disconnected are lost; catch up from the saved game.
      if (dropped) void reload();
      dropped = false;
    };
    es.onerror = () => {
      dropped = true;
      setState((s) => ({ ...s, connected: false }));
    };
    const debug = debugEnabled();
    // Streamed text arrives in many small chunks; gather them and draw once per frame.
    let pending: GameEvent[] = [];
    let frame = 0;
    const flush = () => {
      frame = 0;
      const batch = pending;
      pending = [];
      setState((s) => batch.reduce((acc, e) => apply(acc, e, () => ++rollSeq.current), s));
    };
    const queue = (e: GameEvent) => {
      pending.push(e);
      if (!frame) frame = requestAnimationFrame(flush);
    };
    if (debug) console.info(`[tabletop] debug on. Full transcripts: /api/games/${id}/debug (server needs -debug)`);
    es.onmessage = (msg) => {
      const e = JSON.parse(msg.data) as GameEvent;
      if (debug && e.type !== "narration_delta") console.debug("[tabletop]", e.type, e.actor ?? "", e);
      sound.event(e); // live events only: history loaded with the game stays quiet
      if (e.type === "state" || e.type === "image") {
        if (frame) {
          cancelAnimationFrame(frame);
          flush(); // apply what came before the refresh first
        }
        void reload();
        return;
      }
      queue(e);
    };
    return () => {
      es.close();
      if (frame) cancelAnimationFrame(frame);
    };
  }, [id, reload]);

  const consumeRoll = useCallback((rid: number) => {
    setState((s) => ({ ...s, rolls: s.rolls.filter((r) => r.id !== rid) }));
  }, []);

  const clearError = useCallback(() => setState((s) => ({ ...s, error: null })), []);
  const setError = useCallback((error: string) => setState((s) => ({ ...s, error })), []);

  return { ...state, reload, consumeRoll, clearError, setError };
}

function apply(s: LiveState, e: GameEvent, nextRoll: () => number): LiveState {
  switch (e.type) {
    case "narration_delta": {
      const key = e.stream ?? e.actor ?? "dm";
      return { ...s, thinking: null, streams: { ...s.streams, [key]: (s.streams[key] ?? "") + (e.text ?? "") } };
    }
    case "thinking":
      return { ...s, thinking: e.actor ?? "dm", streams: e.actor ? { ...s.streams, [e.actor]: "" } : s.streams };
    case "awaiting":
      return { ...s, thinking: null };
    case "roll":
      return { ...s, tail: [...s.tail, e], rolls: e.roll ? [...s.rolls, { id: nextRoll(), roll: e.roll }] : s.rolls };
    case "narration":
    case "speech": {
      const streams = { ...s.streams };
      delete streams[e.actor ?? "dm"];
      return { ...s, streams, tail: [...s.tail, e] };
    }
    case "scene":
      return {
        ...s,
        tail: [...s.tail, e],
        game: s.game && e.scene ? { ...s.game, scene: e.scene } : s.game,
      };
    case "error":
      return { ...s, thinking: null, tail: [...s.tail, e], error: e.text ?? "Something went wrong" };
    default:
      return { ...s, tail: [...s.tail, e] };
  }
}
