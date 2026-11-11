import { useEffect, useState } from "react";
import type { Character, Notices, OpenSeat, TavernHero, WantedPost } from "../api";
import { api } from "../api";
import { Portrait } from "../components/Portrait";
import { navigate } from "../router";

/** The tavern's notice board: seats in friends' adventures, and heroes looking for one. */
export function Tavern() {
  const [seats, setSeats] = useState<OpenSeat[] | null>(null);
  const [heroes, setHeroes] = useState<TavernHero[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [taking, setTaking] = useState<string | null>(null);
  const [notices, setNotices] = useState<Notices | null>(null);
  const [idle, setIdle] = useState<Character[]>([]);
  const loadNotices = () =>
    api
      .notices()
      .then(setNotices)
      .catch(() => undefined);

  useEffect(() => {
    api.tavernSeats().then(setSeats).catch((e: Error) => setError(e.message));
    void loadNotices();
    api
      .roster()
      .then((r) => setIdle(r.filter((c) => c.status === "idle" && !c.sheet.dead)))
      .catch(() => undefined);
    api
      .tavern()
      .then((t) => setHeroes(t.forHire))
      .catch(() => undefined);
  }, []);

  const take = async (s: OpenSeat) => {
    setTaking(s.character.id);
    setError(null);
    try {
      await api.takeSeat(s.gameId, s.character.id);
      navigate(`/play/${s.gameId}`);
    } catch (e) {
      setError((e as Error).message);
      setTaking(null);
    }
  };

  return (
    <div className="mx-auto max-w-4xl px-4 py-8 sm:py-12">
      <button onClick={() => navigate("/")} className="text-mist hover:text-bone">
        ← Back
      </button>
      <h1 className="mt-4 font-display text-3xl sm:text-4xl text-gold-2">The Tavern</h1>
      <p className="mt-2 text-mist">
        Friends post seats here when they want company. Take one and you play that companion live in their adventure, on your own AI key. The
        character stays theirs, and you can give the seat back whenever you like.
      </p>
      {error && <p className="mt-4 rounded border border-blood/50 bg-blood/10 px-3 py-2 text-[#f0a49b]">{error}</p>}

      <h2 className="mt-10 mb-4 font-display text-sm tracking-[0.25em] uppercase text-gold">Seats at the table</h2>
      {seats === null && !error && <p className="text-mist italic">Reading the notice board…</p>}
      {seats?.length === 0 && (
        <p className="text-mist italic">
          No open seats right now. When a friend offers one of their companions, it appears here. You can offer seats in your own adventures
          from the party panel.
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        {seats?.map((s) => (
          <div key={s.gameId + s.character.id} className="flex gap-4 rounded-xl bg-ink-2/80 p-4 gilded">
            {s.portrait ? (
              <img src={s.portrait} alt="" className="h-24 w-24 shrink-0 rounded-lg object-cover ring-1 ring-gold/40" />
            ) : (
              <Portrait c={s.character} size={96} />
            )}
            <div className="min-w-0 flex-1">
              <div className="font-display text-xl text-bone">{s.character.name}</div>
              <div className="text-[14px] text-mist">
                Level {s.character.level} {s.character.race} {s.character.class}
                {s.character.persona?.archetype ? ` · ${s.character.persona.archetype}` : ""}
              </div>
              <div className="mt-1.5 text-[14px] text-bone/85">
                In <span className="text-gold-2">{s.title}</span>, led by {s.leader}
              </div>
              <div className="text-[13px] text-faint">
                {s.scene} · with {s.party.filter((n) => n !== s.character.name).join(", ")}
              </div>
              <button
                onClick={() => void take(s)}
                disabled={taking !== null}
                className="mt-3 rounded-lg bg-gold px-4 py-1.5 font-display text-[14px] tracking-wide text-ink hover:bg-gold-2 disabled:opacity-50"
              >
                {taking === s.character.id ? "Taking a seat…" : "Take this seat"}
              </button>
            </div>
          </div>
        ))}
      </div>

      <h2 className="mt-12 mb-2 font-display text-sm tracking-[0.25em] uppercase text-gold">Parties looking for heroes</h2>
      <p className="mb-4 text-[14px] text-faint">
        Offer one of your heroes; if the leader welcomes them, they join the party played by their agent on your key, and you can take over any
        time.
      </p>
      {notices?.posts.length === 0 && <p className="text-mist italic">No posts right now. Leaders post from the tavern in their adventure.</p>}
      <div className="grid gap-4 sm:grid-cols-2">
        {notices?.posts.map((p) => (
          <PostCard key={p.gameId} p={p} idle={idle} onOffered={() => void loadNotices()} />
        ))}
      </div>
      {(notices?.waiting.length ?? 0) > 0 && (
        <p className="mt-4 text-[14px] text-mist">
          Waiting on answers:{" "}
          {notices!.waiting.map((n) => `${n.call.name} (${n.call.kind === "ask" ? "you asked" : "offered"}, ${n.title})`).join("; ")}
        </p>
      )}

      {heroes.length > 0 && (
        <>
          <h2 className="mt-12 mb-2 font-display text-sm tracking-[0.25em] uppercase text-gold">Heroes looking for adventure</h2>
          <p className="mb-4 text-[14px] text-faint">Party leaders can recruit these from the tavern in their adventure.</p>
          <div className="grid gap-3 sm:grid-cols-3">
            {heroes.map((h) => (
              <div key={h.id} className="flex items-center gap-3 rounded-xl bg-ink-2/70 p-3 gilded">
                <Portrait c={h.sheet} size={52} />
                <div className="min-w-0">
                  <div className="truncate font-display text-bone">{h.sheet.name}</div>
                  <div className="truncate text-[13px] text-mist">
                    Level {h.sheet.level} {h.sheet.class} · {h.ownerName}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function PostCard({ p, idle, onOffered }: { p: WantedPost; idle: Character[]; onOffered: () => void }) {
  const [hero, setHero] = useState(idle[0]?.id ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const chosen = hero || idle[0]?.id || "";
  const offer = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.offerHero(p.gameId, chosen);
      onOffered();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="rounded-xl bg-ink-2/80 p-4 gilded">
      <p className="text-lg italic text-bone">“{p.text}”</p>
      <div className="mt-1.5 text-[14px] text-bone/85">
        <span className="text-gold-2">{p.title}</span>, led by {p.leader}
      </div>
      <div className="text-[13px] text-faint">
        {p.scene} · {p.party.join(", ")}
      </div>
      {p.offered ? (
        <p className="mt-3 text-[14px] text-verdigris">You offered {p.offered}. Waiting for {p.leader}.</p>
      ) : idle.length === 0 ? (
        <p className="mt-3 text-[14px] italic text-faint">None of your heroes are free right now.</p>
      ) : (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <select
            value={chosen}
            onChange={(e) => setHero(e.target.value)}
            aria-label="Hero to offer"
            className="rounded-lg bg-ink px-2 py-1.5 text-[14px] text-bone ring-1 ring-line focus:ring-gold"
          >
            {idle.map((c) => (
              <option key={c.id} value={c.id}>
                {c.sheet.name}, level {c.sheet.level} {c.sheet.class}
              </option>
            ))}
          </select>
          <button
            onClick={() => void offer()}
            disabled={busy || !chosen}
            className="rounded-lg bg-gold px-4 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2 disabled:opacity-50"
          >
            {busy ? "Offering…" : "Offer this hero"}
          </button>
        </div>
      )}
      {error && <p className="mt-2 text-[13px] text-[#f0a49b]">{error}</p>}
    </div>
  );
}
