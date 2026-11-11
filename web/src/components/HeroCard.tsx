import type { Character } from "../api";
import { Portrait } from "./Portrait";

const statusLook: Record<Character["status"], { label: string; cls: string }> = {
  idle: { label: "In town", cls: "bg-verdigris/20 text-[#9fd8c2]" },
  adventuring: { label: "Adventuring", cls: "bg-gold/20 text-gold-2" },
  fallen: { label: "Fallen", cls: "bg-blood/25 text-[#f0a49b]" },
};

/** A roster character as a portrait card. */
export function HeroCard({
  ch,
  selected = false,
  onClick,
  disabled = false,
  badge,
}: {
  ch: Character;
  selected?: boolean;
  onClick?: () => void;
  disabled?: boolean;
  badge?: string;
}) {
  const s = ch.sheet;
  const st = statusLook[ch.status];
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-pressed={selected}
      className={`group relative overflow-hidden rounded-xl text-left transition ${selected ? "ring-2 ring-gold shadow-[0_0_24px_rgba(201,162,74,.35)]" : "ring-1 ring-line hover:ring-gold/60"} bg-ink-2 disabled:cursor-not-allowed disabled:opacity-45`}
    >
      <div className="relative aspect-[4/5] w-full bg-ink">
        {ch.portrait ? (
          <img src={ch.portrait} alt="" className={`h-full w-full object-cover transition-transform duration-500 group-hover:scale-[1.03] ${ch.status === "fallen" ? "grayscale" : ""}`} />
        ) : (
          <div className="grid h-full w-full place-items-center">
            <Portrait c={s} size={88} painting={ch.painting} />
          </div>
        )}
        <div className="absolute inset-0 bg-gradient-to-t from-ink via-ink/10 to-transparent" />
        <span className={`absolute left-2 top-2 rounded-full px-2 py-0.5 text-[11px] uppercase tracking-widest ${st.cls}`}>{st.label}</span>
        {badge && <span className="absolute right-2 top-2 rounded-full bg-gold px-2 py-0.5 font-display text-[11px] tracking-wider text-ink">{badge}</span>}
        {ch.painting && ch.portrait && <span className="absolute right-2 bottom-14 h-4 w-4 animate-spin rounded-full border-2 border-gold border-t-transparent" />}
        <div className="absolute inset-x-0 bottom-0 p-3">
          <div className="font-display text-lg leading-tight text-bone">{s.name}</div>
          <div className="text-[13px] text-mist">
            Level {s.level} {s.race} {s.class}
          </div>
        </div>
      </div>
    </button>
  );
}
