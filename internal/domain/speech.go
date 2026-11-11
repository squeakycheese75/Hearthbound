package domain

// SpeechVoices are the AI narration voices a player may pick from.
var SpeechVoices = []string{
	"fable", "onyx", "ash", "ballad", "coral", "echo", "sage", "nova",
	"shimmer", "alloy", "verse", "marin", "cedar",
}

// MaxSpeechChars bounds one line sent for narration.
const MaxSpeechChars = 4000

// SpeechStyle says how a line should be performed.
type SpeechStyle string

// Speech styles.
const (
	StyleNarrator  SpeechStyle = "narrator"
	StyleCharacter SpeechStyle = "character"
)
