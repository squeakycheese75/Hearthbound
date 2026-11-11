import { useCallback, useEffect, useMemo, useState } from "react";
import type { Character, Item, Listing, ShopItem } from "../api";
import { api } from "../api";
import { ItemInspector, rarityColour } from "../components/Backpack";
import { Portrait } from "../components/Portrait";
import { navigate } from "../router";

type Tab = "buy" | "mine" | "shop" | "sell";

/** The realm's market: other players' listings, your own, the town shop, and selling. Only heroes in town trade. */
export function Market() {
  const [roster, setRoster] = useState<Character[] | null>(null);
  const [heroId, setHeroId] = useState<string>("");
  const [market, setMarket] = useState<{ open: Listing[]; mine: Listing[]; feePercent: number } | null>(null);
  const [shop, setShop] = useState<{ items: ShopItem[]; buyBackPercent: number } | null>(null);
  const [tab, setTab] = useState<Tab>("buy");
  const [inspect, setInspect] = useState<{ item: Item; listing?: Listing } | null>(null);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const [r, m, s] = await Promise.all([api.roster(), api.market(), api.shop()]);
    setRoster(r);
    setMarket(m);
    setShop(s);
    setHeroId((id) => id || r.find((c) => c.status === "idle")?.id || "");
  }, []);
  useEffect(() => {
    load().catch((e: Error) => setMsg({ ok: false, text: e.message }));
  }, [load]);

  const inTown = useMemo(() => (roster ?? []).filter((c) => c.status === "idle"), [roster]);
  const hero = inTown.find((c) => c.id === heroId);

  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true);
    setMsg(null);
    try {
      await fn();
      setMsg({ ok: true, text: done });
      setInspect(null);
      await load();
    } catch (e) {
      setMsg({ ok: false, text: (e as Error).message });
    } finally {
      setBusy(false);
    }
  };

  if (!roster || !market || !shop) return <div className="grid h-full place-items-center italic text-mist">{msg?.text ?? "Walking to the market square…"}</div>;

  const forSale = market.open.filter((l) => !l.mine);
  const tabs: [Tab, string][] = [
    ["buy", `For sale (${forSale.length})`],
    ["mine", `Your listings (${market.mine.length})`],
    ["shop", "Town shop"],
    ["sell", "Sell"],
  ];

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:py-12">
      <button onClick={() => navigate("/")} className="text-mist hover:text-bone">
        ← Back
      </button>
      <h1 className="mt-4 font-display text-3xl text-gold-2 sm:text-4xl">The market</h1>
      <p className="mt-1 text-mist">
        Trade with other players or the town shop. Only heroes in town can trade; {market.feePercent}% of every sale between players leaves the realm.
      </p>

      <div className="mt-6 flex flex-wrap items-center gap-3 rounded-xl bg-ink-2/80 px-4 py-3 gilded">
        <span className="text-[14px] uppercase tracking-widest text-faint">Trading as</span>
        {inTown.length === 0 ? (
          <span className="italic text-mist">None of your heroes are in town.</span>
        ) : (
          inTown.map((c) => (
            <button
              key={c.id}
              onClick={() => setHeroId(c.id)}
              className={`flex items-center gap-2 rounded-lg px-2 py-1 ${c.id === heroId ? "bg-ink-3 ring-1 ring-gold/60" : "hover:bg-ink-3/60"}`}
            >
              <Portrait c={c.sheet} size={30} src={c.portrait} />
              <span className="text-bone">{c.sheet.name}</span>
              <span className="tabular-nums text-gold-2">{c.sheet.gold} gp</span>
            </button>
          ))
        )}
      </div>

      <div className="mt-5 flex gap-1 border-b border-line/60" role="tablist">
        {tabs.map(([t, label]) => (
          <button
            key={t}
            role="tab"
            aria-selected={tab === t}
            onClick={() => setTab(t)}
            className={`-mb-px border-b-2 px-3 py-2 font-display text-[14px] tracking-wider ${tab === t ? "border-gold text-gold-2" : "border-transparent text-mist hover:text-bone"}`}
          >
            {label}
          </button>
        ))}
      </div>

      {msg && <p className={`mt-4 rounded px-3 py-2 text-[15px] ${msg.ok ? "bg-verdigris/15 text-[#9fd8c2]" : "bg-blood/15 text-[#f0a49b]"}`}>{msg.text}</p>}

      {tab === "buy" && (
        <Grid empty="Nothing for sale right now.">
          {forSale.map((l) => (
            <ListingCard key={l.id} l={l} onOpen={() => setInspect({ item: l.item, listing: l })} />
          ))}
        </Grid>
      )}

      {tab === "mine" && (
        <Grid empty="You have nothing for sale. List items from the Sell tab.">
          {market.mine.map((l) => (
            <div key={l.id} className="rounded-xl bg-ink-2/80 p-3 gilded">
              <ItemLine item={l.item} />
              {l.status === "open" ? (
                <div className="mt-2 flex items-center justify-between">
                  <span className="tabular-nums text-gold-2">{l.price} gp</span>
                  <button disabled={busy || !hero} onClick={() => void act(() => api.listingAction(l.id, "withdraw", heroId), `${l.item.name} is back with ${hero?.sheet.name}.`)} className="text-[14px] text-mist hover:text-bone disabled:opacity-40">
                    Withdraw to {hero?.sheet.name ?? "a hero"}
                  </button>
                </div>
              ) : (
                <div className="mt-2 flex items-center justify-between">
                  <span className="text-[14px] text-verdigris">Sold for {l.price} gp</span>
                  <button disabled={busy || !hero} onClick={() => void act(() => api.listingAction(l.id, "collect", heroId), `${hero?.sheet.name} collects ${l.proceeds} gp.`)} className="rounded-md bg-gold px-3 py-1 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-40">
                    Collect {l.proceeds} gp
                  </button>
                </div>
              )}
            </div>
          ))}
        </Grid>
      )}

      {tab === "shop" && <Shop items={shop.items} hero={hero} busy={busy} act={act} />}

      {tab === "sell" && (
        <Sell hero={hero} buyBack={shop.buyBackPercent} fee={market.feePercent} busy={busy} act={act} onInspect={(item) => setInspect({ item })} />
      )}

      <ItemInspector item={inspect?.item ?? null} onClose={() => setInspect(null)} />
      {inspect?.listing && (
        <div className="fixed inset-x-0 bottom-6 z-[70] flex justify-center px-4">
          <div className="flex items-center gap-3 rounded-xl bg-ink-2 px-4 py-3 shadow-2xl gilded">
            <span className="text-mist">
              {inspect.listing.sellerName ? `${inspect.listing.sellerName}'s` : "A"} listing · <span className="tabular-nums text-gold-2">{inspect.listing.price} gp</span>
            </span>
            <button
              disabled={busy || !hero || (hero?.sheet.gold ?? 0) < inspect.listing.price}
              onClick={() => void act(() => api.listingAction(inspect.listing!.id, "buy", heroId), `${hero?.sheet.name} buys ${inspect.listing!.item.name}.`)}
              className="rounded-md bg-gold px-4 py-1.5 font-display text-ink hover:bg-gold-2 disabled:opacity-40"
              title={!hero ? "Choose a hero in town" : (hero.sheet.gold < inspect.listing.price ? `${hero.sheet.name} can't afford it` : undefined)}
            >
              Buy for {hero?.sheet.name ?? "…"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Grid({ empty, children }: { empty: string; children: React.ReactNode[] }) {
  if (children.length === 0) return <p className="mt-6 italic text-faint">{empty}</p>;
  return <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{children}</div>;
}

function ItemLine({ item }: { item: Item }) {
  const tint = rarityColour[item.rarity] ?? rarityColour.common;
  return (
    <div className="min-w-0">
      <div className="truncate font-display text-[16px]" style={{ color: item.rarity === "common" ? undefined : tint }}>
        <span className={item.rarity === "common" ? "text-bone" : ""}>{item.name}</span>
        {(item.qty ?? 0) > 1 ? ` ×${item.qty}` : ""}
      </div>
      <div className="text-[12px] uppercase tracking-wider text-faint">
        {item.rarity} {item.kind}
      </div>
    </div>
  );
}

function ListingCard({ l, onOpen }: { l: Listing; onOpen: () => void }) {
  return (
    <button onClick={onOpen} className="rounded-xl bg-ink-2/80 p-3 text-left gilded hover:bg-ink-3" title="Inspect and buy">
      <ItemLine item={l.item} />
      <div className="mt-2 flex items-baseline justify-between">
        <span className="tabular-nums text-gold-2">{l.price} gp</span>
        <span className="text-[13px] text-faint">{l.sellerName ? `from ${l.sellerName}` : ""}</span>
      </div>
    </button>
  );
}

type Act = (fn: () => Promise<unknown>, done: string) => Promise<void>;

function Shop({ items, hero, busy, act }: { items: ShopItem[]; hero?: Character; busy: boolean; act: Act }) {
  const [qty, setQty] = useState<Record<string, number>>({});
  const kinds: [ShopItem["kind"], string][] = [
    ["weapon", "Weapons"],
    ["armour", "Armour"],
    ["shield", "Shields"],
    ["potion", "Potions"],
    ["gear", "Gear"],
    ["pack", "Packs"],
    ["trinket", "Trinkets"],
  ];
  return (
    <div className="mt-4 space-y-6">
      {kinds.map(([kind, label]) => {
        const list = items.filter((i) => i.kind === kind);
        if (list.length === 0) return null;
        return (
          <section key={kind}>
            <h3 className="font-display text-[13px] uppercase tracking-[0.2em] text-gold">{label}</h3>
            <ul className="mt-2 divide-y divide-line/40 rounded-xl bg-ink-2/80 gilded">
              {list.map((i) => {
                const n = i.stack ? qty[i.key] ?? 1 : 1;
                return (
                  <li key={i.key} className="flex flex-wrap items-center gap-3 px-4 py-2">
                    <div className="min-w-0 flex-1">
                      <div className="text-bone">{i.name}</div>
                      <div className="truncate text-[13px] text-faint">{[i.damage, i.ac ? `AC ${i.kind === "shield" ? "+" : ""}${i.ac}` : "", i.desc].filter(Boolean).join(" · ")}</div>
                    </div>
                    {i.stack && (
                      <input
                        type="number"
                        min={1}
                        max={20}
                        value={n}
                        onChange={(e) => setQty({ ...qty, [i.key]: Math.max(1, Math.min(20, Number(e.target.value) || 1)) })}
                        className="w-16 rounded bg-ink px-2 py-1 text-bone tabular-nums"
                        aria-label={`How many ${i.name}`}
                      />
                    )}
                    <span className="w-16 text-right tabular-nums text-gold-2">{i.price * n} gp</span>
                    <button
                      disabled={busy || !hero || hero.sheet.gold < i.price * n}
                      onClick={() => void act(() => api.shopBuy(hero!.id, i.key, n), `${hero!.sheet.name} buys ${i.name}${n > 1 ? ` ×${n}` : ""}.`)}
                      className="rounded-md bg-gold px-3 py-1 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-40"
                    >
                      Buy
                    </button>
                  </li>
                );
              })}
            </ul>
          </section>
        );
      })}
    </div>
  );
}

function Sell({ hero, buyBack, fee, busy, act, onInspect }: { hero?: Character; buyBack: number; fee: number; busy: boolean; act: Act; onInspect: (it: Item) => void }) {
  const [price, setPrice] = useState<Record<string, string>>({});
  if (!hero) return <p className="mt-6 italic text-faint">Choose a hero in town to sell from.</p>;
  const items = (hero.sheet.items ?? []).filter((it) => !it.slot && it.kind !== "keepsake");
  if (items.length === 0) return <p className="mt-6 italic text-faint">{hero.sheet.name} has nothing to sell (equipped items and keepsakes can't be sold).</p>;
  return (
    <ul className="mt-4 divide-y divide-line/40 rounded-xl bg-ink-2/80 gilded">
      {items.map((it) => {
        const shopPays = Math.floor((it.value * Math.max(it.qty ?? 1, 1) * buyBack) / 100);
        const ask = Number(price[it.id] ?? it.value);
        return (
          <li key={it.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5">
            <button onClick={() => onInspect(it)} className="min-w-0 flex-1 text-left" title="Inspect">
              <ItemLine item={it} />
            </button>
            <button
              disabled={busy}
              onClick={() => void act(() => api.shopSell(hero.id, it.id), `The shop pays ${shopPays} gp for ${it.name}.`)}
              className="rounded-md px-3 py-1 text-[14px] text-mist ring-1 ring-line hover:text-bone disabled:opacity-40"
            >
              Sell to shop: {shopPays} gp
            </button>
            <span className="flex items-center gap-1.5">
              <input
                type="number"
                min={1}
                value={price[it.id] ?? String(it.value)}
                onChange={(e) => setPrice({ ...price, [it.id]: e.target.value })}
                className="w-20 rounded bg-ink px-2 py-1 text-bone tabular-nums"
                aria-label={`Asking price for ${it.name}`}
              />
              <span className="text-[13px] text-faint">gp</span>
              <button
                disabled={busy || !(ask >= 1)}
                onClick={() => void act(() => api.listItem(hero.id, it.id, ask), `${it.name} is for sale at ${ask} gp (you'll get ${ask - Math.ceil((ask * fee) / 100)} gp after the fee).`)}
                className="rounded-md bg-gold px-3 py-1 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-40"
              >
                List at market
              </button>
            </span>
          </li>
        );
      })}
    </ul>
  );
}
