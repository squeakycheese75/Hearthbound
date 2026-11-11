import { useMemo } from "react";

// Hand-built SVG backdrops, one per scene key the DM may choose. They are
// deterministic (seeded) so a scene always looks the same, and stand in until
// licensed illustrations are dropped into /public/scenes/<key>.webp.

type Kind = "outdoor" | "interior" | "underground";

interface Palette {
  sky: [string, string, string];
  far: string;
  mid: string;
  near: string;
  glow?: string;
  kind: Kind;
  celestial?: "moon" | "sun" | "none";
  stars?: boolean;
  fog?: string;
}

const palettes: Record<string, Palette> = {
  tavern: { sky: ["#2a160b", "#4a2512", "#6b3416"], far: "#3a1f10", mid: "#26140a", near: "#140b05", glow: "#ff9b3d", kind: "interior" },
  village: { sky: ["#1b2340", "#4a3c5c", "#c27a4e"], far: "#2c2a3e", mid: "#1d1b28", near: "#100f16", glow: "#ffb35c", kind: "outdoor", celestial: "sun" },
  town: { sky: ["#141b33", "#36385a", "#8a5a4a"], far: "#24243a", mid: "#17172a", near: "#0c0c16", glow: "#ffc06a", kind: "outdoor", celestial: "moon", stars: true },
  road: { sky: ["#1f2a3a", "#5c5a5a", "#c98b54"], far: "#3a3a44", mid: "#252a2a", near: "#121512", kind: "outdoor", celestial: "sun" },
  forest: { sky: ["#0d1a16", "#1f3a2e", "#4f6f50"], far: "#1c3328", mid: "#11231b", near: "#07120d", kind: "outdoor", celestial: "none", fog: "#9fc3a8" },
  swamp: { sky: ["#141c14", "#2f3a24", "#6a6a3a"], far: "#273021", mid: "#182016", near: "#0b100a", kind: "outdoor", celestial: "moon", fog: "#b7c49a" },
  mountains: { sky: ["#101830", "#3b4d73", "#b9a5a0"], far: "#47577a", mid: "#2a3450", near: "#141a2a", kind: "outdoor", celestial: "sun", fog: "#dfe6f2" },
  cave: { sky: ["#07090c", "#121820", "#1e2832"], far: "#151b22", mid: "#0c1015", near: "#050608", glow: "#5fd0c0", kind: "underground" },
  crypt: { sky: ["#08080c", "#16141f", "#252033"], far: "#1b1826", mid: "#110f19", near: "#07060b", glow: "#7cc7a0", kind: "underground" },
  dungeon: { sky: ["#0b0806", "#1e1510", "#33241a"], far: "#241a13", mid: "#170f0a", near: "#0a0705", glow: "#ff8a3d", kind: "underground" },
  castle: { sky: ["#0f1226", "#2e2a4f", "#7b4a5e"], far: "#2a2540", mid: "#1a1729", near: "#0d0b15", glow: "#ffcf7a", kind: "outdoor", celestial: "moon", stars: true },
  camp: { sky: ["#05070f", "#0f1630", "#1d2a4a"], far: "#141b2e", mid: "#0c111d", near: "#05070c", glow: "#ff8f3a", kind: "outdoor", celestial: "moon", stars: true },
  ruins: { sky: ["#1a1a24", "#4a4256", "#a67c6a"], far: "#3a3444", mid: "#24202c", near: "#121016", kind: "outdoor", celestial: "sun", fog: "#cbbcc0" },
  temple: { sky: ["#141018", "#34283a", "#6e5040"], far: "#2e2430", mid: "#1d1720", near: "#0d0a0f", glow: "#ffe1a0", kind: "interior" },
};

function rng(seed: string) {
  let h = 2166136261;
  for (const ch of seed) h = Math.imul(h ^ ch.charCodeAt(0), 16777619);
  return () => {
    h = Math.imul(h ^ (h >>> 15), 2246822507);
    h = Math.imul(h ^ (h >>> 13), 3266489909);
    return ((h ^= h >>> 16) >>> 0) / 4294967296;
  };
}

const W = 1200;
const H = 420;

function ridge(r: () => number, base: number, amp: number, steps: number, jag = 0.5): string {
  let d = `M0 ${H} L0 ${base}`;
  for (let i = 1; i <= steps; i++) {
    const x = (i / steps) * W;
    const y = base - amp * (0.5 + 0.5 * Math.sin(i * 0.9 + r() * 2)) - (r() - 0.5) * amp * jag;
    d += ` L${x.toFixed(0)} ${y.toFixed(0)}`;
  }
  return d + ` L${W} ${H} Z`;
}

function pines(r: () => number, base: number, count: number, size: number): string {
  let d = "";
  for (let i = 0; i < count; i++) {
    const x = r() * W;
    const h = size * (0.6 + r() * 0.8);
    const w = h * 0.32;
    const y = base + r() * 20;
    d += `M${x} ${y - h} L${x - w} ${y} L${x + w} ${y} Z `;
    d += `M${x} ${y - h * 0.75} L${x - w * 1.25} ${y - h * 0.15} L${x + w * 1.25} ${y - h * 0.15} Z `;
  }
  return d;
}

function buildings(r: () => number, base: number, count: number, tall: number, castle = false) {
  const out: { d: string; windows: { x: number; y: number }[] } = { d: "", windows: [] };
  let x = r() * 40;
  for (let i = 0; i < count && x < W; i++) {
    const w = 50 + r() * 70;
    const h = tall * (0.5 + r() * 0.7);
    const top = base - h;
    if (castle && i % 3 === 1) {
      const tw = w * 0.6;
      out.d += `M${x} ${base} L${x} ${top - 60} L${x + tw} ${top - 60} L${x + tw} ${base} Z `;
      out.d += `M${x - 6} ${top - 60} L${x + tw / 2} ${top - 120} L${x + tw + 6} ${top - 60} Z `;
    }
    out.d += `M${x} ${base} L${x} ${top} L${x + w / 2} ${top - w * 0.45} L${x + w} ${top} L${x + w} ${base} Z `;
    if (r() > 0.3) out.windows.push({ x: x + w * (0.3 + r() * 0.3), y: top + 18 + r() * (h * 0.4) });
    x += w + r() * 30;
  }
  return out;
}

function Outdoor({ p, k, r }: { p: Palette; k: string; r: () => number }) {
  const far = ridge(r, k === "mountains" ? 210 : 270, k === "mountains" ? 170 : 70, k === "mountains" ? 9 : 14, k === "mountains" ? 1.2 : 0.4);
  const mid = ridge(r, 320, 60, 18, 0.6);
  const town = k === "village" || k === "town" || k === "castle" ? buildings(r, 340, k === "village" ? 9 : 16, k === "castle" ? 120 : 70, k === "castle") : null;
  const trees = k === "forest" ? pines(r, 330, 70, 150) : k === "swamp" || k === "camp" || k === "road" ? pines(r, 345, 22, 90) : "";
  const nearTrees = k === "forest" ? pines(r, 420, 14, 300) : "";
  return (
    <>
      <path d={far} fill={p.far} />
      {p.fog && <rect y={230} width={W} height={120} fill="url(#fog)" opacity={0.5} />}
      <path d={mid} fill={p.mid} />
      {trees && <path d={trees} fill={p.mid} />}
      {town && (
        <>
          <path d={town.d} fill={p.mid} />
          {town.windows.map((w, i) => (
            <rect key={i} x={w.x} y={w.y} width={7} height={10} fill={p.glow} opacity={0.85} />
          ))}
        </>
      )}
      {k === "ruins" && <Columns r={r} fill={p.mid} broken />}
      {k === "road" && <path d={`M520 ${H} C 560 360, 600 345, 610 330 L 625 330 C 640 345, 700 380, 780 ${H} Z`} fill="#3d3328" opacity={0.8} />}
      {k === "swamp" && <rect y={360} width={W} height={60} fill="#1c2a24" opacity={0.85} />}
      <path d={ridge(r, 380, 30, 22, 0.8)} fill={p.near} />
      {nearTrees && <path d={nearTrees} fill={p.near} />}
      {k === "camp" && <Campfire x={600} y={392} glow={p.glow!} />}
    </>
  );
}

function Columns({ r, fill, broken = false, interior = false }: { r: () => number; fill: string; broken?: boolean; interior?: boolean }) {
  const cols = interior ? [140, 330, 870, 1060] : [180, 300, 440, 760, 900, 1020];
  return (
    <g fill={fill}>
      {cols.map((x, i) => {
        const top = interior ? 40 : broken ? 200 + r() * 90 : 160;
        return (
          <g key={i}>
            <rect x={x - 22} y={top} width={44} height={H - top} />
            <rect x={x - 32} y={top - 10} width={64} height={14} />
          </g>
        );
      })}
      {!broken && !interior && <rect x={150} y={140} width={900} height={22} />}
    </g>
  );
}

function Campfire({ x, y, glow }: { x: number; y: number; glow: string }) {
  return (
    <g>
      <circle cx={x} cy={y - 10} r={120} fill="url(#fire)" />
      <path d={`M${x - 30} ${y} L${x + 30} ${y - 8} M${x - 28} ${y - 8} L${x + 28} ${y}`} stroke="#2a1a10" strokeWidth={8} strokeLinecap="round" />
      <path d={`M${x} ${y - 46} C ${x + 18} ${y - 22}, ${x + 16} ${y - 6}, ${x} ${y - 4} C ${x - 16} ${y - 6}, ${x - 18} ${y - 22}, ${x} ${y - 46} Z`} fill={glow}>
        <animate attributeName="opacity" values="1;0.75;1" dur="0.9s" repeatCount="indefinite" />
      </path>
    </g>
  );
}

function Interior({ p, k, r }: { p: Palette; k: string; r: () => number }) {
  if (k === "temple") {
    return (
      <>
        <rect x={470} y={60} width={260} height={360} fill="url(#window)" opacity={0.6} />
        <Columns r={r} fill={p.mid} interior />
        <rect y={380} width={W} height={40} fill={p.near} />
      </>
    );
  }
  // Tavern: timber beams, a hearth, warm window light.
  return (
    <>
      <rect y={0} width={W} height={34} fill={p.near} />
      {[0, 1, 2, 3, 4].map((i) => (
        <rect key={i} x={i * 300 - 10} y={0} width={26} height={H} fill={p.mid} />
      ))}
      <rect x={80} y={110} width={150} height={120} fill={p.glow} opacity={0.18} />
      <path d="M80 170 H230 M155 110 V230" stroke={p.mid} strokeWidth={8} />
      <g>
        <circle cx={860} cy={330} r={170} fill="url(#fire)" />
        <path d="M760 420 V250 Q860 190 960 250 V420 Z" fill={p.near} />
        <path d="M800 420 V290 Q860 250 920 290 V420 Z" fill="#1a0c05" />
        <Campfire x={860} y={410} glow={p.glow!} />
      </g>
      <path d={`M0 ${H} L0 380 L${W} 380 L${W} ${H} Z`} fill={p.near} />
      <rect x={330} y={330} width={260} height={16} fill={p.mid} />
      <rect x={350} y={346} width={14} height={40} fill={p.mid} />
      <rect x={556} y={346} width={14} height={40} fill={p.mid} />
    </>
  );
}

function Underground({ p, k }: { p: Palette; k: string }) {
  const arch = (x: number, w: number, h: number) => `M${x} ${H} V${H - h + w / 2} A${w / 2} ${w / 2} 0 0 1 ${x + w} ${H - h + w / 2} V${H} Z`;
  return (
    <>
      {k === "cave" ? (
        <>
          <path d={`M0 0 H${W} V120 Q1000 170 900 110 Q760 60 620 140 Q480 200 360 110 Q220 40 120 130 Q60 170 0 120 Z`} fill={p.mid} />
          {[160, 340, 520, 700, 880, 1060].map((x, i) => (
            <path key={i} d={`M${x - 14} 100 L${x} ${180 + (i % 3) * 30} L${x + 14} 100 Z`} fill={p.mid} />
          ))}
          <ellipse cx={600} cy={330} rx={260} ry={90} fill={p.glow} opacity={0.08} />
        </>
      ) : (
        <>
          <path d={arch(380, 440, 360)} fill={p.far} />
          <path d={arch(470, 260, 280)} fill="#040304" />
          <path d={`M0 0 H${W} V60 H0 Z`} fill={p.mid} />
          {[110, 1090].map((x) => (
            <g key={x}>
              <circle cx={x} cy={200} r={90} fill="url(#torch)" />
              <rect x={x - 5} y={200} width={10} height={40} fill="#2a1d12" />
              <path d={`M${x} 172 C ${x + 10} 186, ${x + 9} 196, ${x} 200 C ${x - 9} 196, ${x - 10} 186, ${x} 172 Z`} fill={p.glow}>
                <animate attributeName="opacity" values="1;0.7;1" dur="0.7s" repeatCount="indefinite" />
              </path>
            </g>
          ))}
        </>
      )}
      <path d={`M0 ${H} L0 370 Q300 350 600 372 T${W} 362 L${W} ${H} Z`} fill={p.near} />
    </>
  );
}

export function SceneArt({ sceneKey, className = "", anchor = "middle" }: { sceneKey: string; className?: string; anchor?: "middle" | "bottom" }) {
  const p = palettes[sceneKey] ?? palettes.road;
  const content = useMemo(() => {
    const r = rng(sceneKey);
    if (p.kind === "interior") return <Interior p={p} k={sceneKey} r={r} />;
    if (p.kind === "underground") return <Underground p={p} k={sceneKey} />;
    return <Outdoor p={p} k={sceneKey} r={r} />;
  }, [sceneKey, p]);
  const stars = useMemo(() => {
    if (!p.stars) return [];
    const r = rng(sceneKey + "*");
    return Array.from({ length: 70 }, () => ({ x: r() * W, y: r() * 200, s: r() * 1.4 + 0.3 }));
  }, [sceneKey, p.stars]);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio={anchor === "bottom" ? "xMidYMax slice" : "xMidYMid slice"} className={className} role="img" aria-label={`${sceneKey} scene`}>
      <defs>
        <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={p.sky[0]} />
          <stop offset="0.6" stopColor={p.sky[1]} />
          <stop offset="1" stopColor={p.sky[2]} />
        </linearGradient>
        <radialGradient id="fire">
          <stop offset="0" stopColor={p.glow ?? "#ff9b3d"} stopOpacity="0.55" />
          <stop offset="1" stopColor={p.glow ?? "#ff9b3d"} stopOpacity="0" />
        </radialGradient>
        <radialGradient id="torch">
          <stop offset="0" stopColor={p.glow ?? "#ff9b3d"} stopOpacity="0.4" />
          <stop offset="1" stopColor={p.glow ?? "#ff9b3d"} stopOpacity="0" />
        </radialGradient>
        <linearGradient id="fog" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={p.fog ?? "#fff"} stopOpacity="0" />
          <stop offset="0.5" stopColor={p.fog ?? "#fff"} stopOpacity="0.35" />
          <stop offset="1" stopColor={p.fog ?? "#fff"} stopOpacity="0" />
        </linearGradient>
        <linearGradient id="window" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={p.glow ?? "#ffe1a0"} stopOpacity="0.9" />
          <stop offset="1" stopColor={p.glow ?? "#ffe1a0"} stopOpacity="0" />
        </linearGradient>
        <linearGradient id="shade" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0.55" stopColor="#0e0c0a" stopOpacity="0" />
          <stop offset="1" stopColor="#0e0c0a" stopOpacity="0.95" />
        </linearGradient>
      </defs>
      <rect width={W} height={H} fill="url(#sky)" />
      {stars.map((s, i) => (
        <circle key={i} cx={s.x} cy={s.y} r={s.s} fill="#fff" opacity={0.7} />
      ))}
      {p.celestial === "moon" && <circle cx={960} cy={90} r={34} fill="#f1ead2" opacity={0.9} />}
      {p.celestial === "sun" && <circle cx={880} cy={250} r={70} fill="#ffd9a0" opacity={0.35} />}
      {content}
      <rect width={W} height={H} fill="url(#shade)" />
    </svg>
  );
}
