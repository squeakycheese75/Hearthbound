import { AnimatePresence, motion } from "motion/react";
import { useEffect, useState } from "react";
import type { Affix, Item, Rarity, Slot } from "../api";

// A character's gear: what's equipped, by slot, then the rest of the pack.
// Items are coloured by rarity; click one to inspect (and equip) it.

const looks: Record<Item["kind"], { label: string; path: string }> = {
  weapon: { label: "Weapon", path: "M14.5 3l6.5.5.5 6.5-9.5 9.5-2-2-3.5 3.5-2-2 3.5-3.5-2-2z" },
  armour: { label: "Armour", path: "M7 3h10l3 4-2 2v12H6V9L4 7z" },
  shield: { label: "Shield", path: "M12 2l8 3v6c0 5-3.5 9-8 11-4.5-2-8-6-8-11V5z" },
  trinket: { label: "Trinket", path: "M12 3a5 5 0 1 1 0 10 5 5 0 0 1 0-10zm0 10v8m-3-3h6" },
  potion: { label: "Potion", path: "M9 2h6v2h-1v4.2l4.7 8.2A3 3 0 0 1 16.1 21H7.9a3 3 0 0 1-2.6-4.6L10 8.2V4H9z" },
  gear: { label: "Gear", path: "M4 8h16v12H4zM8 8V5a4 4 0 0 1 8 0v3M4 12h16" },
  pack: { label: "Pack", path: "M6 7h12l1 14H5zM9 7V4h6v3M8 12h8" },
  keepsake: { label: "Keepsake", path: "M6 2h9l5 5v13a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2zm8 1v5h5" },
};

/** Rarity colours, from plain to legendary. */
export const rarityColour: Record<Rarity, string> = {
  common: "#cbbfa8",
  uncommon: "#5fb98f",
  rare: "#6f97e6",
  "very rare": "#b07de8",
  legendary: "#e8963a",
};

export const slotLabel: Record<Slot, string> = {
  main: "Main hand",
  off: "Off hand",
  ranged: "Ranged",
  armour: "Armour",
  trinket1: "Trinket",
  trinket2: "Trinket",
};

const slotOrder: Slot[] = ["main", "off", "ranged", "armour", "trinket1", "trinket2"];
const kindOrder: Item["kind"][] = ["weapon", "armour", "shield", "trinket", "potion", "keepsake", "gear", "pack"];

/** The slots an item of this kind might go in (the server has the final say). */
function slotsFor(it: Item, worn: Item[]): Slot[] {
  switch (it.kind) {
    case "weapon":
      return ["main", "ranged", "off"];
    case "armour":
      return ["armour"];
    case "shield":
      return ["off"];
    case "trinket":
      // Fill an empty trinket slot first; otherwise replace the first.
      return [worn.some((w) => w.slot === "trinket1") && !worn.some((w) => w.slot === "trinket2") ? "trinket2" : "trinket1"];
  }
  return [];
}

/** Changes what's equipped: put an item in a slot, or "" to take it off. */
export type EquipFn = (item: Item, slot: Slot | "") => Promise<void>;

export function affixText(a: Affix): string {
  const sign = a.value >= 0 ? "+" : "";
  const what: Record<string, string> = { to_hit: "to hit", damage: "damage", ac: "AC", max_hp: "max HP" };
  return `${sign}${a.value} ${what[a.stat] ?? a.stat}`;
}

/** Old free-text items (before the server converted them) shown as keepsakes. */
function fromText(text: string, i: number): Item {
  return { id: `text-${i}`, name: text, kind: "keepsake", rarity: "common", value: 0, weight: 0 };
}

export function Backpack({ items, legacy = [], gold, onEquip }: { items: Item[]; legacy?: string[]; gold: number; onEquip?: EquipFn }) {
  const [openId, setOpenId] = useState<string | null>(null);
  const worn = slotOrder.map((slot) => items.find((it) => it.slot === slot)).filter((it): it is Item => !!it);
  const pack = [...items.filter((it) => !it.slot), ...legacy.map(fromText)].sort((a, b) => kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind));
  const open = [...items, ...pack].find((it) => it.id === openId) ?? null;
  return (
    <div className="mt-3">
      <div className="flex items-center gap-3 rounded-xl bg-gradient-to-r from-gold/15 to-transparent px-4 py-3 ring-1 ring-gold/30">
        <svg viewBox="0 0 24 24" className="h-8 w-8" aria-hidden>
          <path d="M7 6c0-2 2-3 5-3s5 1 5 3l-1 1c3 2 5 5 5 9 0 3-3 5-9 5s-9-2-9-5c0-4 2-7 5-9z" fill="#c9a24a" />
          <path d="M8 8h8" stroke="#6e5a36" strokeWidth="1.5" />
        </svg>
        <div>
          <div className="font-display text-2xl text-gold-2 tabular-nums">{gold}</div>
          <div className="text-[12px] uppercase tracking-widest text-faint">gold pieces</div>
        </div>
      </div>
      {worn.length > 0 && (
        <>
          <h4 className="mt-4 font-display text-[12px] uppercase tracking-[0.2em] text-gold">Equipped</h4>
          <ul className="mt-2 grid grid-cols-2 gap-2">
            {worn.map((it) => (
              <ItemTile key={it.id} it={it} caption={slotLabel[it.slot!]} onOpen={() => setOpenId(it.id)} />
            ))}
          </ul>
          <h4 className="mt-4 font-display text-[12px] uppercase tracking-[0.2em] text-gold">Pack</h4>
        </>
      )}
      {pack.length === 0 ? (
        <p className="mt-3 italic text-faint">The pack is empty.</p>
      ) : (
        <ul className="mt-2 grid grid-cols-2 gap-2">
          {pack.map((it) => (
            <ItemTile key={it.id} it={it} onOpen={() => setOpenId(it.id)} />
          ))}
        </ul>
      )}
      <ItemInspector item={open} worn={worn} onClose={() => setOpenId(null)} onEquip={onEquip} />
    </div>
  );
}

function ItemTile({ it, caption, onOpen }: { it: Item; caption?: string; onOpen: () => void }) {
  const look = looks[it.kind] ?? looks.gear;
  const tint = rarityColour[it.rarity] ?? rarityColour.common;
  const plain = it.rarity === "common";
  return (
    <li>
      <button
        onClick={onOpen}
        className="relative flex w-full items-center gap-2.5 rounded-lg bg-ink/70 px-2.5 py-2 text-left transition hover:bg-ink-3"
        style={{ boxShadow: `inset 0 0 0 1px ${tint}${plain ? "33" : "88"}` }}
        title={`Inspect ${it.name}`}
      >
        <span className="grid h-9 w-9 shrink-0 place-items-center rounded-md" style={{ background: `${tint}1f`, boxShadow: `inset 0 0 0 1px ${tint}55` }}>
          <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke={tint} strokeWidth="1.6" strokeLinejoin="round" strokeLinecap="round" aria-hidden>
            <path d={look.path} />
          </svg>
        </span>
        <span className="min-w-0">
          <span className={`block truncate text-[15px] leading-tight ${plain ? "text-bone" : ""}`} style={{ color: plain ? undefined : tint }}>
            {it.name}
          </span>
          <span className="block text-[11px] uppercase tracking-wider text-faint">{caption ?? `${plain ? "" : `${it.rarity} `}${look.label}`}</span>
        </span>
        {(it.qty ?? 0) > 1 && (
          <span className="absolute -right-1.5 -top-1.5 grid h-6 min-w-6 place-items-center rounded-full bg-gold px-1 font-display text-[12px] text-ink tabular-nums">
            {it.qty}
          </span>
        )}
      </button>
    </li>
  );
}

/** Everything about one item: kind, rarity, stats, affixes, value and where it's been. */
export function ItemInspector({ item, worn = [], onClose, onEquip }: { item: Item | null; worn?: Item[]; onClose: () => void; onEquip?: EquipFn }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => setError(null), [item?.id, item?.slot]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  const equip = async (slot: Slot | "") => {
    if (!item || !onEquip) return;
    setBusy(true);
    setError(null);
    try {
      await onEquip(item, slot);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const slots = item ? slotsFor(item, worn) : [];
  return (
    <AnimatePresence>
      {item && (
        <motion.div className="fixed inset-0 z-[60] grid place-items-center bg-black/60 p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
          <motion.section
            role="dialog"
            aria-label={item.name}
            className="w-full max-w-sm rounded-xl bg-ink-2 p-5 gilded"
            style={{ boxShadow: `0 0 0 1px ${rarityColour[item.rarity]}66, 0 20px 50px rgba(0,0,0,.6)` }}
            initial={{ y: 12, scale: 0.98 }}
            animate={{ y: 0, scale: 1 }}
            exit={{ y: 12 }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start gap-3">
              <div className="min-w-0">
                <h3 className="font-display text-xl" style={{ color: rarityColour[item.rarity] }}>
                  {item.name}
                  {(item.qty ?? 0) > 1 ? ` ×${item.qty}` : ""}
                </h3>
                <p className="text-[13px] uppercase tracking-wider text-faint">
                  {item.rarity} {looks[item.kind]?.label ?? item.kind}
                  {item.slot ? ` · ${slotLabel[item.slot]}` : ""}
                </p>
              </div>
              <button onClick={onClose} className="ml-auto rounded px-2 text-2xl text-mist hover:text-bone" aria-label="Close">
                ×
              </button>
            </div>

            {(item.damage || item.ac || (item.props?.length ?? 0) > 0) && (
              <dl className="mt-3 space-y-1 text-[15px]">
                {item.damage && <Row k="Damage" v={item.damage} />}
                {item.ac ? <Row k={item.kind === "shield" ? "Shield" : "Armour class"} v={item.kind === "shield" ? `+${item.ac} AC` : `${item.ac}`} /> : null}
                {(item.props?.length ?? 0) > 0 && <Row k="Properties" v={item.props!.join(", ")} />}
              </dl>
            )}
            {(item.affixes?.length ?? 0) > 0 && (
              <ul className="mt-3 space-y-0.5">
                {item.affixes!.map((a) => (
                  <li key={a.key} className="text-[15px] text-[#8fb3ff]">
                    {affixText(a)} <span className="text-[13px] text-faint">({a.name})</span>
                  </li>
                ))}
              </ul>
            )}
            {item.desc && <p className="mt-3 text-[15px] italic text-mist">{item.desc}</p>}
            {item.kind !== "keepsake" ? (
              <div className="mt-3 flex gap-4 text-[14px] text-mist">
                <span>
                  Value <span className="tabular-nums text-gold-2">{item.value} gp</span>
                </span>
                {item.weight > 0 && <span className="tabular-nums">{item.weight} lb</span>}
              </div>
            ) : (
              <p className="mt-3 text-[14px] text-faint">A keepsake: it matters to the story, not to the rules.</p>
            )}
            {(item.history?.length ?? 0) > 0 && (
              <>
                <h4 className="mt-4 font-display text-[12px] uppercase tracking-[0.2em] text-gold">History</h4>
                <ol className="mt-1 space-y-0.5 text-[13px] text-mist">
                  {item.history!.map((h, i) => (
                    <li key={i}>{h}</li>
                  ))}
                </ol>
              </>
            )}
            {onEquip && (item.slot || slots.length > 0) && (
              <div className="mt-4 flex flex-wrap gap-2">
                {item.slot ? (
                  <button onClick={() => void equip("")} disabled={busy} className="rounded-md px-3 py-1.5 text-[14px] text-mist ring-1 ring-line hover:text-bone disabled:opacity-50">
                    Take off
                  </button>
                ) : (
                  slots.map((slot) => (
                    <button key={slot} onClick={() => void equip(slot)} disabled={busy} className="rounded-md bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-50">
                      Equip: {slotLabel[slot]}
                    </button>
                  ))
                )}
              </div>
            )}
            {error && <p className="mt-2 text-[13px] text-[#f0a49b]">{error}</p>}
            <p className="mt-4 text-[12px] text-faint">Trading comes next.</p>
          </motion.section>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-faint">{k}</dt>
      <dd className="text-right text-bone">{v}</dd>
    </div>
  );
}
