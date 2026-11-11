import { useState } from "react";
import type { Creature } from "../api";
import { useArt } from "../art";

// Class emblems drawn as simple SVG glyphs inside a gilded medallion.
const emblems: Record<string, string> = {
  fighter: "M12 2l2 4-1 1v9l3 2v2h-3l-1 2-1-2H8v-2l3-2V7l-1-1z", // sword
  barbarian: "M6 3c5 0 7 3 7 6l7 11-2 2-7-11C7 11 4 8 6 3z", // axe
  rogue: "M14 2l1 7 2 2-2 1-1 10h-2L11 12 9 11l2-2z", // dagger
  ranger: "M5 3c8 3 11 9 11 18M5 3l14 14M14 17l5 0 0-5", // bow + arrow
  wizard: "M12 2l3 8 5 1-4 4 1 7-5-3-5 3 1-7-4-4 5-1z", // star
  cleric: "M10 2h4v6h6v4h-6v10h-4V12H4V8h6z", // holy symbol
  bard: "M9 18a3 3 0 11-2-3V5l11-2v12a3 3 0 11-2-3V7l-7 1.3z", // note
};

const monsterTint = "#7a2a1d";

export function Portrait({
  c,
  size = 56,
  active = false,
  src,
  painting = false,
}: {
  c: Creature;
  size?: number;
  active?: boolean;
  /** A painted portrait; falls back to the class emblem when absent or broken. */
  src?: string;
  painting?: boolean;
}) {
  const [broken, setBroken] = useState<string | null>(null);
  const gallery = useArt();
  src = src ?? gallery.art[c.id];
  painting = painting || gallery.painting.includes(c.id);
  const key = (c.class ?? c.portrait ?? "").toLowerCase();
  const glyph = emblems[key];
  const monster = c.kind === "monster";
  const down = c.dead || c.hp <= 0;
  return (
    <div
      className="relative shrink-0 rounded-full grid place-items-center overflow-visible transition-shadow"
      style={{
        width: size,
        height: size,
        background: monster
          ? `radial-gradient(circle at 35% 30%, #5a2a22, #1d0d0a)`
          : `radial-gradient(circle at 35% 30%, #3a3022, #15110c)`,
        boxShadow: active
          ? "0 0 0 2px #e6c77a, 0 0 18px rgba(230,199,122,.55)"
          : `0 0 0 2px ${monster ? monsterTint : "#6e5a36"}, inset 0 2px 6px rgba(0,0,0,.6)`,
        filter: down ? "grayscale(1) brightness(.55)" : undefined,
      }}
      aria-hidden
    >
      {src && broken !== src ? (
        <img src={src} alt="" onError={() => setBroken(src)} className="absolute inset-0 h-full w-full rounded-full object-cover" draggable={false} />
      ) : glyph ? (
        <svg viewBox="0 0 24 24" width={size * 0.5} height={size * 0.5} fill={key === "ranger" ? "none" : "#e6c77a"} stroke={key === "ranger" ? "#e6c77a" : "none"} strokeWidth={1.8} strokeLinecap="round">
          <path d={glyph} />
        </svg>
      ) : (
        <span className="font-display font-bold" style={{ fontSize: size * 0.38, color: monster ? "#f0b8a8" : "#e6c77a" }}>
          {c.name.slice(0, 1)}
        </span>
      )}
      {painting && (
        <span className="absolute -inset-[3px] rounded-full border-2 border-gold/70 border-t-transparent animate-spin" title="Painting portrait…" />
      )}
      {c.dead && (
        <span className="absolute inset-0 grid place-items-center font-display text-blood" style={{ fontSize: size * 0.5 }}>
          ✕
        </span>
      )}
    </div>
  );
}
