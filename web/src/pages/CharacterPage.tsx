import { useEffect, useState } from "react";
import type { Character, Gender } from "../api";
import { api } from "../api";
import { SheetDetails, XpBar } from "../components/Party";
import { Portrait } from "../components/Portrait";
import { navigate } from "../router";

const statusText: Record<Character["status"], string> = {
  idle: "Resting in town, ready for adventure.",
  adventuring: "Away on an adventure.",
  fallen: "Fallen in battle.",
};

const hireOptions: { value: Character["hire"]; label: string; note: string }[] = [
  { value: "no", label: "Not for hire", note: "Only you can take them adventuring." },
  { value: "auto", label: "Looking for adventure", note: "Other players can recruit them at a tavern. Their agent plays them on your key, and you can take over any time." },
  { value: "ask", label: "Ask me first", note: "Listed at the tavern; a leader asks first, and you answer from your front page." },
];

function HireSetting({ ch, onSaved }: { ch: Character; onSaved: (c: Character) => void }) {
  const [msg, setMsg] = useState<string | null>(null);
  const choose = async (hire: Character["hire"]) => {
    setMsg(null);
    try {
      await api.setHire(ch.id, hire);
      onSaved({ ...ch, hire });
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <>
      <h2 className="mt-8 font-display text-sm tracking-[0.2em] uppercase text-gold">Looking for adventure?</h2>
      <div className="mt-2 space-y-1" role="radiogroup" aria-label="Who can recruit this hero">
        {hireOptions.map((o) => {
          const on = ch.hire === o.value;
          return (
            <button
              key={o.value}
              role="radio"
              aria-checked={on}
              onClick={() => !on && void choose(o.value)}
              className={`block w-full rounded-lg px-3 py-2 text-left ${on ? "bg-ink-3 ring-1 ring-gold/60" : "hover:bg-ink-3/60"}`}
            >
              <span className={on ? "text-gold-2" : "text-bone"}>{on ? "● " : "○ "}{o.label}</span>
              <span className="block text-[13px] text-faint">{o.note}</span>
            </button>
          );
        })}
      </div>
      {msg && <p className="mt-2 text-[14px] text-[#f0a49b]">{msg}</p>}
    </>
  );
}

/** Heroes made before genders existed choose one, once. */
function GenderSetting({ ch, onSaved }: { ch: Character; onSaved: (c: Character) => void }) {
  const [msg, setMsg] = useState<string | null>(null);
  const choose = async (g: Gender) => {
    setMsg(null);
    try {
      await api.setGender(ch.id, g);
      onSaved(await api.character(ch.id));
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <div className="mt-4 rounded-lg bg-ink-3/70 px-4 py-3 ring-1 ring-gold/40">
      <p className="text-[15px] text-bone">Is {ch.sheet.name} male or female? It shapes their portrait and voice, and can't be changed later.</p>
      <div className="mt-2 flex gap-2">
        <button onClick={() => void choose("m")} className="rounded-md bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2">
          Male
        </button>
        <button onClick={() => void choose("f")} className="rounded-md bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2">
          Female
        </button>
      </div>
      {msg && <p className="mt-2 text-[14px] text-[#f0a49b]">{msg}</p>}
    </div>
  );
}

export function CharacterPage({ id }: { id: string }) {
  const [ch, setCh] = useState<Character | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const load = () => api.character(id).then(setCh).catch((e: Error) => setError(e.message));
    void load();
    const t = setInterval(load, 15000); // pick up a freshly painted portrait
    return () => clearInterval(t);
  }, [id]);

  if (!ch) return <div className="grid h-full place-items-center text-mist italic">{error ?? "Finding them in the roster…"}</div>;
  const s = ch.sheet;

  const retire = async () => {
    if (!confirm(`Retire ${s.name}? They will leave your roster for good.`)) return;
    try {
      await api.retire(ch.id);
      navigate("/");
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:py-12">
      <button onClick={() => navigate("/")} className="text-mist hover:text-bone">
        ← Roster
      </button>
      <div className="mt-6 grid gap-8 md:grid-cols-[320px_minmax(0,1fr)]">
        <div>
          <div className="overflow-hidden rounded-2xl gilded bg-ink">
            {ch.portrait ? (
              <img src={ch.portrait} alt={`Portrait of ${s.name}`} className={`aspect-square w-full object-cover ${ch.status === "fallen" ? "grayscale" : ""}`} />
            ) : (
              <div className="grid aspect-square place-items-center">
                <Portrait c={s} size={140} painting={ch.painting} />
                {ch.painting && <p className="-mt-10 text-[14px] italic text-mist">Painting portrait…</p>}
              </div>
            )}
          </div>
          <h1 className="mt-5 font-display text-3xl text-gold-2">{s.name}</h1>
          <p className="text-mist">
            Level {s.level} {s.gender === "f" ? "female " : s.gender === "m" ? "male " : ""}
            {s.race} {s.class} · {s.xp} XP
          </p>
          {!s.gender && ch.status !== "fallen" && <GenderSetting ch={ch} onSaved={setCh} />}
          {ch.xpLevels?.length > 0 && <XpBar c={s} levels={ch.xpLevels} />}
          <p className="mt-2 text-[15px] italic text-bone/70">{statusText[ch.status]}</p>
          {ch.status === "adventuring" && ch.adventureId && (
            <button onClick={() => navigate(`/play/${ch.adventureId}`)} className="mt-3 rounded-lg bg-gold px-4 py-2 font-display text-ink hover:bg-gold-2">
              Rejoin the adventure
            </button>
          )}

          {ch.status !== "fallen" && <HireSetting ch={ch} onSaved={setCh} />}

          <h2 className="mt-8 font-display text-sm tracking-[0.2em] uppercase text-gold">Chronicle</h2>
          {(ch.history ?? []).length === 0 ? (
            <p className="mt-2 italic text-faint">No tales told yet.</p>
          ) : (
            <ol className="mt-2 space-y-1.5 border-l border-gold/30 pl-4">
              {ch.history!.map((h, i) => (
                <li key={i} className="text-[15px] text-bone/85">
                  {h}
                </li>
              ))}
            </ol>
          )}
          {ch.status !== "adventuring" && (
            <button onClick={retire} className="mt-10 text-[14px] text-faint hover:text-[#f0a49b]">
              {ch.status === "fallen" ? "Retire for good (removes them from the Hall of the Fallen)" : "Retire this hero"}
            </button>
          )}
          {error && <p className="mt-3 text-[14px] text-[#f0a49b]">{error}</p>}
        </div>
        <div>
          <SheetDetails
            c={s}
            onImprove={
              ch.status === "idle"
                ? async (increases) => {
                    await api.improveInTown(ch.id, increases);
                    setCh(await api.character(ch.id));
                  }
                : undefined
            }
            onEquip={
              ch.status === "idle"
                ? async (item, slot) => {
                    await api.equipInTown(ch.id, item.id, slot);
                    setCh(await api.character(ch.id));
                  }
                : undefined
            }
          />
          {ch.status === "adventuring" && (
            <p className="mt-3 text-[13px] text-faint">
              Change {s.name}'s gear{(s.asi ?? 0) > 0 ? " and choose their ability improvement" : ""} in their adventure.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
