import { useState } from "react";
import type { Ability, Creature } from "../api";
import { signed } from "../api";

const abilities: Ability[] = ["STR", "DEX", "CON", "INT", "WIS", "CHA"];

export type ImproveFn = (increases: Partial<Record<Ability, number>>) => Promise<void>;

function mod(score: number) {
  return Math.floor((score - 10) / 2);
}

/**
 * Spend an Ability Score Improvement: two points, +2 to one ability or +1 to
 * two, none above 20. Click an ability to add a point; click again to clear.
 */
export function ImproveAbilities({ c, onImprove }: { c: Creature; onImprove: ImproveFn }) {
  const [inc, setInc] = useState<Partial<Record<Ability, number>>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const spent = Object.values(inc).reduce((a, b) => a + (b ?? 0), 0);
  const base = (a: Ability) => c.baseAbilities?.[a] ?? c.abilities[a] ?? 10;

  const tap = (a: Ability) => {
    const now = inc[a] ?? 0;
    if (spent < 2 && now < 2 && base(a) + now < 20) {
      setInc({ ...inc, [a]: now + 1 });
      return;
    }
    const next = { ...inc };
    delete next[a];
    setInc(next);
  };
  const confirm = async () => {
    setBusy(true);
    setError(null);
    try {
      await onImprove(inc);
      setInc({});
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-4 rounded-lg bg-gold/10 p-3 ring-1 ring-gold/50 text-left">
      <div className="font-display text-[13px] uppercase tracking-[0.18em] text-gold-2">
        Ability Score Improvement{(c.asi ?? 0) > 1 ? ` (${c.asi} to spend)` : ""}
      </div>
      <p className="mt-1 text-[14px] text-mist">Raise one ability by 2, or two abilities by 1 (to at most 20).</p>
      <div className="mt-2 grid grid-cols-6 gap-1.5">
        {abilities.map((a) => {
          const add = inc[a] ?? 0;
          const score = base(a) + add;
          return (
            <button
              key={a}
              onClick={() => tap(a)}
              disabled={busy}
              className={`rounded-lg py-1.5 text-center ring-1 transition ${add ? "bg-gold/25 ring-gold" : "bg-ink/60 ring-line hover:ring-gold/60"}`}
            >
              <div className="text-[11px] tracking-widest text-faint">{a}</div>
              <div className="font-display text-lg text-bone tabular-nums">{score}</div>
              <div className="text-[12px] text-mist">{add ? `+${add}` : signed(mod(score))}</div>
            </button>
          );
        })}
      </div>
      <div className="mt-2 flex items-center gap-3">
        <button
          onClick={() => void confirm()}
          disabled={busy || spent !== 2}
          className="rounded-md bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-40"
        >
          Improve
        </button>
        <span className="text-[13px] text-faint">{2 - spent} point{2 - spent === 1 ? "" : "s"} left</span>
        {error && <span className="text-[13px] text-[#f0a49b]">{error}</span>}
      </div>
    </div>
  );
}
