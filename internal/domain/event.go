package domain

// EventType tells the UI how to render an event.
type EventType string

// Event types streamed to the client.
const (
	// EvNarrationDelta is a streamed chunk of narration or speech.
	EvNarrationDelta EventType = "narration_delta"
	// EvNarration is a complete block of DM narration.
	EvNarration EventType = "narration"
	// EvSpeech is a complete line said by a character.
	EvSpeech EventType = "speech"
	// EvPlayer is the human player's typed action.
	EvPlayer EventType = "player"
	// EvRoll is a resolved dice roll.
	EvRoll EventType = "roll"
	// EvScene is a scene change.
	EvScene EventType = "scene"
	// EvSystem is a mechanical note (damage, conditions, XP).
	EvSystem EventType = "system"
	// EvState tells the client the game state changed.
	EvState EventType = "state"
	// EvAwaiting means the game waits for the human.
	EvAwaiting EventType = "awaiting"
	// EvThinking shows which seat is acting.
	EvThinking EventType = "thinking"
	// EvError reports a failure.
	EvError EventType = "error"
	// EvImage means an illustration finished; Text holds its art key.
	EvImage EventType = "image"
	// EvLevelUp is a hero reaching a new level; Actor is the hero, LevelUp what they gained.
	EvLevelUp EventType = "level_up"
)

// Roll is a resolved d20 test or damage roll, shown with animated dice.
type Roll struct {
	Label     string `json:"label"`
	Dice      string `json:"dice"`
	Rolls     []int  `json:"rolls"`
	Kept      int    `json:"kept"`
	Modifier  int    `json:"modifier"`
	Total     int    `json:"total"`
	Target    int    `json:"target,omitempty"`
	Success   *bool  `json:"success,omitempty"`
	Advantage string `json:"advantage,omitempty"`
	Critical  bool   `json:"critical,omitempty"`
}

// Event is one entry in the game's visible history.
type Event struct {
	Type   EventType `json:"type"`
	Actor  string    `json:"actor,omitempty"`
	Text   string    `json:"text,omitempty"`
	Roll   *Roll     `json:"roll,omitempty"`
	Scene  *Scene    `json:"scene,omitempty"`
	Stream string    `json:"stream,omitempty"`
	// Cue names a sound effect for the moment, if any.
	Cue Cue `json:"cue,omitempty"`
	// LevelUp lists what a hero gained (EvLevelUp).
	LevelUp *LevelUp `json:"levelUp,omitempty"`
}

// LevelUp is what a hero gained on reaching a level.
type LevelUp struct {
	Level int      `json:"level"`
	Gains []string `json:"gains"`
}

// Cue is a sound effect the UI plays for an event. The UI picks the sound;
// the engine only says what happened.
type Cue string

// Sound cues.
const (
	CueCombat  Cue = "combat"  // a fight starts
	CueVictory Cue = "victory" // the party wins a fight
	CueDefeat  Cue = "defeat"  // the whole party falls
	CueFall    Cue = "fall"    // one hero falls
	CueCoins   Cue = "coins"   // gold changes hands
	CueLoot    Cue = "loot"    // an item is found or given
	CueLevel   Cue = "level"   // a hero levels up
	CueXP      Cue = "xp"      // the party earns experience
	CueRest    Cue = "rest"    // the party rests
	CueArrive  Cue = "arrive"  // someone joins or appears
	CueLeave   Cue = "leave"   // someone leaves
	CueChapter Cue = "chapter" // a chapter closes
)

// Transient reports whether the event is only for live streaming and not stored in the log.
func (e Event) Transient() bool {
	switch e.Type {
	case EvNarrationDelta, EvState, EvThinking, EvImage:
		return true
	}
	return false
}
