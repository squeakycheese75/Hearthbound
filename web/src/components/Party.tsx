import { ImproveAbilities } from "./Improve";
import type { ImproveFn } from "./Improve";
import { AnimatePresence, motion } from "motion/react";
import { useEffect } from "react";
import type { Ability, Creature } from "../api";
import { mod, signed } from "../api";
import { useArt } from "../art";
import { Backpack } from "./Backpack";
import type { EquipFn } from "./Backpack";
import { Portrait } from "./Portrait";
import { Term } from "./Term";

export function HpBar({ c, thin = false }: { c: Creature; thin?: boolean }) {
  const pct = Math.max(0, Math.min(100, (c.hp / Math.max(1, c.maxHp)) * 100));
  const color = pct > 50 ? "#4f9a7f" : pct > 25 ? "#d9a03b" : "#c4483f";
  return (
    <div className={`w-full rounded-full bg-black/50 overflow-hidden ${thin ? "h-1" : "h-1.5"}`} aria-label={`${c.hp} of ${c.maxHp} hit points`}>
      <motion.div className="h-full rounded-full" style={{ background: color }} animate={{ width: `${pct}%` }} transition={{ type: "spring", stiffness: 120, damping: 20 }} />
    </div>
  );
}

/** Progress towards the next level, from the game's XP table. */
export function XpBar({ c, levels }: { c: Creature; levels: number[] }) {
  const max = c.level >= levels.length;
  const from = levels[c.level - 1] ?? 0;
  const to = levels[c.level] ?? from;
  const pct = max ? 100 : Math.max(0, Math.min(100, ((c.xp - from) / Math.max(1, to - from)) * 100));
  const label = max ? `${c.xp} XP · max level` : `${c.xp}/${to} XP to level ${c.level + 1}`;
  return (
    <div className="mt-1 flex items-center gap-2" title={label}>
      <div className="h-1 w-full overflow-hidden rounded-full bg-black/50" aria-label={label}>
        <motion.div className="h-full rounded-full bg-arcane/80" animate={{ width: `${pct}%` }} transition={{ type: "spring", stiffness: 120, damping: 20 }} />
      </div>
      <span className="shrink-0 text-[11px] tabular-nums text-faint">{max ? "max" : `${c.xp}/${to}`}</span>
    </div>
  );
}

function status(c: Creature): string | null {
  if (c.dead) return "Dead";
  if (c.hp <= 0) return c.stable ? "Stable" : `Dying ${c.deathSaves.successes}✓ ${c.deathSaves.failures}✗`;
  return null;
}

export interface SeatInfo {
  mine: boolean;
  acted: boolean;
  waiting: boolean;
  /** The owner is away and the character's agent is standing in. */
  covering: boolean;
  /** The owner has not let the agent stand in: the character holds back while they are away. */
  hold: boolean;
}

/** Who is at this seat right now, from the viewer's point of view. */
function seatWho(c: Creature, seat?: SeatInfo): string {
  const live = c.controller === "human";
  if (seat?.mine) {
    if (!live) return "Your agent";
    return seat.covering ? "Your agent (while you're away)" : "You";
  }
  if (!live) return c.persona?.archetype ? `Agent · ${c.persona.archetype}` : "Agent";
  return seat?.covering ? "Agent (player away)" : "Another player";
}

function seatState(c: Creature, seat: SeatInfo): { text: string; tone: string } {
  const live = c.controller === "human";
  if (c.dead) return { text: "Fallen", tone: "text-[#f0a49b]" };
  if (seat.acted) return { text: "✓ Acted this round", tone: "text-verdigris" };
  if (seat.waiting) return { text: seat.covering ? "Away · agent steps in at the deadline" : "Waiting to act…", tone: "text-gold-2" };
  if (seat.covering) return { text: "Away · agent covering", tone: "text-[#c3baf5]" };
  return live ? { text: "Playing live", tone: "text-faint" } : { text: "Agent is playing", tone: "text-faint" };
}

export function PartyCard({
  c,
  active,
  onOpen,
  seat,
  onAutopilot,
  onCover,
  onSendHome,
  player,
  seatOffer,
  onGiveBack,
}: {
  c: Creature;
  active: boolean;
  onOpen: () => void;
  seat?: SeatInfo;
  onAutopilot?: (on: boolean) => void;
  onCover?: (allow: boolean) => void;
  /** Set when this character can be sent home now (yours, agent-played, at a tavern). */
  onSendHome?: () => void;
  /** The friend playing this seat, if one is. */
  player?: string;
  /** Set for the leader's agent-played companions: offer the seat to friends. */
  seatOffer?: { open: boolean; toggle: () => void };
  /** Set when the viewer holds this seat for its owner: hand it back. */
  onGiveBack?: () => void;
}) {
  const st = status(c);
  const live = c.controller === "human";
  const who = player && !seat?.mine ? `Played by ${player}` : seatWho(c, seat);
  return (
    <div className={`rounded-xl transition-colors ${active ? "bg-ink-3 ring-1 ring-gold/60" : "bg-ink-2/70 hover:bg-ink-3"} gilded`}>
      <button onClick={onOpen} className="flex w-full items-center gap-3 px-3 pt-2.5 pb-2 text-left">
        <span className="relative shrink-0">
          <Portrait c={c} size={46} active={active} />
          <span
            className={`absolute -bottom-1 -right-1 rounded px-1 text-[10px] font-semibold uppercase tracking-wider ring-1 ring-ink ${
              !live || seat?.covering ? "bg-arcane text-white" : "bg-gold text-ink"
            }`}
            title={!live ? "Played by its agent" : seat?.covering ? "Its agent covers while the player is away" : "Played live by its owner"}
          >
            {!live || seat?.covering ? "AI" : "Live"}
          </span>
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline justify-between gap-2">
            <span className="font-display text-[15px] text-bone truncate">{c.name}</span>
            <span className="text-[13px] text-faint shrink-0 tabular-nums">
              <Term k="AC" /> {c.ac}
            </span>
          </div>
          <div className="text-[13px] text-mist truncate">
            {who} · {c.race} {c.class} {c.level}
          </div>
          <div className="mt-1.5 flex items-center gap-2">
            <HpBar c={c} />
            <span className="text-[12px] tabular-nums text-mist shrink-0">
              {c.hp}/{c.maxHp}
            </span>
          </div>

          {(st || (c.conditions?.length ?? 0) > 0) && (
            <div className="mt-1 flex flex-wrap gap-1">
              {st && (
                <span className="rounded bg-blood/25 px-1.5 text-[11px] uppercase tracking-wider text-[#f0a49b]">
                  <Term k={st.startsWith("Dying") ? "dying" : st}>{st}</Term>
                </span>
              )}
              {c.conditions?.map((x) => (
                <span key={x} className="rounded bg-arcane/20 px-1.5 text-[11px] uppercase tracking-wider text-[#c3baf5]">
                  <Term k={x}>{x}</Term>
                </span>
              ))}
            </div>
          )}
        </div>
      </button>
      {seat && (seat.mine || live) && (
        <div className="flex items-center justify-between gap-2 border-t border-line/50 px-3 py-1.5 text-[12px]">
          <span className={seatState(c, seat).tone}>{seatState(c, seat).text}</span>
          {seat.mine && onAutopilot && !c.dead && (
            <button
              onClick={() => onAutopilot(live)}
              title={
                live
                  ? "Its agent plays this character for you. The story only moves while the party's leader plays one of their characters."
                  : "Play this character yourself. Your other characters go to their agents."
              }
              className="rounded px-1.5 py-0.5 text-gold hover:bg-gold/10 hover:text-gold-2"
            >
              {live ? "Let my agent play" : `Play as ${c.name}`}
            </button>
          )}
        </div>
      )}
      {seatOffer && (
        <div className="flex items-center justify-between gap-2 border-t border-line/50 px-3 py-1.5 text-[12px]">
          <span className={seatOffer.open ? "text-[#9fd8c2]" : "text-faint"}>
            {seatOffer.open ? "Seat open on the tavern board" : "Let a friend play this seat"}
          </span>
          <button
            onClick={seatOffer.toggle}
            title={
              seatOffer.open
                ? "Take the seat off the board."
                : `Post ${c.name}'s seat on the tavern's notice board. A friend who takes it plays ${c.name} live, on their own AI key; ${c.name} stays yours.`
            }
            className="rounded px-1.5 py-0.5 text-gold hover:bg-gold/10 hover:text-gold-2"
          >
            {seatOffer.open ? "Close" : "Offer seat"}
          </button>
        </div>
      )}
      {onGiveBack && (
        <div className="flex items-center justify-between gap-2 border-t border-line/50 px-3 py-1.5 text-[12px]">
          <span className="text-faint">You hold this seat for its owner</span>
          <button
            onClick={onGiveBack}
            title={`Leave the table: ${c.name} goes back to their owner's agent.`}
            className="rounded px-1.5 py-0.5 text-mist hover:bg-ink-3 hover:text-bone"
          >
            Give seat back
          </button>
        </div>
      )}
      {onSendHome && (
        <div className="flex justify-end border-t border-line/50 px-3 py-1 text-[12px]">
          <button
            onClick={onSendHome}
            title={`${c.name} leaves the party and goes back to your roster with what they earned.`}
            className="rounded px-1.5 py-0.5 text-mist hover:bg-ink-3 hover:text-bone"
          >
            Send home
          </button>
        </div>
      )}
      {seat?.mine && live && onCover && !c.dead && (
        <label
          className="flex cursor-pointer items-center justify-between gap-2 border-t border-line/50 px-3 py-1.5 text-[12px] text-mist"
          title="When you miss a deadline, your agent plays this character on your key. Off: the character holds back until you return."
        >
          <span>{seat.hold ? "Holds back while I'm away" : "My agent covers while I'm away"}</span>
          <input type="checkbox" checked={!seat.hold} onChange={(e) => onCover(e.target.checked)} className="accent-[#c9a24a]" />
        </label>
      )}
    </div>
  );
}

const abilities: Ability[] = ["STR", "DEX", "CON", "INT", "WIS", "CHA"];

function SheetPortrait({ c }: { c: Creature }) {
  const { art, painting, enabled, repaint } = useArt();
  const src = art[c.id];
  const busy = painting.includes(c.id);
  if (!enabled && !src) return null;
  return (
    <figure className="relative mt-5 overflow-hidden rounded-xl gilded bg-ink">
      {src ? (
        <img src={src} alt={`Portrait of ${c.name}`} className={`aspect-square w-full object-cover transition-opacity ${busy ? "opacity-50" : ""}`} />
      ) : (
        <div className="grid aspect-square w-full place-items-center text-mist italic">{busy ? "Painting portrait…" : "No portrait yet"}</div>
      )}
      {enabled && repaint && (
        <button
          onClick={() => repaint(c.id)}
          disabled={busy}
          className="absolute bottom-3 right-3 rounded-lg bg-ink/85 px-3 py-1.5 font-display text-[13px] tracking-wider text-gold-2 ring-1 ring-gold/50 backdrop-blur hover:bg-ink disabled:opacity-60"
        >
          {busy ? "Painting…" : "Repaint"}
        </button>
      )}
    </figure>
  );
}

export function CharacterSheet({ c, onClose, onEquip, onImprove }: { c: Creature | null; onClose: () => void; onEquip?: EquipFn; onImprove?: ImproveFn }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <AnimatePresence>
      {c && (
        <motion.div className="fixed inset-0 z-50 flex justify-end bg-black/60" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
          <motion.aside
            role="dialog"
            aria-label={`${c.name} character sheet`}
            className="h-full w-full max-w-md overflow-y-auto scroll-thin bg-ink-2 border-l border-line p-6"
            initial={{ x: 60 }}
            animate={{ x: 0 }}
            exit={{ x: 60 }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-4">
              <Portrait c={c} size={72} />
              <div className="min-w-0">
                <h2 className="font-display text-2xl text-bone">{c.name}</h2>
                <div className="text-mist">
                  Level {c.level} {c.race} {c.class} · {c.xp} XP
                </div>
              </div>
              <button onClick={onClose} className="ml-auto self-start rounded px-2 text-2xl text-mist hover:text-bone" aria-label="Close">
                ×
              </button>
            </div>

            <SheetPortrait c={c} />

            <SheetDetails c={c} onEquip={onEquip} onImprove={onImprove} />
          </motion.aside>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

/** Persona, stats, abilities, actions, spell slots and backpack: shared by the in-game sheet and the character page. */
export function SheetDetails({ c, onEquip, onImprove }: { c: Creature; onEquip?: EquipFn; onImprove?: ImproveFn }) {
  return (
    <>
    {(c.asi ?? 0) > 0 && onImprove && <ImproveAbilities c={c} onImprove={onImprove} />}
    {c.persona && (
      <div className="mt-5 rounded-lg bg-ink/60 gilded p-4 text-[16px]">
        <div className="font-display text-gold text-sm tracking-widest uppercase">{c.persona.archetype}</div>
        <p className="mt-1 text-bone/85">{c.persona.personality}</p>
        {c.persona.backstory && <p className="mt-2 italic text-mist">{c.persona.backstory}</p>}
      </div>
    )}

    <div className="mt-5 grid grid-cols-3 gap-2 text-center">
      {[
        ["HP", `${c.hp}/${c.maxHp}`],
        ["AC", c.ac],
        ["Speed", `${c.speed} ft`],
      ].map(([k, v]) => (
        <div key={k} className="rounded-lg bg-ink/60 gilded py-2">
          <div className="text-[12px] uppercase tracking-widest text-faint">
            <Term k={String(k)} focusable />
          </div>
          <div className="font-display text-xl text-bone tabular-nums">{v}</div>
        </div>
      ))}
    </div>

    <div className="mt-3 grid grid-cols-6 gap-1.5 text-center">
      {abilities.map((a) => (
        <div key={a} className="rounded-lg bg-ink/60 gilded py-2">
          <div className="text-[11px] tracking-widest text-faint">
            <Term k={a} focusable />
          </div>
          <div className="font-display text-lg text-bone">{signed(mod(c.abilities[a]))}</div>
          <div className="text-[12px] text-mist tabular-nums">{c.abilities[a] ?? 10}</div>
        </div>
      ))}
    </div>

    {(c.features ?? []).length > 0 && (
      <>
        <h3 className="mt-6 font-display text-sm tracking-[0.2em] uppercase text-gold">
          <Term k="features" focusable>Class features</Term>
        </h3>
        <ul className="mt-2 space-y-2">
          {c.features!.map((f) => {
            const max = f.max ?? 0;
            const left = max - (f.used ?? 0);
            return (
              <li key={f.key} className="rounded-lg bg-ink/60 gilded px-3 py-2">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-bone">{f.name}</span>
                  {max > 0 && (
                    <span className="flex items-center gap-1" title={`${left} of ${max} left · recharges on a ${f.recharge} rest`}>
                      {Array.from({ length: max }, (_, i) => (
                        <span key={i} className={`h-2.5 w-2.5 rounded-full ${i < left ? "bg-gold" : "border border-gold/50"}`} />
                      ))}
                      <span className="ml-1 text-[12px] text-faint">{f.recharge} rest</span>
                    </span>
                  )}
                </div>
                <div className="text-[14px] text-faint">{f.description}</div>
              </li>
            );
          })}
        </ul>
      </>
    )}
    {c.inspiration ? (
      <p className="mt-3 rounded-lg bg-arcane/15 px-3 py-2 text-[14px] text-bone ring-1 ring-arcane/40">
        Inspired: a d{c.inspiration} will be added to a roll that would otherwise fail.
      </p>
    ) : null}

    <h3 className="mt-6 font-display text-sm tracking-[0.2em] uppercase text-gold">Actions & spells</h3>
    <ul className="mt-2 space-y-2">
      {c.actions.map((a) => (
        <li key={a.id} className="rounded-lg bg-ink/60 gilded px-3 py-2">
          <div className="flex justify-between gap-2">
            <span className="text-bone">{a.name}</span>
            <span className="text-[13px] text-mist tabular-nums">
              {a.kind === "attack" && `${signed(a.toHit ?? 0)} to hit`}
              {a.kind === "save" && `DC ${a.saveDc} ${a.saveAbility}`}
              {a.kind === "heal" && "healing"}
              {a.kind === "auto_hit" && "never misses"}
            </span>
          </div>
          <div className="text-[14px] text-faint">
            {a.description}
            {a.slotLevel ? ` · level ${a.slotLevel} slot` : a.kind !== "attack" || a.slotLevel === 0 ? "" : ""}
            {a.rangeFt > 5 ? ` · ${a.rangeFt} ft` : ""}
          </div>
        </li>
      ))}
    </ul>

    {c.spellSlots && Object.keys(c.spellSlots).length > 0 && (
      <div className="mt-4 flex gap-4 text-[15px] text-mist">
        {Object.entries(c.spellSlots).map(([lvl, max]) => {
          const left = max - (c.slotsUsed?.[lvl] ?? 0);
          return (
            <div key={lvl} className="flex items-center gap-1.5">
              <span>
                <Term k="slots">Level {lvl}</Term>
              </span>
              {Array.from({ length: max }, (_, i) => (
                <span key={i} className={`h-3 w-3 rotate-45 ${i < left ? "bg-arcane" : "border border-arcane/50"}`} />
              ))}
            </div>
          );
        })}
      </div>
    )}

    <h3 className="mt-6 font-display text-sm tracking-[0.2em] uppercase text-gold">Backpack</h3>
    <Backpack items={c.items ?? []} legacy={c.inventory ?? []} gold={c.gold} onEquip={onEquip} />
    </>
  );
}
