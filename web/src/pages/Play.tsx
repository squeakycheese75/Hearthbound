import { AnimatePresence, motion } from "motion/react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { Ability, Creature, Game, GameEvent, Option } from "../api";
import { ImproveAbilities } from "../components/Improve";
import { api, billed, untilText } from "../api";
import {
  ActionPicker,
  BattleMap,
  InitiativeRibbon, NumberBadge } from "../components/Combat";
import { DiceTray } from "../components/Dice";
import { Narrative } from "../components/Narrative";
import { CharacterSheet, HpBar, PartyCard } from "../components/Party";
import { Portrait } from "../components/Portrait";
import { SceneArt } from "../components/SceneArt";
import { TokenPanel } from "../components/TokenPanel";
import { SoundControl } from "../components/SoundControl";
import { Tips, useTips } from "../components/Tips";
import { sound } from "../sound";
import { castFrom } from "../voices";
import { ReadingBar } from "../components/ReadingBar";
import { NPCList, SceneCast } from "../components/NPCs";
import { TableCalls } from "../components/Calls";
import { Journal, RestButtons } from "../components/Journal";
import { RecruitButton, RecruitDialog } from "../components/Recruit";
import { navigate } from "../router";
import { ArtContext } from "../art";
import type { ArtState } from "../art";
import { useGame } from "../useGame";

/** A hero of yours levelled up: what they gained, front and centre. */
function LevelUpModal({
  e,
  hero,
  onClose,
  onImprove,
}: {
  e: GameEvent | null;
  hero?: Creature;
  onClose: () => void;
  onImprove: (characterId: string, increases: Partial<Record<Ability, number>>) => Promise<void>;
}) {
  return (
    <AnimatePresence>
      {e && (
        <motion.div className="fixed inset-0 z-50 grid place-items-center bg-black/70 p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose}>
          <motion.div
            role="dialog"
            aria-label="Level up"
            className="w-full max-w-md rounded-2xl bg-ink-2 p-6 text-center ring-1 ring-gold/60 shadow-[0_0_60px_rgba(201,162,74,.35)]"
            initial={{ scale: 0.9, y: 20 }}
            animate={{ scale: 1, y: 0 }}
            exit={{ scale: 0.95, opacity: 0 }}
            onClick={(ev) => ev.stopPropagation()}
          >
            {hero && (
              <div className="mx-auto w-fit">
                <Portrait c={hero} size={96} />
              </div>
            )}
            <div className="mt-3 font-display text-[13px] uppercase tracking-[0.35em] text-gold">Level {e.levelUp?.level}</div>
            <h2 className="mt-1 font-display text-2xl text-gold-2">{e.text}</h2>
            <ul className="mt-4 space-y-2 text-left text-[15px] text-bone">
              {(e.levelUp?.gains ?? []).map((g) => (
                <li key={g} className="flex gap-2 rounded-lg bg-ink/60 px-3 py-2 gilded">
                  <span aria-hidden className="text-gold">✦</span>
                  <span>{g}</span>
                </li>
              ))}
            </ul>
            {hero && (hero.asi ?? 0) > 0 && <ImproveAbilities c={hero} onImprove={(inc) => onImprove(hero.id, inc)} />}
            <button onClick={onClose} className="mt-5 rounded-lg bg-gold px-6 py-2 font-display tracking-wider text-ink hover:bg-gold-2">
              {hero && (hero.asi ?? 0) > 0 ? "Decide later" : "Onward"}
            </button>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

/** Shown when the leader isn't playing: their AI runs the DM, so the story waits for them. */
function PausedNotice({ mine, leader, onPlay }: { mine: Creature[]; leader: boolean; onPlay: (characterId: string) => void }) {
  const playable = leader ? mine.filter((c) => !c.dead) : [];
  return (
    <div className="rounded-lg bg-ink-3/70 px-4 py-3 ring-1 ring-gold/40">
      <p className="font-display text-gold-2">The story is paused</p>
      <p className="mt-0.5 text-[15px] text-mist">
        {leader
          ? playable.length > 0
            ? "Your AI runs the Dungeon Master, so the story only moves while you play one of your characters. Who will you play?"
            : "Your AI runs the Dungeon Master, but none of your characters can carry on. Recruit one at a tavern to continue."
          : "The party's leader runs the Dungeon Master on their AI, so the story waits until they play a character again."}
      </p>
      {playable.length > 0 && (
        <div className="mt-2.5 flex flex-wrap gap-2">
          {playable.map((c) => (
            <button key={c.id} onClick={() => onPlay(c.id)} className="rounded-md bg-gold px-3 py-1.5 font-display text-[14px] text-ink hover:bg-gold-2">
              Play as {c.name}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/** Shown in exploration when the table is not waiting on you. */
function WaitingNotice({ game, hero, onTakeControl }: { game: Game; hero?: Creature; onTakeControl: (characterId: string) => void }) {
  const agentPlaying = hero && hero.controller !== "human";
  const acted = hero && game.pending?.[hero.id] !== undefined;
  return (
    <div className="mb-2.5 flex flex-wrap items-center justify-between gap-2 rounded-lg bg-ink-3/70 px-3 py-2 text-[15px] text-mist ring-1 ring-line">
      <span>
        {agentPlaying
          ? `Your agent is playing ${hero.name}.`
          : acted
            ? `You've acted: "${game.pending![hero!.id]}".`
            : "It isn't your move yet."}{" "}
        {game.waitingOn.length > 0 && `Waiting for ${game.waitingOn.join(", ")}.`}
      </span>
      {agentPlaying && (
        <button onClick={() => onTakeControl(hero.id)} className="rounded-md bg-gold px-3 py-1 font-display text-[14px] text-ink hover:bg-gold-2">
          Play as {hero.name}
        </button>
      )}
    </div>
  );
}

function statusText(
  thinking: string | null,
  streams: Record<string, string>,
  creatures: Creature[],
  resumed: boolean,
): string {
  const name = (id: string) =>
    creatures.find((c) => c.id === id)?.name ?? "A companion";
  const speaking = Object.keys(streams).find((k) => streams[k]);
  const who = speaking ?? thinking;
  if (who === "dm")
    return speaking
      ? "The Dungeon Master is narrating…"
      : "The Dungeon Master is considering what happens…";
  if (who)
    return speaking
      ? `${name(who)} is speaking…`
      : `${name(who)} is deciding what to do…`;
  if (resumed)
    return "The table is still resolving the last turn. It will appear here when it's done.";
  return "The table is resolving your action…";
}


export function Play({ id }: { id: string }) {
  const live = useGame(id);
  const { game } = live;
  const [pending, setPending] = useState(false);
  const [text, setText] = useState("");
  const [sheet, setSheet] = useState<Creature | null>(null);
  const [tokensOpen, setTokensOpen] = useState(false);
  const [recruitOpen, setRecruitOpen] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [hovered, setHovered] = useState<string | null>(null);
  const began = useRef(false);

  const run = async (fn: () => Promise<void>, onFail?: () => void) => {
    setPending(true);
    try {
      await fn();
    } catch (e) {
      onFail?.();
      live.setError((e as Error).message);
      void live.reload();
    } finally {
      setPending(false);
    }
  };

  useEffect(() => {
    if (game && !game.started && !began.current) {
      began.current = true;
      void run(() => api.begin(id));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game?.started, id]);

  // Scenes from before NPCs were tracked (or a failed check): ask the server to
  // look for NPCs in the latest narration once. It quietly does nothing if busy.
  const castAsked = useRef("");
  useEffect(() => {
    if (!game?.castStale || game.busy) return;
    const key = `${id}:${(game.logBase ?? 0) + (game.log?.length ?? 0)}`;
    if (castAsked.current === key) return;
    castAsked.current = key;
    api.checkCast(id).catch(() => undefined);
  }, [game?.castStale, game?.busy, game?.logBase, game?.log?.length, id]);

  const party = useMemo(
    () =>
      game
        ? (game.party
            .map((pid) => game.creatures.find((c) => c.id === pid))
            .filter(Boolean) as Creature[])
        : [],
    [game],
  );
  const mineIds = useMemo(() => new Set(game?.mine ?? []), [game?.mine]);
  const hero = party.find((c) => mineIds.has(c.id) && c.controller === "human") ?? party.find((c) => mineIds.has(c.id));
  const monsters = game?.creatures.filter((c) => c.kind === "monster") ?? [];
  const log = useMemo(
    () => [...(game?.log ?? []), ...live.tail],
    [game?.log, live.tail],
  );
  const combat = game?.mode === "combat" && game.encounter;
  const current = combat
    ? game.encounter!.order[game.encounter!.turn % game.encounter!.order.length]
    : null;
  const options = game?.options ?? [];
  const heroTurn = !!combat && options.length > 0;

  // Ambience follows the scene, with drums in a fight; silence when the tale ends.
  const bed = !game ? "" : game.mode === "combat" ? "combat" : game.mode === "over" ? "" : game.scene.key;
  useEffect(() => {
    sound.ambience(bed);
  }, [bed]);
  useEffect(() => () => sound.ambience(""), []);
  const [tipsOn, setTipsOn] = useTips(game);

  // A level-up for one of your heroes gets its own moment.
  const [celebrate, setCelebrate] = useState<GameEvent | null>(null);
  const seenTail = useRef(0);
  useEffect(() => {
    const fresh = live.tail.slice(seenTail.current);
    seenTail.current = live.tail.length;
    const mine = fresh.filter((e) => e.type === "level_up" && e.actor && mineIds.has(e.actor)).pop();
    if (mine) setCelebrate(mine);
  }, [live.tail, mineIds]);

  // The narrator needs to know who might be speaking.
  const cast = useMemo(() => (game ? castFrom(game) : []), [game]);
  useEffect(() => sound.setCast(id, cast), [id, cast]);
  // A chime when it becomes your move in a fight.
  useEffect(() => {
    if (heroTurn) sound.play("turn");
  }, [heroTurn]);
  const myMove = !!game?.yourTurn;
  const autopilot = (characterId: string, on: boolean) => void run(() => api.autopilot(id, characterId, on));
  const cover = (characterId: string, allow: boolean) => void run(() => api.cover(id, characterId, allow));
  const sendHome = (c: Creature) => {
    if (window.confirm(`Send ${c.name} home? They leave the party and go back to your roster with what they earned.`)) {
      void run(() => api.sendHome(id, c.id));
    }
  };
  const serverBusy = !!game?.busy;
  const busy = pending || serverBusy || !game?.awaiting;
  const [since, setSince] = useState<number | null>(null);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!busy) {
      setSince(null);
      return;
    }
    setSince((s) => s ?? Date.now());
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [busy]);
  const elapsed = since ? Math.max(0, Math.round((now - since) / 1000)) : 0;
  const targetOpts = options.filter(
    (o) => o.actionId === selected && o.targetId,
  );
  const targets = new Set(targetOpts.map((o) => o.targetId!));

  useEffect(() => setSelected(null), [game?.encounter?.turn, game?.mode]);

  const send = (choice?: string) => {
    const t = (choice ?? text).trim();
    if (!t || busy) return;
    if (sound.settings.catchUp) sound.skipAll();
    setText("");
    live.clearError();
    void run(
      () => api.act(id, t),
      () => setText(t),
    );
  };
  const offerSeat = (characterId: string, open: boolean) => void run(() => api.offerSeat(id, characterId, open));
  const giveBack = (characterId: string) => {
    if (!confirm("Leave this table? The character goes back to its owner's agent.")) return;
    void run(() => api.giveSeatBack(id, characterId)).then(() => navigate("/"));
  };
  const fight = (o: Option) => {
    if (sound.settings.catchUp) sound.skipAll();
    setSelected(null);
    void run(() => api.fight(id, o));
  };

  const artState: ArtState = useMemo(
    () => ({
      art: game?.art ?? {},
      painting: game?.painting ?? [],
      enabled: !!game?.artEnabled,
      repaint: (subject: string) => {
        api
          .repaint(id, subject)
          .then(() => live.reload())
          .catch((e: Error) => live.setError(e.message));
      },
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [game?.art, game?.painting, game?.artEnabled, id],
  );

  if (!game) {
    return (
      <div className="grid h-full place-items-center px-4 text-center text-mist italic">
        {live.error ? (
          <div>
            <p className="text-[#f0a49b] not-italic">{live.error}</p>
            <button onClick={() => navigate("/")} className="mt-4 text-gold">
              Back to campaigns
            </button>
          </div>
        ) : (
          "Setting the table…"
        )}
      </div>
    );
  }

  // Billing-weighted, matching the daily cap: cache reads cost a tenth of fresh input.
  const tokens = billed(game.usage);

  const sceneArt = game.art?.scene;
  return (
    <ArtContext.Provider value={artState}>
      <div className="flex h-full flex-col">
        <header className="relative z-30 flex items-center gap-3 border-b border-line/70 bg-ink/90 px-4 py-2.5 backdrop-blur">
          <button
            onClick={() => navigate("/")}
            className="text-mist hover:text-bone"
            aria-label="Back to campaigns"
          >
            ←
          </button>
          <div className="min-w-0 flex-1">
            <h1 className="truncate font-display text-[17px] text-gold-2">
              {game.title}
            </h1>
          </div>
          <span
            className={`hidden sm:inline rounded-full px-2.5 py-0.5 text-[12px] uppercase tracking-widest ${combat ? "bg-blood/30 text-[#f0a49b]" : game.mode === "over" ? "bg-ink-3 text-mist" : "bg-verdigris/20 text-[#9fd8c2]"}`}
          >
            {combat
              ? `Combat · round ${game.encounter!.round}`
              : game.mode === "over"
                ? "Concluded"
                : "Exploring"}
          </span>
          <button
            onClick={() => setTipsOn(!tipsOn)}
            className={`rounded-full px-2 text-[15px] font-display ring-1 ${tipsOn ? "text-[#9fd8c2] ring-verdigris/50" : "text-faint ring-line hover:text-bone"}`}
            aria-label={tipsOn ? "Hide tips" : "Show tips"}
            title={tipsOn ? "Hide tips" : "Show tips"}
          >
            ?
          </button>
          <SoundControl cast={cast.filter((c) => c.kind !== "monster" && (game.party.includes(c.id) || game.npcs?.some((n) => n.id === c.id && n.here)))} />
          {tokens > 0 && (
            <button
              onClick={() => setTokensOpen(true)}
              className="hidden md:inline rounded px-1.5 py-0.5 text-[12px] text-faint tabular-nums hover:bg-ink-3 hover:text-bone"
              title="Tokens by agent: who pays, prompt sizes, cache use"
            >
              ≈{(tokens / 1000).toFixed(1)}k tokens
            </button>
          )}
          {game.painting.length > 0 && (
          <span className="hidden sm:flex items-center gap-1.5 text-[12px] text-gold-2" title="Images take 30–90 seconds each">
            <span className="h-3 w-3 rounded-full border-2 border-gold border-t-transparent animate-spin" />
            Painting {game.painting.length} illustration{game.painting.length > 1 ? "s" : ""}…
          </span>
        )}
        {!live.connected && (
            <span
              className="text-[12px] text-ember"
              title="Reconnecting to the live stream"
            >
              ●
            </span>
          )}
        </header>

        <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(0,1fr)_340px]">
          <main className="flex min-h-0 min-w-0 flex-col">
            {/* Stage: the illustrated scene, or the battle map in combat. */}
            <section
              className={`relative shrink-0 overflow-hidden border-b border-line/60 ${combat ? "" : sceneArt ? "h-[34vh] min-h-[180px] max-h-[400px]" : "h-[24vh] min-h-[150px] max-h-[280px]"}`}
            >
              <AnimatePresence mode="wait">
                <motion.div
                  key={sceneArt ?? game.scene.key}
                  className="absolute inset-0"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.9 }}
                >
                  {sceneArt ? (
                    <img
                      src={sceneArt}
                      alt={game.scene.title}
                      className={`h-full w-full object-cover ${combat ? "opacity-25 blur-[3px]" : ""}`}
                    />
                  ) : (
                    <SceneArt
                      sceneKey={game.scene.key}
                      anchor="bottom"
                      className={`h-full w-full ${combat ? "opacity-25 blur-[3px]" : ""}`}
                    />
                  )}
                  {sceneArt && !combat && (
                    <div className="absolute inset-0 bg-gradient-to-t from-ink via-ink/20 to-transparent" />
                  )}
                </motion.div>
              </AnimatePresence>
              {!combat && (game.npcs?.length ?? 0) > 0 && <SceneCast npcs={game.npcs!} />}
              {!combat && game.atTavern && game.you === game.leaderId && game.mode !== "over" && (
                <RecruitButton full={party.length >= game.partyLimit} onOpen={() => setRecruitOpen(true)} />
              )}
              {combat ? (
                <div className="relative px-3 pb-3">
                  <InitiativeRibbon
                    enc={game.encounter!}
                    creatures={game.creatures}
                    hovered={hovered}
                    onHover={setHovered}
                  />
                  <BattleMap
                    enc={game.encounter!}
                    creatures={game.creatures}
                    targets={targets}
                    onTarget={(tid) =>
                      fight(targetOpts.find((o) => o.targetId === tid)!)
                    }
                    sceneKey={game.scene.key}
                    sceneArt={sceneArt}
                    battlemap={game.art?.battlemap}
                    hovered={hovered}
                    onHover={setHovered}
                    reach={heroTurn && !busy ? hero : undefined}
                  />
                </div>
              ) : (
                <div className="absolute inset-x-0 bottom-0 px-5 pb-3 sm:px-8">
                  <div className="font-display text-[22px] sm:text-[28px] text-bone drop-shadow-[0_2px_10px_rgba(0,0,0,.9)]">
                    {game.scene.title}
                  </div>
                  {game.scene.mood && (
                    <div className="text-[15px] italic text-bone/70">
                      {game.scene.mood}
                    </div>
                  )}
                  {game.artEnabled && (
                    <button
                      onClick={() => artState.repaint?.("scene")}
                      disabled={game.painting.includes("scene")}
                      className="absolute bottom-3 right-4 rounded-md bg-ink/70 px-2.5 py-1 text-[12px] tracking-wider text-bone/80 ring-1 ring-line hover:text-gold-2 disabled:opacity-70"
                    >
                      {game.painting.includes("scene")
                        ? "Painting scene…"
                        : "Repaint scene"}
                    </button>
                  )}
                </div>
              )}
            </section>

            {/* Mobile party strip */}
            <div className="flex gap-3 overflow-x-auto scroll-thin border-b border-line/60 px-3 py-2 lg:hidden">
              {party.map((c) => (
                <button
                  key={c.id}
                  onClick={() => setSheet(c)}
                  className="flex shrink-0 items-center gap-2 rounded-lg bg-ink-2/80 px-2 py-1"
                >
                  <Portrait c={c} size={30} active={c.id === current} />
                  <div className="w-20 text-left">
                    <div className="truncate text-[13px] text-bone">
                      {c.name}
                    </div>
                    <HpBar c={c} thin />
                  </div>
                </button>
              ))}
            </div>

            {/* The story so far, on parchment. */}
            <div className="min-h-0 flex-1 overflow-y-auto scroll-thin">
              <article className="parchment mx-auto my-4 min-h-[calc(100%-2rem)] max-w-[760px] rounded-sm sm:my-6">
                <Narrative
                  log={log}
                  base={game.logBase ?? 0}
                  settleKey={`${myMove}|${busy}|${(game.choices ?? []).join("|")}|${game.paused}`}
                  streams={live.streams}
                  thinking={live.thinking}
                  creatures={game.creatures}
                  heroId={hero?.id}
                />
              </article>
            </div>

            {/* Your move. */}
            <footer className="shrink-0 border-t border-line/70 bg-ink-2/95 px-4 py-3">
              <div className="mx-auto max-w-[760px]">
                {tipsOn && (
                  <Tips state={{ game, hero, combat: !!combat, heroTurn, busy, myMove }} onHide={() => setTipsOn(false)} />
                )}
                {game.mode === "over" ? (
                  <div className="flex items-center justify-between gap-4">
                    <p className="font-display text-gold-2">
                      The tale is told.
                    </p>
                    <button
                      onClick={() => navigate("/new")}
                      className="rounded-lg bg-gold px-4 py-2 font-display text-ink"
                    >
                      New adventure
                    </button>
                  </div>
                ) : game.paused && !busy ? (
                  <PausedNotice mine={party.filter((c) => mineIds.has(c.id))} leader={game.you === game.leaderId} onPlay={(cid) => autopilot(cid, false)} />
                ) : combat ? (
                  heroTurn && !busy ? (
                    <div>
                      <div className="mb-2 font-display text-[13px] tracking-[0.2em] uppercase text-gold">
                        Your turn, {hero?.name}
                      </div>
                      <ActionPicker
                        options={options}
                        busy={busy}
                        onChoose={fight}
                        selected={selected}
                        setSelected={setSelected}
                      />
                    </div>
                  ) : (
                    <p className="text-mist italic">
                      {busy
                        ? "The battle rages…"
                        : game.waitingOn.length
                          ? `Waiting for ${game.waitingOn.join(", ")} to take their turn…`
                          : "Waiting for your turn…"}
                    </p>
                  )
                ) : (
                  <>
                    {!busy && !myMove && (
                      <WaitingNotice game={game} hero={hero} onTakeControl={(cid) => autopilot(cid, false)} />
                    )}
                    {!busy && myMove && (
                      <div className="mb-2.5 flex flex-wrap gap-2" aria-label="Suggested actions">
                        {(game.choices ?? []).map((p) => (
                          <button
                            key={p}
                            onClick={() => send(p)}
                            title={game.mayType ? "Do this now (or type your own below)" : "Do this now"}
                            className="group flex items-center gap-1.5 rounded-full bg-ink-3 py-1.5 pl-2.5 pr-3.5 text-[15px] text-bone/90 ring-1 ring-gold/30 transition hover:bg-gold/15 hover:text-gold-2 hover:ring-gold/70"
                          >
                            <span aria-hidden className="text-[11px] text-gold/70 group-hover:text-gold-2">◆</span>
                            {p}
                          </button>
                        ))}
                        {game.mayType && <RestButtons onRest={send} />}
                      </div>
                    )}
                    {!game.mayType && (
                      <p className="text-[14px] text-faint">
                        {game.typing === "leader" ? "Only the leader types actions here; pick one of the choices above." : "This table plays from the quick choices."}
                      </p>
                    )}
                    {game.mayType && (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        send();
                      }}
                      className="relative flex items-end gap-2"
                    >
                      <textarea
                        value={text}
                        onChange={(e) => setText(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" && !e.shiftKey) {
                            e.preventDefault();
                            send();
                          }
                        }}
                        rows={1}
                        maxLength={game.maxAction}
                        disabled={busy || !myMove}
                        placeholder={
                          busy
                            ? "The table is resolving…"
                            : !myMove
                              ? "Waiting for the rest of the table…"
                              : `What does ${hero?.name ?? "your hero"} do or say?`
                        }
                        aria-label="Your action"
                        className="max-h-40 min-h-[46px] flex-1 resize-none rounded-lg bg-ink px-4 py-2.5 text-[18px] text-bone placeholder:text-faint gilded outline-none focus:ring-1 focus:ring-gold disabled:opacity-60"
                      />
                      <button
                        type="submit"
                        disabled={busy || !myMove || !text.trim()}
                        className="h-[46px] rounded-lg bg-gold px-5 font-display tracking-wider text-ink hover:bg-gold-2 disabled:opacity-40"
                      >
                        Act
                      </button>
                      {text.length > game.maxAction * 0.8 && (
                        <span className="pointer-events-none absolute -top-5 right-[88px] text-[12px] tabular-nums text-faint" aria-live="polite">
                          {text.length}/{game.maxAction}
                        </span>
                      )}
                    </form>
                    )}
                  </>
                )}
                {game.you === game.leaderId && game.mode !== "over" && (
                  <div className="mt-2">
                    <TableCalls calls={game.calls ?? []} onAnswer={(cid, yes) => api.answerCall(id, cid, yes)} />
                  </div>
                )}
                {game.you === game.leaderId && game.mode !== "over" && (
                  <label className="mt-2 flex items-center gap-2 text-[13px] text-faint">
                    Typed actions:
                    <select
                      value={game.typing ?? "everyone"}
                      onChange={(e) => void run(() => api.setTyping(id, e.target.value as "everyone" | "leader" | "choices"))}
                      className="rounded bg-ink px-1.5 py-0.5 text-mist ring-1 ring-line focus:ring-gold"
                      title="Who can type their own actions. Everyone else picks from the quick choices: useful at tables with people you don't know."
                    >
                      <option value="everyone">Everyone</option>
                      <option value="leader">Leader only</option>
                      <option value="choices">Quick choices only</option>
                    </select>
                  </label>
                )}
                {!busy && game.mode !== "over" && untilText(game.deadline) && (
                  <p className="mt-2 text-[13px] text-faint">
                    ⏳ If anyone is away, their agents act for them {untilText(game.deadline)}
                    {game.pace?.name ? ` (${game.pace.name} pace)` : ""}.
                  </p>
                )}
                {busy && game.mode !== "over" && (
                  <div
                    className="mt-2 flex items-center gap-2 text-[14px] text-mist"
                    role="status"
                    aria-live="polite"
                  >
                    <span className="h-2 w-2 animate-pulse rounded-full bg-gold" />
                    <span className="truncate">
                      {statusText(
                        live.thinking,
                        live.streams,
                        game.creatures,
                        serverBusy && !pending,
                      )}
                    </span>
                    <span className="ml-auto tabular-nums text-faint">
                      {elapsed}s
                    </span>
                  </div>
                )}
                {live.error && (
                  <div
                    className="mt-2 flex items-center justify-between gap-2 text-[14px] text-[#f0a49b]"
                    role="alert"
                  >
                    <span className="truncate">{live.error}</span>
                    <button
                      onClick={live.clearError}
                      className="text-faint hover:text-bone"
                    >
                      Dismiss
                    </button>
                  </div>
                )}
              </div>
            </footer>
          </main>

          <aside className="hidden min-h-0 flex-col gap-3 overflow-y-auto scroll-thin border-l border-line/60 bg-ink/60 p-4 lg:flex">
            <h2 className="font-display text-[13px] tracking-[0.25em] uppercase text-gold">
              The party
            </h2>
            {party.map((c) => (
              <PartyCard
                key={c.id}
                c={c}
                active={c.id === current}
                onOpen={() => setSheet(c)}
                seat={{
                  mine: mineIds.has(c.id),
                  acted: game.pending?.[c.id] !== undefined,
                  waiting: game.awaiting && !game.busy && game.waitingOn.includes(c.name),
                  covering: !!game.covering?.[c.id],
                  hold: !!game.hold?.[c.id],
                }}
                onAutopilot={mineIds.has(c.id) && game.mode !== "over" ? (on) => autopilot(c.id, on) : undefined}
                onCover={mineIds.has(c.id) && game.mode !== "over" ? (allow) => cover(c.id, allow) : undefined}
                onSendHome={
                  mineIds.has(c.id) && party.length > 1 && c.controller !== "human" && !c.dead && game.atTavern ? () => sendHome(c) : undefined
                }
                player={game.players?.[c.id]}
                seatOffer={
                  game.you === game.leaderId && mineIds.has(c.id) && c.controller !== "human" && !c.dead && game.mode === "exploration"
                    ? { open: !!game.openSeats?.includes(c.id), toggle: () => offerSeat(c.id, !game.openSeats?.includes(c.id)) }
                    : undefined
                }
                onGiveBack={c.playerId && c.playerId === game.you ? () => giveBack(c.id) : undefined}
              />
            ))}
            <NPCList npcs={game.npcs ?? []} />
            {combat && monsters.length > 0 && (
              <>
                <h2 className="mt-3 font-display text-[13px] tracking-[0.25em] uppercase text-[#d0665c]">
                  Foes
                </h2>
                {monsters.map((m) => (
                  <div
                    key={m.id}
                    onMouseEnter={() => setHovered(m.id)}
                    onMouseLeave={() => setHovered(null)}
                    className={`flex items-center gap-3 rounded-xl px-3 py-2 gilded transition ${hovered === m.id ? "bg-ink-3 ring-1 ring-bone/40" : "bg-ink-2/70"} ${m.dead ? "opacity-45" : ""}`}
                  >
                    <span className="relative">
                      <Portrait c={m} size={36} active={m.id === current} />
                      <NumberBadge c={m} size={16} />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex justify-between text-[14px]">
                        <span className={`truncate text-bone ${m.dead ? "line-through" : ""}`}>{m.name}</span>
                        <span className="text-faint">{m.dead ? "slain" : `AC ${m.ac}`}</span>
                      </div>
                      {!m.dead && <HpBar c={m} thin />}
                    </div>
                  </div>
                ))}
              </>
            )}
            <Journal quests={game.quests ?? []} />
            <div className="mt-auto pt-6 text-[12px] leading-relaxed text-faint">
              The DM and companions are AI agents. Every die is rolled by the
              rules engine, never by the model.
            </div>
          </aside>
        </div>

        <DiceTray rolls={live.rolls} onConsume={live.consumeRoll} />
        <ReadingBar />
        <LevelUpModal
          e={celebrate}
          hero={celebrate ? game.creatures.find((c) => c.id === celebrate.actor) : undefined}
          onClose={() => setCelebrate(null)}
          onImprove={async (characterId, increases) => {
            await api.improveInGame(id, characterId, increases);
            await live.reload();
          }}
        />
        <CharacterSheet
          c={
            sheet
              ? (game.creatures.find((c) => c.id === sheet.id) ?? sheet)
              : null
          }
          onClose={() => setSheet(null)}
          onImprove={
            sheet && mineIds.has(sheet.id)
              ? async (increases) => {
                  await api.improveInGame(id, sheet.id, increases);
                  await live.reload();
                }
              : undefined
          }
          onEquip={
            sheet && mineIds.has(sheet.id) && !combat
              ? async (item, slot) => {
                  await api.equipInGame(id, sheet.id, item.id, slot);
                  await live.reload();
                }
              : undefined
          }
        />
        <TokenPanel game={game} open={tokensOpen} onClose={() => setTokensOpen(false)} />
        <RecruitDialog
          open={recruitOpen}
          partyIds={game.party}
          partyClasses={party.map((c) => c.class ?? "").filter(Boolean)}
          onClose={() => setRecruitOpen(false)}
          onRecruit={(cid) => api.recruit(id, cid)}
          asked={(game.calls ?? []).filter((c) => c.kind === "ask").map((c) => c.characterId)}
          wanted={game.wanted ?? ""}
          onAsk={(cid) => api.askForHero(id, cid)}
          onPost={(text) => api.postWanted(id, text)}
        />
      </div>
    </ArtContext.Provider>
  );
}
