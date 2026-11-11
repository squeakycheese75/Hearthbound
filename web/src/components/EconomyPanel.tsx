import { useCallback, useEffect, useState } from "react";
import type { Economy, EconomyReport, GoldFlows } from "../api";
import { api } from "../api";
import { rarityColour } from "./Backpack";

const reasonLabel: Record<string, string> = {
  loot: "Loot from the DM",
  starting: "New heroes' purses",
  shop_buyback: "Shop buying items",
  dm_charge: "In-story purchases",
  shop_sale: "Shop selling items",
  market_fee: "Market fees",
  market_sale: "Player sales (transfers)",
  market_payout: "Proceeds collected (transfers)",
};
const faucets = ["loot", "starting", "shop_buyback"];
const sinks = ["dm_charge", "shop_sale", "market_fee"];

/** The realm owner's view of the money supply, its flows, and the controls. */
export function EconomyPanel() {
  const [report, setReport] = useState<EconomyReport | null>(null);
  const [form, setForm] = useState<Economy | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const load = useCallback(async () => {
    const r = await api.economy();
    setReport(r);
    setForm(r.economy);
  }, []);
  useEffect(() => {
    load().catch((e: Error) => setMsg(e.message));
  }, [load]);

  if (!report || !form) return <p className="mt-3 italic text-mist">{msg ?? "Counting the coin…"}</p>;
  const save = async () => {
    setMsg(null);
    try {
      await api.setEconomy(form);
      setMsg("Saved. New rates apply from the next trade or turn.");
      await load();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  const field = (key: keyof Economy, label: string, note: string, maxValue: number) => (
    <label className="block">
      <span className="flex items-baseline justify-between gap-3">
        <span className="text-bone">{label}</span>
        <span className="flex items-center gap-1">
          <input
            type="number"
            min={0}
            max={maxValue}
            value={form[key]}
            onChange={(e) => setForm({ ...form, [key]: Number(e.target.value) })}
            className="w-20 rounded bg-ink px-2 py-1 text-right text-bone tabular-nums outline-none focus:ring-1 focus:ring-gold"
          />
          <span className="text-faint">%</span>
        </span>
      </span>
      <span className="block text-[13px] text-faint">{note}</span>
    </label>
  );

  return (
    <div className="mt-3 space-y-4">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Stat label="Gold in the realm" value={`${report.supply.toLocaleString()} gp`} />
        <Stat label="Living heroes" value={String(report.heroes)} />
        <Stat label="Net, last 24h" value={signed(report.day.net)} tone={report.day.net} />
        <Stat label="Net, last 7 days" value={signed(report.week.net)} tone={report.week.net} />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <FlowTable title="Last 24 hours" f={report.day} />
        <FlowTable title="Last 7 days" f={report.week} />
      </div>

      <div className="rounded-xl bg-ink-2/80 p-4 gilded">
        <h3 className="font-display text-[13px] uppercase tracking-[0.2em] text-gold">Controls</h3>
        <div className="mt-3 space-y-3">
          {field("lootPercent", "Loot rate", "Scales the gold the DM may hand out per turn (100% = the standard cap by level).", 500)}
          {field("marketFeePercent", "Market fee", "The cut of every sale between players that leaves the realm.", 50)}
          {field("shopBuyBackPercent", "Shop buy-back", "What the town shop pays for an item, as a share of its value.", 100)}
        </div>
        <button onClick={() => void save()} className="mt-4 rounded-lg bg-gold px-4 py-2 font-display text-ink hover:bg-gold-2">
          Save
        </button>
        {msg && <span className="ml-3 text-[14px] text-mist">{msg}</span>}
      </div>

      <div className="rounded-xl bg-ink-2/80 p-4 gilded">
        <h3 className="font-display text-[13px] uppercase tracking-[0.2em] text-gold">Market sales, last 7 days</h3>
        {report.sales.length === 0 ? (
          <p className="mt-2 italic text-faint">No sales yet.</p>
        ) : (
          <ul className="mt-2 divide-y divide-line/40">
            {report.sales.map((s, i) => (
              <li key={i} className="flex justify-between gap-3 py-1.5 text-[15px]">
                <span style={{ color: s.rarity === "common" ? undefined : rarityColour[s.rarity] }} className={s.rarity === "common" ? "text-bone" : ""}>
                  {s.item}
                </span>
                <span className="tabular-nums text-gold-2">{s.price} gp</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function signed(n: number): string {
  return `${n > 0 ? "+" : ""}${n.toLocaleString()} gp`;
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: number }) {
  const colour = tone === undefined || tone === 0 ? "text-bone" : tone > 0 ? "text-[#9fd8c2]" : "text-[#f0a49b]";
  return (
    <div className="rounded-lg bg-ink/60 px-3 py-2 gilded">
      <div className="text-[12px] uppercase tracking-widest text-faint">{label}</div>
      <div className={`font-display text-lg tabular-nums ${colour}`}>{value}</div>
    </div>
  );
}

function FlowTable({ title, f }: { title: string; f: GoldFlows }) {
  const rows = (keys: string[]) => keys.filter((k) => f.by[k]).map((k) => [k, f.by[k]] as const);
  return (
    <div className="rounded-xl bg-ink-2/80 p-4 gilded">
      <h3 className="font-display text-[13px] uppercase tracking-[0.2em] text-gold">{title}</h3>
      <dl className="mt-2 space-y-1 text-[14px]">
        <div className="flex justify-between text-[#9fd8c2]">
          <dt>Into the realm</dt>
          <dd className="tabular-nums">+{f.in.toLocaleString()}</dd>
        </div>
        {rows(faucets).map(([k, v]) => (
          <div key={k} className="flex justify-between pl-3 text-faint">
            <dt>{reasonLabel[k]}</dt>
            <dd className="tabular-nums">{v.toLocaleString()}</dd>
          </div>
        ))}
        <div className="flex justify-between text-[#f0a49b]">
          <dt>Out of the realm</dt>
          <dd className="tabular-nums">−{f.out.toLocaleString()}</dd>
        </div>
        {rows(sinks).map(([k, v]) => (
          <div key={k} className="flex justify-between pl-3 text-faint">
            <dt>{reasonLabel[k]}</dt>
            <dd className="tabular-nums">{v.toLocaleString()}</dd>
          </div>
        ))}
        {(f.by.market_sale ?? 0) > 0 && (
          <div className="flex justify-between pt-1 text-faint">
            <dt>Traded between players</dt>
            <dd className="tabular-nums">{f.by.market_sale.toLocaleString()}</dd>
          </div>
        )}
      </dl>
    </div>
  );
}
