import { sound, useReading } from "../sound";

/** A small bar while the story is read aloud: who's speaking, and skip controls. */
export function ReadingBar() {
  const r = useReading();
  if (!r.active && r.waiting === 0) return null;
  return (
    <div className="fixed bottom-4 left-1/2 z-40 flex -translate-x-1/2 items-center gap-2 rounded-full bg-ink-2/95 py-1.5 pl-4 pr-1.5 text-[14px] text-bone shadow-2xl ring-1 ring-line backdrop-blur">
      <span className="h-2 w-2 animate-pulse rounded-full bg-gold" />
      <span className="max-w-[40vw] truncate">
        {r.who || "Reading"}
        {r.waiting > 0 && <span className="text-faint"> · {r.waiting} more</span>}
      </span>
      <button onClick={() => sound.skip()} className="rounded-full px-3 py-1 text-mist hover:bg-ink-3 hover:text-bone" title="Skip this line">
        Skip ›
      </button>
      <button onClick={() => sound.skipAll()} className="rounded-full bg-ink-3 px-3 py-1 text-mist hover:text-bone" title="Stop reading and catch up">
        Catch up »
      </button>
    </div>
  );
}
