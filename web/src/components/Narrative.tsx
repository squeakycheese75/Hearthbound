import { motion } from "motion/react";
import { Fragment, memo, useEffect, useRef } from "react";
import type { Creature, GameEvent } from "../api";
import { Portrait } from "./Portrait";

/** Renders "spoken words" and *actions* the way companions write them. */
function Rich({ text }: { text: string }) {
  const parts = text.split(/(\*[^*]+\*|"[^"]+"|“[^”]+”)/g);
  return (
    <>
      {parts.map((p, i) => {
        if (p.startsWith("*") && p.endsWith("*")) return <em key={i} className="text-sepia/70">{p.slice(1, -1)}</em>;
        if (p.startsWith('"') || p.startsWith("“")) return <span key={i} className="text-sepia">{p}</span>;
        return <Fragment key={i}>{p}</Fragment>;
      })}
    </>
  );
}

function Paragraphs({ text, caret = false, first = false }: { text: string; caret?: boolean; first?: boolean }) {
  const paras = text.split(/\n{2,}/).filter((p) => p.trim());
  return (
    <>
      {paras.map((p, i) => (
        <p key={i} className={`mb-3 last:mb-0 ${first && i === 0 ? "dropcap" : ""} ${caret && i === paras.length - 1 ? "caret" : ""}`}>
          {p}
        </p>
      ))}
    </>
  );
}

function Speech({ c, text, live = false }: { c?: Creature; text: string; live?: boolean }) {
  return (
    <div className="flex gap-3 items-start my-4">
      {c ? <Portrait c={c} size={38} /> : <div className="w-[38px]" />}
      <div className="inked min-w-0 flex-1 px-4 py-2 text-[17px] leading-relaxed text-sepia">
        <div className="font-display text-[12px] tracking-[0.14em] uppercase text-[#7a2a1d] mb-0.5">{c?.name ?? "Companion"}</div>
        <span className={live ? "caret" : ""}>
          <Rich text={text} />
        </span>
      </div>
    </div>
  );
}

function RollLine({ e, who }: { e: GameEvent; who?: Creature }) {
  const r = e.roll!;
  const ok = r.success;
  const mark = ok === undefined ? "" : ok ? "✓" : "✗";
  const color = r.critical ? "text-[#a8741c]" : ok === false ? "text-[#9b2f22]" : ok ? "text-[#2f6e55]" : "text-sepia";
  return (
    <div className="flex items-baseline gap-2 text-[15px] text-sepia/65 my-0.5 sm:pl-6">
      <span aria-hidden className="text-[#a07a3a]">⬢</span>
      <span className="min-w-0 flex-1 truncate italic" title={r.label}>
        {r.label || who?.name}
      </span>
      <span className={`shrink-0 font-display font-bold tabular-nums ${color}`}>
        {r.total}
        {r.target ? ` / ${r.target}` : ""} {mark}
      </span>
    </div>
  );
}

/** A hero's level-up, in the story: what they gained. */
function LevelUpCard({ who, e }: { who?: Creature; e: GameEvent }) {
  return (
    <div className="my-6 rounded-lg border-2 border-[#a8741c]/60 bg-[#f3e2b3]/70 px-5 py-4 text-sepia shadow-[inset_0_0_24px_rgba(168,116,28,.18)]">
      <div className="flex items-center gap-3">
        {who && <Portrait c={who} size={44} />}
        <div>
          <div className="font-display text-[12px] uppercase tracking-[0.25em] text-[#a8741c]">Level up</div>
          <div className="font-display text-xl text-[#5a2a10]">{e.text}</div>
        </div>
      </div>
      <ul className="mt-3 space-y-1 text-[16px] leading-snug">
        {(e.levelUp?.gains ?? []).map((g) => (
          <li key={g} className="flex gap-2">
            <span aria-hidden className="text-[#a8741c]">✦</span>
            <span>{g}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export interface NarrativeProps {
  log: GameEvent[];
  /** Position of log[0] in the whole story, so keys stay stable when old events are trimmed. */
  base?: number;
  /** Changes when the action bar changes size (choices appear), to keep the story's end in view. */
  settleKey?: string;
  streams: Record<string, string>;
  thinking: string | null;
  creatures: Creature[];
  heroId?: string;
}

/**
 * The story so far. Memoised: streaming text changes many times a second and
 * must not redraw every past paragraph each time.
 */
const History = memo(function History({ log, base, creatures, heroId }: { log: GameEvent[]; base: number; creatures: Creature[]; heroId?: string }) {
  const byId = (id?: string) => creatures.find((c) => c.id === id);
  let narrationCount = 0;
  return (
    <>
      {log.map((e, n) => {
        const i = base + n;
        switch (e.type) {
          case "narration":
            narrationCount++;
            return (
              // No fade-in: this text usually just finished streaming, and fading it in again made it blink.
              <div key={i} className="my-5 text-sepia">
                <Paragraphs text={e.text ?? ""} first={narrationCount === 1} />
              </div>
            );
          case "speech":
            return <Speech key={i} c={byId(e.actor)} text={e.text ?? ""} />;
          case "player":
            return (
              <div key={i} className="my-5 flex justify-end">
                <div className="max-w-[85%] rounded-lg border border-[#b08a4a]/50 bg-[#d9c497]/60 px-4 py-2 italic text-sepia">
                  <span className="not-italic font-display text-[12px] tracking-[0.14em] uppercase text-[#7a2a1d] mr-2">
                    {byId(e.actor ?? heroId)?.name ?? "You"}
                  </span>
                  {e.text}
                </div>
              </div>
            );
          case "roll":
            return <RollLine key={i} e={e} who={byId(e.actor)} />;
          case "scene":
            return (
              <div key={i} className="my-8 flex items-center gap-4 text-[#7a2a1d]">
                <span className="h-px flex-1 bg-[#7a2a1d]/30" />
                <span className="font-display text-sm tracking-[0.2em] uppercase">{e.scene?.title ?? e.text}</span>
                <span className="h-px flex-1 bg-[#7a2a1d]/30" />
              </div>
            );
          case "level_up":
            return <LevelUpCard key={i} who={byId(e.actor)} e={e} />;
          case "system":
            return (
              <div key={i} className="my-2 text-center text-[15px] italic text-sepia/70">
                {e.text}
              </div>
            );
          case "error":
            return (
              <div key={i} className="my-3 rounded border border-blood/40 bg-blood/10 px-3 py-2 text-[15px] text-[#7a2a1d]">
                The weave falters: {e.text}
              </div>
            );
          default:
            return null;
        }
      })}
    </>
  );
});

export function Narrative({ log, base = 0, settleKey, streams, thinking, creatures, heroId }: NarrativeProps) {
  const end = useRef<HTMLDivElement>(null);
  const byId = (id?: string) => creatures.find((c) => c.id === id);
  const dmLive = streams["dm"];
  const streamed = Object.values(streams).join("").length;

  // A new entry scrolls smoothly; streaming text just keeps the end in view
  // (a smooth scroll per chunk piles up animations and stutters).
  useEffect(() => {
    end.current?.scrollIntoView({ behavior: "smooth", block: "end", inline: "nearest" });
  }, [log.length, thinking]);
  useEffect(() => {
    if (streamed) end.current?.scrollIntoView({ block: "end", inline: "nearest" });
  }, [streamed]);

  // The action bar grows when choices appear; keep the end of the story in view without animating.
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end", inline: "nearest" });
  }, [settleKey]);

  const narrated = log.some((e) => e.type === "narration");
  const thinker = thinking && thinking !== "dm" ? byId(thinking) : undefined;

  return (
    <div className="px-5 sm:px-10 py-8 text-[19px] leading-[1.65]">
      {log.length === 0 && !dmLive && !thinking && (
        <p className="text-center italic text-sepia/60">The tale has not yet begun.</p>
      )}
      <History log={log} base={base} creatures={creatures} heroId={heroId} />

      {Object.entries(streams).map(([seat, text]) => {
        if (!text) return null;
        if (seat === "dm") {
          return (
            <div key={seat} className="my-5 text-sepia">
              <Paragraphs text={text} caret first={!narrated} />
            </div>
          );
        }
        return <Speech key={seat} c={byId(seat)} text={text} live />;
      })}

      {thinking && (
        <div className="my-4 flex items-center gap-3 text-[16px] italic text-sepia/70" role="status">
          <span className="inline-flex gap-1">
            {[0, 1, 2].map((d) => (
              <motion.span
                key={d}
                className="h-1.5 w-1.5 rounded-full bg-[#7a2a1d]"
                animate={{ opacity: [0.2, 1, 0.2] }}
                transition={{ duration: 1.2, repeat: Infinity, delay: d * 0.2 }}
              />
            ))}
          </span>
          {thinking === "dm" ? "The Dungeon Master considers…" : `${thinker?.name ?? "A companion"} is deciding…`}
        </div>
      )}
      <div ref={end} />
    </div>
  );
}
