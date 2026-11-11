import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useEffect, useRef, useState } from "react";
import type { Roll } from "../api";
import { signed } from "../api";

function D20Face({ value, tone, shape = "d20" }: { value: number | string; tone: string; shape?: "d20" | "damage" }) {
  if (shape === "damage") {
    return (
      <svg viewBox="0 0 100 100" className="w-full h-full drop-shadow-[0_8px_18px_rgba(0,0,0,.6)]">
        <polygon points="50,4 94,50 50,96 6,50" fill="#2a1410" stroke={tone} strokeWidth="2.5" />
        <path d="M6 50 H94 M50 4 30 50 50 96 70 50 50 4" stroke={tone} strokeOpacity=".4" strokeWidth="1.4" fill="none" />
        <text x="50" y="58" textAnchor="middle" fontFamily="Cinzel, serif" fontWeight="700" fontSize="22" fill="#f4e6c4">
          {value}
        </text>
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 100 100" className="w-full h-full drop-shadow-[0_8px_18px_rgba(0,0,0,.6)]">
      <defs>
        <linearGradient id="die" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#3b1916" />
          <stop offset="1" stopColor="#140807" />
        </linearGradient>
      </defs>
      <polygon points="50,3 93,27 93,73 50,97 7,73 7,27" fill="url(#die)" stroke={tone} strokeWidth="2.5" />
      <g stroke={tone} strokeOpacity=".45" strokeWidth="1.4" fill="none">
        <polygon points="50,22 76,66 24,66" />
        <path d="M50 3 50 22 M93 27 76 66 M7 27 24 66 M50 97 76 66 M50 97 24 66 M93 73 76 66 M7 73 24 66 M93 27 50 22 M7 27 50 22" />
      </g>
      <text x="50" y="57" textAnchor="middle" fontFamily="Cinzel, serif" fontWeight="700" fontSize="22" fill="#f4e6c4">
        {value}
      </text>
    </svg>
  );
}

function isD20(r: Roll) {
  return r.dice.startsWith("1d20") || (r.rolls.length > 0 && r.kept > 0);
}

/** A single roll that tumbles, then settles on the engine's result. */
function RollCard({ roll, hold, onDone }: { roll: Roll; hold: number; onDone: () => void }) {
  const reduce = useReducedMotion();
  // Keep the latest callback without restarting the timer: the page re-renders
  // constantly while narration streams, which used to keep cards up for ever.
  const done = useRef(onDone);
  done.current = onDone;
  const [face, setFace] = useState<number | string>("?");
  const [settled, setSettled] = useState(false);
  const d20 = isD20(roll);

  useEffect(() => {
    if (reduce) {
      setFace(d20 ? roll.kept : roll.total);
      setSettled(true);
    } else {
      const top = d20 ? 20 : Math.max(4, roll.total);
      const id = setInterval(() => setFace(1 + Math.floor(Math.random() * top)), 70);
      const stop = setTimeout(() => {
        clearInterval(id);
        setFace(d20 ? roll.kept : roll.total);
        setSettled(true);
      }, 850);
      return () => {
        clearInterval(id);
        clearTimeout(stop);
      };
    }
  }, [roll, d20, reduce]);

  useEffect(() => {
    if (!settled) return;
    const t = setTimeout(() => done.current(), hold);
    return () => clearTimeout(t);
  }, [settled, hold]);

  const tone = roll.critical ? "#e6c77a" : roll.success === false ? "#c4483f" : roll.success ? "#6fc4a3" : "#c9a24a";
  const verdict = roll.critical
    ? roll.success === false
      ? "Critical!"
      : "Critical hit!"
    : roll.success === undefined
      ? null
      : roll.success
        ? "Success"
        : "Failure";

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 40, scale: 0.9 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, x: 60 }}
      className="flex items-center gap-3 rounded-xl bg-ink-2/95 gilded px-3 py-2 backdrop-blur-sm w-[300px] max-w-[calc(100vw-32px)]"
      role="status"
    >
      <motion.div
        className="w-16 h-16 shrink-0"
        animate={settled || reduce ? { rotate: 0, scale: 1 } : { rotate: [0, 200, 420, 610, 720], scale: [0.6, 1.1, 0.9, 1.05, 1] }}
        transition={{ duration: 0.85, ease: "easeOut" }}
      >
        <D20Face value={face} tone={tone} shape={d20 ? "d20" : "damage"} />
      </motion.div>
      <div className="min-w-0">
        <div className="text-[13px] leading-tight text-mist truncate" title={roll.label}>
          {roll.label}
        </div>
        <div className="font-display text-lg text-bone">
          {settled ? (
            <>
              {roll.total}
              {roll.target ? <span className="text-mist text-sm"> vs {roll.target}</span> : null}
            </>
          ) : (
            "…"
          )}
        </div>
        {settled && (
          <div className="text-[13px] text-faint">
            {roll.dice}
            {d20 && roll.rolls.length > 1 ? ` (${roll.advantage}: ${roll.rolls.join(", ")})` : d20 ? ` [${roll.kept}${roll.modifier ? signed(roll.modifier) : ""}]` : roll.rolls.length ? ` [${roll.rolls.join("+")}]` : ""}
            {verdict && (
              <span className="ml-2 font-display text-xs tracking-wider uppercase" style={{ color: tone }}>
                {verdict}
              </span>
            )}
          </div>
        )}
      </div>
    </motion.div>
  );
}

/** Shows the latest rolls as a short stack in the corner. Older rolls are
 * dropped rather than queued, so the dice keep pace with the story (every
 * roll is also written in the story log). */
export function DiceTray({ rolls, onConsume }: { rolls: { id: number; roll: Roll }[]; onConsume: (id: number) => void }) {
  useEffect(() => {
    for (const r of rolls.slice(0, -3)) onConsume(r.id);
  }, [rolls, onConsume]);
  const visible = rolls.slice(-3);
  const hold = rolls.length > 1 ? 1400 : 2200;
  return (
    <div className="pointer-events-none fixed bottom-28 right-4 z-40 flex flex-col items-end gap-2 sm:bottom-6">
      <AnimatePresence initial={false}>
        {visible.map((r) => (
          <RollCard key={r.id} roll={r.roll} hold={hold} onDone={() => onConsume(r.id)} />
        ))}
      </AnimatePresence>
    </div>
  );
}

export { D20Face };
