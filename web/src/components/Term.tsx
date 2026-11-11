import type { ReactNode } from "react";

/** Plain-English meanings of the 5e terms the table shows. */
export const glossary: Record<string, string> = {
  AC: "Armour Class: how hard you are to hit. An attack hits when its roll (a d20 plus a bonus) equals or beats your AC.",
  HP: "Hit points: how much harm you can take. At 0 you fall unconscious and start making death saves.",
  DC: "Difficulty Class: the number a roll has to reach to succeed.",
  Speed: "How far you can move on your turn, in feet. One square on the battle map is 5 feet.",
  STR: "Strength: muscle. Melee attacks with heavy weapons, lifting, climbing, grappling.",
  DEX: "Dexterity: agility. Ranged and finesse attacks, stealth, dodging, Armour Class.",
  CON: "Constitution: toughness. Adds to your hit points every level.",
  INT: "Intelligence: reasoning and lore. A wizard's spellcasting.",
  WIS: "Wisdom: awareness and insight. A cleric's or ranger's spellcasting.",
  CHA: "Charisma: force of personality. A bard's spellcasting, persuasion, deception.",
  action: "Your main thing each turn: attack, cast a spell, move up to a foe, use an item.",
  bonus: "A quick extra on your turn, used as well as your action, if one of your features or spells allows it.",
  free: "Costs nothing: use it any time on your turn, before your action.",
  slots: "Spell slots: the magical energy for your spells. Casting a spell uses one; a long rest brings them all back.",
  advantage: "Advantage: roll two d20s and keep the higher. Disadvantage: keep the lower.",
  save: "A saving throw: a roll to resist something (poison, a spell, a trap), against a DC.",
  initiative: "Turn order in a fight, rolled when combat starts. Highest goes first.",
  features: "Class features: abilities your class gives you. Some can be used a few times, then come back after a short or long rest.",
  // Conditions.
  prone: "Prone: knocked down. Attacks against you from nearby have advantage; your own attacks have disadvantage.",
  poisoned: "Poisoned: disadvantage on attack rolls and ability checks.",
  frightened: "Frightened: disadvantage on attacks while the source of your fear is in sight.",
  restrained: "Restrained: you can't move; attacks against you have advantage, yours have disadvantage.",
  blinded: "Blinded: you can't see. Your attacks have disadvantage; attacks against you have advantage.",
  unconscious: "Unconscious: out cold at 0 HP. You make death saves each turn until healed or stable.",
  dodging: "Dodging: attacks against you have disadvantage until your next turn.",
  raging: "Raging: +2 melee damage, and you take half damage from weapons.",
  reckless: "Attacking recklessly: advantage on your melee attacks, but foes have advantage against you.",
  hidden: "Hidden: your next attack has advantage (and gives you away).",
  exhausted: "Exhausted: disadvantage on ability checks until a long rest.",
  turned: "Turned: fleeing from holy power, unable to act until hurt.",
  frenzied: "In a frenzy: a bonus melee attack each turn; exhausted when the fight ends.",
  dying: "Dying: at 0 HP. Each turn, roll a death save: three successes and you stabilise, three failures and you die. Any healing brings you back.",
};

/**
 * A word with a plain-English explanation on hover (and on focus, when
 * focusable). Unknown terms render as plain text.
 */
export function Term({ k, children, focusable = false }: { k: string; children?: ReactNode; focusable?: boolean }) {
  const text = glossary[k] ?? glossary[k.toLowerCase()];
  if (!text) return <>{children ?? k}</>;
  return (
    <span className="group/term relative inline-block cursor-help" tabIndex={focusable ? 0 : undefined}>
      <span className="underline decoration-dotted decoration-current/40 underline-offset-2">{children ?? k}</span>
      <span
        role="tooltip"
        className="pointer-events-none invisible absolute bottom-full left-1/2 z-50 mb-1.5 w-64 -translate-x-1/2 rounded-lg bg-ink px-3 py-2 text-left text-[13px] font-normal normal-case leading-snug tracking-normal text-bone opacity-0 shadow-xl ring-1 ring-gold/40 transition-opacity group-hover/term:visible group-hover/term:opacity-100 group-focus/term:visible group-focus/term:opacity-100"
      >
        {text}
      </span>
    </span>
  );
}
