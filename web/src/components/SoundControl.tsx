import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { sound, useSoundSettings, useVoices } from "../sound";
import type { CastMember } from "../voices";
import { aiVoice } from "../voices";

/** The speaker button in the play header, with a panel for sound and narration. */
export function SoundControl({ cast }: { cast: CastMember[] }) {
  const s = useSoundSettings();
  const browserVoices = useVoices();
  const [open, setOpen] = useState(false);
  const [aiVoices, setAiVoices] = useState<string[]>([]);
  const [hasKey, setHasKey] = useState(false);
  const panel = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api
      .me()
      .then((me) => {
        sound.aiVoices = me.voices ?? [];
        setAiVoices(sound.aiVoices);
        setHasKey(me.local || me.keys.some((k) => k.kind === "openai" && k.set));
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!open) return;
    const close = (e: PointerEvent) => {
      if (panel.current && !panel.current.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, [open]);

  const active = s.on || s.narrate;
  return (
    <div ref={panel} className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        className={`rounded px-1.5 py-0.5 text-[17px] hover:bg-ink-3 ${active ? "text-gold-2" : "text-faint"}`}
        aria-label="Sound and narration"
        title="Sound and narration"
      >
        {active ? "🔊" : "🔇"}
      </button>
      {open && (
        <div className="absolute right-0 top-full z-40 mt-2 w-[300px] space-y-3 rounded-lg bg-ink-2 p-4 text-[14px] text-bone shadow-2xl ring-1 ring-line">
          <Toggle label="Sound effects & ambience" on={s.on} set={(on) => sound.update({ on })} />
          <Slider label="Effects" value={s.effects} disabled={!s.on} set={(effects) => sound.update({ effects })} />
          <Slider label="Ambience" value={s.ambience} disabled={!s.on} set={(ambience) => sound.update({ ambience })} />

          <div className="border-t border-line/60 pt-3">
            <Toggle label="Read the story aloud" on={s.narrate} set={(narrate) => sound.update({ narrate })} />
          </div>
          {s.narrate && (
            <>
              <label className="block">
                <span className="text-[12px] uppercase tracking-widest text-faint">Voice</span>
                <select
                  value={s.engine}
                  onChange={(e) => sound.update({ engine: e.target.value as "browser" | "ai" })}
                  className="mt-1 w-full rounded bg-ink-3 px-2 py-1.5 ring-1 ring-line"
                >
                  <option value="browser">Browser voice (free)</option>
                  <option value="ai">AI voice (your OpenAI key)</option>
                </select>
              </label>
              {s.engine === "ai" && !hasKey && (
                <p className="text-[13px] text-ember">Add an OpenAI key in Settings to use AI voices.</p>
              )}
              {s.engine === "ai" ? (
                <label className="block">
                  <span className="text-[12px] uppercase tracking-widest text-faint">Narrator</span>
                  <select
                    value={s.aiVoice}
                    onChange={(e) => sound.update({ aiVoice: e.target.value })}
                    className="mt-1 w-full rounded bg-ink-3 px-2 py-1.5 capitalize ring-1 ring-line"
                  >
                    {aiVoices.map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                  <span className="mt-1 block text-[12px] text-faint">
                    Characters get their own voices. About a cent per minute of speech, billed to your key.
                  </span>
                </label>
              ) : (
                browserVoices.length > 0 && (
                  <label className="block">
                    <span className="text-[12px] uppercase tracking-widest text-faint">Narrator</span>
                    <select
                      value={s.voice}
                      onChange={(e) => sound.update({ voice: e.target.value })}
                      className="mt-1 w-full rounded bg-ink-3 px-2 py-1.5 ring-1 ring-line"
                    >
                      <option value="">Default</option>
                      {browserVoices.map((v) => (
                        <option key={v.name} value={v.name}>
                          {v.name}
                        </option>
                      ))}
                    </select>
                  </label>
                )
              )}
              {s.engine === "ai" && cast.length > 0 && (
                <div>
                  <span className="text-[12px] uppercase tracking-widest text-faint">Character voices</span>
                  <div className="mt-1 max-h-40 space-y-1 overflow-y-auto pr-1">
                    {cast.map((c) => (
                      <label key={c.id} className="flex items-center justify-between gap-2">
                        <span className="truncate text-[13px]">{c.name}</span>
                        <select
                          value={s.castVoices[c.id] ?? ""}
                          onChange={(e) => {
                            const picks = { ...s.castVoices, [c.id]: e.target.value };
                            if (!e.target.value) delete picks[c.id];
                            sound.update({ castVoices: picks });
                          }}
                          className="w-28 rounded bg-ink-3 px-1.5 py-1 text-[13px] capitalize ring-1 ring-line"
                        >
                          <option value="">
                            Auto ({aiVoice(c.id, cast, s.aiVoice, {}, aiVoices)})
                          </option>
                          {aiVoices.map((v) => (
                            <option key={v} value={v}>
                              {v}
                            </option>
                          ))}
                        </select>
                      </label>
                    ))}
                  </div>
                </div>
              )}
              <Toggle label="Catch up when I act" on={s.catchUp} set={(catchUp) => sound.update({ catchUp })} />
              <Slider label="Speed" value={s.rate} min={0.7} max={1.4} set={(rate) => sound.update({ rate })} />
              {sound.aiError && s.engine === "ai" && <p className="text-[13px] text-ember">{sound.aiError}</p>}
            </>
          )}
        </div>
      )}
    </div>
  );
}

function Toggle({ label, on, set }: { label: string; on: boolean; set: (on: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-3">
      <span>{label}</span>
      <button
        role="switch"
        aria-checked={on}
        onClick={() => set(!on)}
        className={`relative h-5 w-9 shrink-0 rounded-full transition ${on ? "bg-gold" : "bg-ink-3 ring-1 ring-line"}`}
      >
        <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-bone transition-all ${on ? "left-[18px]" : "left-0.5"}`} />
      </button>
    </label>
  );
}

function Slider(props: { label: string; value: number; set: (v: number) => void; disabled?: boolean; min?: number; max?: number }) {
  const { label, value, set, disabled, min = 0, max = 1 } = props;
  return (
    <label className={`flex items-center gap-3 ${disabled ? "opacity-40" : ""}`}>
      <span className="w-20 text-[13px] text-mist">{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        step={0.05}
        value={value}
        disabled={disabled}
        onChange={(e) => set(Number(e.target.value))}
        className="flex-1 accent-[#c9a25a]"
      />
    </label>
  );
}
