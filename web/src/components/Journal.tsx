import type { Quest } from "../api";

const mark: Record<Quest["status"], string> = { open: "◆", done: "✓", failed: "✗" };

/** The party's journal: the goals the DM is tracking, open ones first. */
export function Journal({ quests }: { quests: Quest[] }) {
  if (quests.length === 0) return null;
  const sorted = [...quests.filter((q) => q.status === "open"), ...quests.filter((q) => q.status !== "open").reverse()];
  return (
    <section aria-label="Journal" className="mt-4">
      <h2 className="font-display text-[13px] tracking-[0.25em] uppercase text-gold">Journal</h2>
      <ul className="mt-2 space-y-2">
        {sorted.map((q) => (
          <li key={q.title} className={`rounded-lg bg-ink-2/70 px-3 py-2 ${q.status === "open" ? "ring-1 ring-gold/25" : "opacity-60"}`}>
            <div className={`flex gap-2 text-[14px] ${q.status === "open" ? "text-bone" : "text-mist"}`}>
              <span aria-hidden className={q.status === "failed" ? "text-[#f0a49b]" : q.status === "done" ? "text-verdigris" : "text-gold/80"}>
                {mark[q.status]}
              </span>
              <span className={q.status === "failed" ? "line-through" : undefined}>{q.title}</span>
              {q.status !== "open" && <span className="sr-only">({q.status})</span>}
            </div>
            {q.note && <p className="mt-0.5 pl-5 text-[13px] text-faint">{q.note}</p>}
          </li>
        ))}
      </ul>
    </section>
  );
}

const rests = [
  { label: "Short rest", title: "About an hour to bind wounds and catch your breath. The DM decides if it's safe.", text: "We take a short rest, about an hour, to tend our wounds and catch our breath." },
  { label: "Long rest", title: "Make camp for the night: full healing and spell slots, if nothing interrupts.", text: "We make camp and take a long rest for the night." },
];

/** Quick rest actions beside the suggested choices; the DM still rules on them. */
export function RestButtons({ onRest }: { onRest: (text: string) => void }) {
  return (
    <>
      {rests.map((r) => (
        <button
          key={r.label}
          onClick={() => onRest(r.text)}
          title={r.title}
          className="flex items-center gap-1.5 rounded-full py-1.5 pl-2.5 pr-3.5 text-[15px] text-mist ring-1 ring-line transition hover:text-bone hover:ring-gold/50"
        >
          <span aria-hidden className="text-[12px]">☾</span>
          {r.label}
        </button>
      ))}
    </>
  );
}
