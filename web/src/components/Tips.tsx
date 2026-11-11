import { useEffect, useState } from "react";
import type { Creature, Game } from "../api";

const storeKey = "hearthbound.tips";

/** Whether tips show: on in a first adventure, otherwise when turned on. */
export function useTips(game: Game | null): [boolean, (on: boolean) => void] {
  const read = () => {
    try {
      return localStorage.getItem(storeKey);
    } catch {
      return null;
    }
  };
  const [pref, setPref] = useState<string | null>(read);
  useEffect(() => setPref(read()), [game?.id]);
  const on = pref === "on" || (pref !== "off" && !!game?.tutorial);
  const set = (v: boolean) => {
    try {
      localStorage.setItem(storeKey, v ? "on" : "off");
    } catch {
      // no storage: the choice lasts this page
    }
    setPref(v ? "on" : "off");
  };
  return [on, set];
}

export interface TipState {
  game: Game;
  hero?: Creature;
  combat: boolean;
  heroTurn: boolean;
  busy: boolean;
  myMove: boolean;
}

/** The one tip that fits the moment. */
export function tipFor({ game, hero, combat, heroTurn, busy, myMove }: TipState): string | null {
  const name = hero?.name ?? "your hero";
  switch (true) {
    case game.mode === "over":
      return null;
    case (hero?.asi ?? 0) > 0:
      return `${name} has an Ability Score Improvement to choose: open their card in The Party and pick which abilities to raise.`;
    case busy:
      return "The Dungeon Master is deciding what happens next. Dice are rolled by the game itself, so nobody can cheat.";
    case combat && heroTurn:
      return "It's your turn. Pick an action (an attack, a spell, or Move), then a target, or click a glowing foe on the map. A bonus action is an extra you can use as well. Press End turn when you're done.";
    case combat:
      return "Everyone acts in initiative order: the row of portraits above the map. Your companions and the monsters take their turns; yours comes round soon.";
    case myMove:
      return `What does ${name} do? Tap a suggestion, or type anything: say something in quotes ("Who are you?") or describe an action (I search the bar). The DM decides what happens, and the dice decide whether it works.`;
  }
  return "Waiting for the rest of the party to act.";
}

/** A short, dismissible hint above the action area. */
export function Tips({ state, onHide }: { state: TipState; onHide: () => void }) {
  const tip = tipFor(state);
  if (!tip) return null;
  return (
    <div className="mb-2.5 flex items-start gap-3 rounded-lg bg-verdigris/10 px-3 py-2 text-[14px] leading-snug text-bone/90 ring-1 ring-verdigris/40" role="note">
      <span aria-hidden className="mt-0.5 text-[#9fd8c2]">✦</span>
      <p className="flex-1">{tip}</p>
      <button onClick={onHide} className="shrink-0 text-[12px] text-faint hover:text-bone" title="Hide tips (bring them back with ? at the top)">
        Hide tips
      </button>
    </div>
  );
}
