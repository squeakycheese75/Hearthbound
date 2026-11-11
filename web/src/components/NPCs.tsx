import { AnimatePresence, motion } from "motion/react";
import { useState } from "react";
import type { NPC } from "../api";
import { useArt } from "../art";

const attitudeTone: Record<NPC["attitude"], { dot: string; chip: string }> = {
  friendly: { dot: "#4f9a7f", chip: "bg-verdigris/20 text-[#9fd8c2]" },
  neutral: { dot: "#a99a84", chip: "bg-ink-3 text-mist" },
  wary: { dot: "#d9a03b", chip: "bg-gold/15 text-gold-2" },
  hostile: { dot: "#c4483f", chip: "bg-blood/25 text-[#f0a49b]" },
};

/** An NPC's round portrait: its painting or stock art, else its initial. */
export function NPCMedallion({ n, size = 44 }: { n: NPC; size?: number }) {
  const { art } = useArt();
  const [broken, setBroken] = useState<string | null>(null);
  const src = art[n.id];
  const ring = n.monster ? "#7a2a1d" : "#c9a24a";
  return (
    <span
      className="relative grid shrink-0 place-items-center overflow-hidden rounded-full bg-ink-3"
      style={{ width: size, height: size, boxShadow: `0 0 0 2px ${ring}, 0 4px 10px rgba(0,0,0,.55)` }}
    >
      {src && broken !== src ? (
        <img src={src} alt="" className="h-full w-full object-cover" onError={() => setBroken(src)} />
      ) : (
        <span className="font-display text-bone/80" style={{ fontSize: size * 0.42 }}>
          {n.name.slice(0, 1).toUpperCase()}
        </span>
      )}
      <span
        className="absolute bottom-0 right-0 rounded-full ring-2 ring-ink"
        style={{ width: size * 0.26, height: size * 0.26, background: attitudeTone[n.attitude]?.dot ?? "#a99a84" }}
        title={n.attitude}
      />
    </span>
  );
}

/** Who else is in the scene, shown over the backdrop. */
export function SceneCast({ npcs }: { npcs: NPC[] }) {
  const here = npcs.filter((n) => n.here);
  return (
    <div className="pointer-events-none absolute bottom-3 right-3 flex max-w-[70%] flex-wrap justify-end gap-3" aria-label="Here in this scene">
      <AnimatePresence>
        {here.map((n) => (
          <motion.figure
            key={n.id}
            layout
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 8 }}
            className="flex flex-col items-center"
            title={`${n.name}: ${n.role} (${n.attitude})`}
          >
            <NPCMedallion n={n} size={52} />
            <figcaption className="mt-1 rounded bg-ink/80 px-1.5 text-[12px] leading-5 text-bone ring-1 ring-line/70">{n.name}</figcaption>
          </motion.figure>
        ))}
      </AnimatePresence>
    </div>
  );
}

/** The side panel's "Here" list, plus who the party has met before. */
export function NPCList({ npcs }: { npcs: NPC[] }) {
  const here = npcs.filter((n) => n.here);
  const met = npcs.filter((n) => !n.here);
  if (here.length === 0 && met.length === 0) return null;
  return (
    <>
      {here.length > 0 && (
        <h2 className="mt-3 font-display text-[13px] tracking-[0.25em] uppercase text-gold">Here</h2>
      )}
      {here.map((n) => (
        <div key={n.id} className="flex items-center gap-3 rounded-xl bg-ink-2/70 px-3 py-2.5 gilded">
          <NPCMedallion n={n} size={40} />
          <div className="min-w-0 flex-1">
            <div className="flex items-baseline justify-between gap-2">
              <span className="truncate font-display text-[15px] text-bone">{n.name}</span>
              <span className={`shrink-0 rounded px-1.5 text-[11px] uppercase tracking-wider ${attitudeTone[n.attitude]?.chip ?? ""}`}>
                {n.attitude}
              </span>
            </div>
            <div className="truncate text-[13px] text-mist" title={n.look ?? n.role}>
              {n.role}
              {n.follows ? " · travels with you" : ""}
            </div>
          </div>
        </div>
      ))}
      {met.length > 0 && (
        <p className="text-[13px] text-faint">
          Met earlier: {met.map((n) => n.name).join(", ")}
        </p>
      )}
    </>
  );
}
