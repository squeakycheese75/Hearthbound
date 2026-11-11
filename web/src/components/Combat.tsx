import { Term } from "./Term";
import { AnimatePresence, motion } from "motion/react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Creature, Encounter, Option } from "../api";
import { HpBar } from "./Party";
import { Portrait } from "./Portrait";
import { SceneArt } from "./SceneArt";

/** "Goblin 3" -> "3": the number that tells identical monsters apart. */
export function tokenNumber(c: Creature): string | null {
  if (c.kind !== "monster") return null;
  return c.name.match(/(\d+)\s*$/)?.[1] ?? null;
}

/** A small numbered badge shared by the map, the foes list and the initiative ribbon. */
export function NumberBadge({ c, size = 18 }: { c: Creature; size?: number }) {
  const n = tokenNumber(c);
  if (!n) return null;
  return (
    <span
      className="absolute -right-1 -bottom-1 grid place-items-center rounded-full bg-[#7a2a1d] font-display font-bold text-[#fbe3d8] ring-2 ring-ink"
      style={{ width: size, height: size, fontSize: size * 0.62 }}
    >
      {n}
    </span>
  );
}

export function InitiativeRibbon({
  enc,
  creatures,
  hovered,
  onHover,
}: {
  enc: Encounter;
  creatures: Creature[];
  hovered?: string | null;
  onHover?: (id: string | null) => void;
}) {
  const current = enc.order[enc.turn % enc.order.length];
  return (
    <div className="flex items-center gap-2 overflow-x-auto scroll-thin px-1 py-2" aria-label="Initiative order">
      <span className="shrink-0 font-display text-[12px] tracking-[0.2em] uppercase text-gold pr-1">Round {enc.round}</span>
      {enc.order.map((id) => {
        const c = creatures.find((x) => x.id === id);
        if (!c) return null;
        const on = id === current;
        return (
          <div
            key={id}
            onMouseEnter={() => onHover?.(id)}
            onMouseLeave={() => onHover?.(null)}
            className={`shrink-0 flex items-center gap-1.5 rounded-full pl-0.5 pr-2.5 py-0.5 transition ${on ? "bg-gold/20 ring-1 ring-gold" : hovered === id ? "bg-ink-3 ring-1 ring-bone/40" : "bg-ink-2/80"} ${c.dead ? "opacity-40 line-through" : ""}`}
          >
            <span className="relative">
              <Portrait c={c} size={24} active={on} />
              <NumberBadge c={c} size={13} />
            </span>
            <span className={`text-[13px] ${on ? "text-gold-2" : "text-mist"}`}>{c.name}</span>
            <span className="text-[11px] text-faint tabular-nums">{enc.initiative[id]}</span>
          </div>
        );
      })}
    </div>
  );
}

/** Squares within `steps` moves (5e diagonals) of a position. */
function reachable(pos: { x: number; y: number }, steps: number, w: number, h: number): Set<string> {
  const out = new Set<string>();
  for (let x = Math.max(0, pos.x - steps); x <= Math.min(w - 1, pos.x + steps); x++) {
    for (let y = Math.max(0, pos.y - steps); y <= Math.min(h - 1, pos.y + steps); y++) out.add(`${x},${y}`);
  }
  return out;
}

export function BattleMap({
  enc,
  creatures,
  targets,
  onTarget,
  sceneKey,
  sceneArt,
  battlemap,
  hovered,
  onHover,
  reach,
}: {
  enc: Encounter;
  creatures: Creature[];
  targets: Set<string>;
  onTarget: (id: string) => void;
  sceneKey: string;
  sceneArt?: string;
  /** A top-down map of the scene to use as the floor (stock art). */
  battlemap?: string;
  hovered?: string | null;
  onHover?: (id: string | null) => void;
  /** On the hero's turn: who is moving, and how far they can go. */
  reach?: Creature;
}) {
  const current = enc.order[enc.turn % enc.order.length];
  const board = useRef<HTMLDivElement>(null);
  const [cell, setCell] = useState(48);
  useEffect(() => {
    const el = board.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setCell(el.clientWidth / enc.gridW));
    ro.observe(el);
    return () => ro.disconnect();
  }, [enc.gridW]);
  const token = Math.max(22, Math.min(46, cell * 0.74));
  const range = useMemo(() => (reach ? reachable(reach.pos, Math.floor(reach.speed / 5), enc.gridW, enc.gridH) : new Set<string>()), [reach, enc.gridW, enc.gridH]);
  const cw = 100 / enc.gridW;
  const ch = 100 / enc.gridH;

  return (
    <div className="relative w-full">
      <div
        ref={board}
        className="relative mx-auto w-full overflow-hidden rounded-xl"
        style={{
          aspectRatio: `${enc.gridW} / ${enc.gridH}`,
          maxWidth: `min(900px, calc(46vh * ${enc.gridW} / ${enc.gridH}))`,
          boxShadow: "0 0 0 1px rgba(201,162,74,.45), 0 0 0 5px #120e0b, 0 0 0 6px rgba(201,162,74,.25), 0 18px 40px rgba(0,0,0,.6)",
        }}
      >
        {/* The battlefield floor: a top-down map of the scene, else the scene itself, darkened. */}
        <div className="absolute inset-0">
          {battlemap ? (
            <>
              <img src={battlemap} alt="" className="h-full w-full object-cover" />
              <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(14,12,10,0)_55%,rgba(14,12,10,.55))]" />
            </>
          ) : (
            <>
              {sceneArt ? <img src={sceneArt} alt="" className="h-full w-full object-cover" /> : <SceneArt sceneKey={sceneKey} anchor="bottom" className="h-full w-full" />}
              <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(14,12,10,.35),rgba(14,12,10,.85))]" />
            </>
          )}
        </div>

        {/* Grid and reachable squares. On a map the lines are a fine dark ink with a faint gold edge. */}
        <svg className="absolute inset-0 h-full w-full" viewBox={`0 0 ${enc.gridW} ${enc.gridH}`} preserveAspectRatio="none" aria-hidden>
          {[...range].map((k) => {
            const [x, y] = k.split(",").map(Number);
            return (
              <rect key={k} x={x + 0.06} y={y + 0.06} width={0.88} height={0.88} rx={0.1}
                fill="rgba(230,199,122,.11)" stroke="rgba(244,230,196,.28)" strokeWidth={0.018} />
            );
          })}
          <GridLines w={enc.gridW} h={enc.gridH} stroke={battlemap ? "rgba(18,12,6,.45)" : "rgba(230,199,122,.16)"} width={battlemap ? 0.03 : 0.02} />
          {battlemap && <GridLines w={enc.gridW} h={enc.gridH} stroke="rgba(230,199,122,.12)" width={0.012} />}
        </svg>

        {creatures.map((c) => {
          const targetable = targets.has(c.id);
          const isTurn = c.id === current;
          const lit = hovered === c.id;
          const foe = c.kind === "monster";
          return (
            <motion.button
              key={c.id}
              layout
              type="button"
              disabled={!targetable}
              onClick={() => onTarget(c.id)}
              onMouseEnter={() => onHover?.(c.id)}
              onMouseLeave={() => onHover?.(null)}
              className={`absolute flex flex-col items-center justify-center ${targetable ? "cursor-crosshair" : "cursor-default"} ${c.dead ? "pointer-events-none" : ""}`}
              style={{ width: `${cw}%`, height: `${ch}%`, zIndex: isTurn || lit ? 20 : 10 }}
              animate={{ left: `${c.pos.x * cw}%`, top: `${c.pos.y * ch}%`, opacity: c.dead ? 0.35 : 1 }}
              transition={{ type: "spring", stiffness: 140, damping: 18 }}
              title={`${c.name} — ${c.dead ? "slain" : `${c.hp}/${c.maxHp} HP, AC ${c.ac}`}`}
              aria-label={targetable ? `Target ${c.name}` : c.name}
            >
              {/* grounding shadow */}
              <span className="absolute bottom-[12%] h-[16%] w-[62%] rounded-[50%] bg-black/60 blur-[2px]" />
              <span className="relative">
                {isTurn && (
                  <motion.span
                    className="absolute -inset-2 rounded-full"
                    style={{ boxShadow: "0 0 18px 4px rgba(230,199,122,.6)" }}
                    animate={{ opacity: [0.5, 1, 0.5] }}
                    transition={{ duration: 1.6, repeat: Infinity }}
                  />
                )}
                {targetable && (
                  <motion.span
                    className="absolute -inset-[5px] rounded-full border-2 border-dashed border-[#e0533f]"
                    animate={{ rotate: 360 }}
                    transition={{ duration: 6, repeat: Infinity, ease: "linear" }}
                  />
                )}
                {/* The miniature's base: a metal rim, a dark lip, and a shadow on the floor. */}
                <span
                  className="block rounded-full"
                  style={{
                    boxShadow: `0 0 0 2px ${lit ? "#f4e6c4" : foe ? "#a5302a" : "#c9a24a"}, 0 0 0 3.5px rgba(10,7,4,.85), 0 0 0 4.5px ${foe ? "rgba(165,48,42,.55)" : "rgba(201,162,74,.5)"}, 0 5px 10px rgba(0,0,0,.65)`,
                  }}
                >
                  <Portrait c={c} size={token} active={isTurn} />
                </span>
                <NumberBadge c={c} size={Math.max(14, token * 0.42)} />
                {c.dead && <span className="absolute inset-0 grid place-items-center text-2xl text-[#e0533f]">✕</span>}
              </span>
              {!c.dead && (
                <span className="relative mt-0.5 w-[66%]">
                  <HpBar c={c} thin />
                </span>
              )}
              <AnimatePresence>
                {(lit || cell >= 58) && (
                  <motion.span
                    initial={{ opacity: 0, y: -2 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0 }}
                    className="pointer-events-none absolute top-full mt-0.5 whitespace-nowrap rounded bg-ink/90 px-1.5 text-[11px] leading-4 text-bone ring-1 ring-line"
                  >
                    {c.name}
                  </motion.span>
                )}
              </AnimatePresence>
            </motion.button>
          );
        })}
      </div>
    </div>
  );
}

function GridLines({ w, h, stroke, width }: { w: number; h: number; stroke: string; width: number }) {
  return (
    <>
      {Array.from({ length: w + 1 }, (_, x) => (
        <line key={`v${x}`} x1={x} y1={0} x2={x} y2={h} stroke={stroke} strokeWidth={width} />
      ))}
      {Array.from({ length: h + 1 }, (_, y) => (
        <line key={`h${y}`} x1={0} y1={y} x2={w} y2={y} stroke={stroke} strokeWidth={width} />
      ))}
    </>
  );
}

/** The hero's turn: pick an action, then a target (from the list or the map). */
const targetLabel: Record<string, string> = { bardic_inspiration: "Inspire:", potion_healing: "Who drinks:" };
const friendly = new Set(["bardic_inspiration", "potion_healing"]);

export function ActionPicker({
  options,
  busy,
  onChoose,
  selected,
  setSelected,
}: {
  options: Option[];
  busy: boolean;
  onChoose: (o: Option) => void;
  selected: string | null;
  setSelected: (id: string | null) => void;
}) {
  const groups = useMemo(() => {
    const m = new Map<string, Option[]>();
    for (const o of options) m.set(o.actionId, [...(m.get(o.actionId) ?? []), o]);
    return [...m.entries()];
  }, [options]);
  const [hover, setHover] = useState<string | null>(null);
  const active = groups.find(([id]) => id === selected);

  if (options.length === 0) {
    return <p className="text-mist italic">Waiting for your turn…</p>;
  }
  const endTurn = options.find((o) => o.actionId === "end_turn");
  const rows: { label: string; term: string; cost: string; tone: string }[] = [
    { label: "Action", term: "action", cost: "", tone: "bg-ink-3 text-bone hover:bg-ink-3/70 ring-1 ring-line" },
    { label: "Bonus action", term: "bonus", cost: "bonus", tone: "bg-arcane/15 text-bone hover:bg-arcane/25 ring-1 ring-arcane/50" },
    { label: "Free", term: "free", cost: "free", tone: "bg-gold/10 text-gold-2 hover:bg-gold/20 ring-1 ring-gold/40" },
  ];
  return (
    <div className="space-y-2">
      {rows.map((row) => {
        const here = groups.filter(([id, opts]) => id !== "end_turn" && (opts[0].cost ?? "") === row.cost);
        if (here.length === 0) return null;
        return (
          <div key={row.label} className="flex flex-wrap items-center gap-2">
            <span className="w-28 shrink-0 whitespace-nowrap text-[11px] uppercase tracking-[0.18em] text-faint">
              <Term k={row.term} focusable>{row.label}</Term>
            </span>
            {here.map(([id, opts]) => (
              <button
                key={id}
                disabled={busy}
                title={opts[0].summary}
                onClick={() => (opts.length === 1 && !opts[0].targetId ? onChoose(opts[0]) : setSelected(selected === id ? null : id))}
                onMouseEnter={() => setHover(id)}
                onMouseLeave={() => setHover(null)}
                className={`rounded-lg px-3 py-1.5 font-display text-[14px] tracking-wide transition-colors disabled:opacity-50 ${
                  selected === id ? "bg-gold text-ink" : row.tone
                }`}
              >
                {opts[0].actionName}
                {hover === id && opts.length > 1 && <span className="ml-1 text-[12px] opacity-70">({opts.length})</span>}
              </button>
            ))}
          </div>
        );
      })}
      {endTurn && (
        <div className="flex items-center gap-3">
          <button
            disabled={busy}
            onClick={() => onChoose(endTurn)}
            className="rounded-lg bg-gold px-4 py-1.5 font-display text-[14px] tracking-wide text-ink hover:bg-gold-2 disabled:opacity-50"
          >
            End turn
          </button>
          <span className="text-[13px] text-faint">Your action is spent; use a bonus action or end your turn.</span>
        </div>
      )}
      {active && (
        <div className="flex flex-wrap items-center gap-2 text-[15px]">
          <span className="text-mist">{targetLabel[active[1][0].actionId] ?? "Target:"}</span>
          {active[1].map((o) => (
            <button
              key={o.targetId}
              disabled={busy}
              onClick={() => onChoose(o)}
              className={`rounded-full px-3 py-1 text-bone ring-1 disabled:opacity-50 ${
                friendly.has(o.actionId) ? "bg-verdigris/20 ring-verdigris/60 hover:bg-verdigris/35" : "bg-blood/30 ring-blood/60 hover:bg-blood/50"
              }`}
            >
              {o.actionId === "potion_healing" && o.summary?.startsWith("Drink") ? "Yourself" : o.targetName}
            </button>
          ))}
          <span className="text-faint text-[13px]">or click a glowing token</span>
        </div>
      )}
    </div>
  );
}
