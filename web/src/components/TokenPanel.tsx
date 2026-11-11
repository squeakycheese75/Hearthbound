import { AnimatePresence, motion } from "motion/react";
import { useEffect } from "react";
import type { Creature, Game, SeatUsage, Usage } from "../api";
import { billed, modelName } from "../api";

const helpers: Record<string, { name: string; role: string }> = {
  dm: { name: "Dungeon Master", role: "Narrates and runs the rules" },
  choices: { name: "Quick-choice helper", role: "Writes choices when the DM forgets" },
  recap: { name: "Chapter recaps", role: "DM's private story-so-far notes" },
};

function seatRole(g: Game, c: Creature): string {
  if (c.dead) return "Fallen";
  if (c.controller !== "human") return "Agent-played companion";
  if (g.covering?.[c.id]) return "Agent covering: player away";
  if (g.hold?.[c.id]) return "Played live · holds back when away";
  return "Played live · agent covers when away";
}

interface Row {
  seat: string;
  name: string;
  role: string;
  spend: SeatUsage;
  limit: number;
}

function rows(g: Game): Row[] {
  const spend = g.spend ?? {};
  const party = g.party.map((id) => g.creatures.find((c) => c.id === id)).filter((c): c is Creature => !!c);
  const order = ["dm", ...party.map((c) => c.id), "choices", "recap"];
  const extra = Object.keys(spend).filter((k) => !order.includes(k));
  return [...order, ...extra]
    .filter((seat) => spend[seat])
    .map((seat) => {
      const c = party.find((p) => p.id === seat);
      const known = helpers[seat];
      return {
        seat,
        name: c?.name ?? known?.name ?? seat,
        role: c ? seatRole(g, c) : (known?.role ?? "Agent"),
        spend: spend[seat],
        limit: seat === "dm" ? g.chapterAt.dm : c ? g.chapterAt.companion : 0,
      };
    });
}

const n = (x: number) => x.toLocaleString();
const k = (x: number) => (x >= 10_000 ? `${(x / 1000).toFixed(0)}k` : x >= 1000 ? `${(x / 1000).toFixed(1)}k` : String(x));

function cacheHit(u: Usage): string {
  const prompt = u.input + u.cacheRead + u.cacheWrite;
  return prompt ? `${Math.round((u.cacheRead / prompt) * 100)}%` : "–";
}

function ContextBar({ size, limit }: { size: number; limit: number }) {
  if (!limit) return <span className="text-faint">–</span>;
  const pct = Math.min(100, (size / limit) * 100);
  const color = pct > 85 ? "bg-ember" : pct > 60 ? "bg-gold" : "bg-verdigris";
  return (
    <div className="min-w-[96px]" title={`${n(size)} of ${n(limit)} tokens before the next chapter break`}>
      <div className="flex justify-between text-[12px] tabular-nums text-mist">
        <span>{k(size)}</span>
        <span className="text-faint">/ {k(limit)}</span>
      </div>
      <div className="mt-0.5 h-1 rounded-full bg-black/50">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

/** Per-agent token spend for one adventure: who each agent is, whose key pays, and how big its prompt has grown. */
export function TokenPanel({ game, open, onClose }: { game: Game; open: boolean; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const list = rows(game);
  const total = billed(game.usage);
  const yours = list.filter((r) => r.spend.payer === game.you).reduce((sum, r) => sum + billed(r.spend.total), 0);
  const ledgered = list.reduce((sum, r) => sum + billed(r.spend.total), 0);
  const payer = (id: string) => (id === game.you ? "You" : id === game.leaderId ? "Leader" : "Owner");

  return (
    <AnimatePresence>
      {open && (
        <motion.div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
          <motion.section
            role="dialog"
            aria-label="Token usage by agent"
            className="max-h-full w-full max-w-5xl overflow-y-auto scroll-thin rounded-xl bg-ink-2 p-5 gilded"
            initial={{ y: 16 }}
            animate={{ y: 0 }}
            exit={{ y: 16 }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start gap-4">
              <div>
                <h2 className="font-display text-xl text-gold-2">Tokens by agent</h2>
                <p className="mt-1 text-[14px] text-mist">
                  Each seat is its own agent, on its payer's key and model. The DM and helpers bill the leader; a character's agent bills its owner, on the model they chose.
                </p>
              </div>
              <button onClick={onClose} className="ml-auto rounded px-2 text-2xl text-mist hover:text-bone" aria-label="Close">
                ×
              </button>
            </div>

            <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
              {[
                ["This adventure", `≈${n(total)}`],
                ["On your key", `≈${n(yours)}`],
                ["Chapter", String((game.chapter ?? 0) + 1)],
                ["Model calls", n(list.reduce((sum, r) => sum + r.spend.calls, 0))],
              ].map(([label, value]) => (
                <div key={label} className="rounded-lg bg-ink/60 px-3 py-2">
                  <div className="text-[12px] uppercase tracking-widest text-faint">{label}</div>
                  <div className="font-display text-lg tabular-nums text-bone">{value}</div>
                </div>
              ))}
            </div>

            {list.length === 0 ? (
              <p className="mt-6 italic text-mist">No agent has been called yet.</p>
            ) : (
              <div className="mt-4 overflow-x-auto">
                <table className="w-full text-left text-[14px]">
                  <thead className="text-[12px] uppercase tracking-wider text-faint">
                    <tr className="border-b border-line/60">
                      <th className="py-2 pr-3 font-normal">Agent</th>
                      <th className="py-2 pr-3 font-normal">Model</th>
                      <th className="py-2 pr-3 font-normal">Paid by</th>
                      <th className="py-2 pr-3 text-right font-normal">Calls</th>
                      <th className="py-2 pr-3 font-normal">Prompt now</th>
                      <th className="py-2 pr-3 text-right font-normal" title="Share of prompt tokens served from cache">Cached</th>
                      <th className="py-2 pr-3 text-right font-normal">New in</th>
                      <th className="py-2 pr-3 text-right font-normal" title="Billed at 1.25×">Cache write</th>
                      <th className="py-2 pr-3 text-right font-normal" title="Billed at 0.1×">Cache read</th>
                      <th className="py-2 pr-3 text-right font-normal">Out</th>
                      <th className="py-2 text-right font-normal" title="Weighted the way it is billed">≈ Billed</th>
                    </tr>
                  </thead>
                  <tbody className="tabular-nums">
                    {list.map((r) => {
                      const t = r.spend.total;
                      const last = r.spend.last;
                      return (
                        <tr key={r.seat} className="border-b border-line/30 align-top">
                          <td className="py-2 pr-3">
                            <div className="text-bone">{r.name}</div>
                            <div className="text-[12px] text-faint">{r.role}</div>
                          </td>
                          <td className="py-2 pr-3 whitespace-nowrap text-mist" title={r.spend.model}>
                            {r.spend.model ? modelName(r.spend.model) : "–"}
                          </td>
                          <td className={`py-2 pr-3 ${r.spend.payer === game.you ? "text-gold-2" : "text-mist"}`}>{payer(r.spend.payer)}</td>
                          <td className="py-2 pr-3 text-right text-mist">{n(r.spend.calls)}</td>
                          <td className="py-2 pr-3">
                            <ContextBar size={game.context?.[r.seat] ?? 0} limit={r.limit} />
                          </td>
                          <td className="py-2 pr-3 text-right text-mist" title={`Last call: ${cacheHit(last)} cached`}>
                            {cacheHit(t)}
                          </td>
                          <td className="py-2 pr-3 text-right text-mist">{n(t.input)}</td>
                          <td className="py-2 pr-3 text-right text-mist">{n(t.cacheWrite)}</td>
                          <td className="py-2 pr-3 text-right text-mist">{n(t.cacheRead)}</td>
                          <td className="py-2 pr-3 text-right text-mist" title={`Last call: ${n(last.output)} out`}>
                            {n(t.output)}
                          </td>
                          <td className="py-2 text-right text-bone">{n(billed(t))}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}

            <p className="mt-4 text-[13px] text-faint">
              "Prompt now" is each agent's latest prompt against its chapter limit; at the limit the next quiet moment ends the chapter and every history restarts
              from a recap. Billed weighting: cache reads ×0.1, cache writes ×1.25.
              {ledgered < total && ` About ${n(total - ledgered)} tokens predate the per-agent breakdown.`}
            </p>
          </motion.section>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
