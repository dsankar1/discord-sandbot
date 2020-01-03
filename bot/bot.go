package bot

// Bot for receiving and responding with audio
type Bot struct {
}

// New creates a new Bot
func New() (bot *Bot) {
	bot = &Bot{}
	return
}

// ReceiveOpus processes opus audio data
func (bot *Bot) ReceiveOpus(bytes []byte) {
	// fmt.Println("Opus Bytes", bytes)
}
