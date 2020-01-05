package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	token string
)

func init() {

}

func handleVoiceConnection(vc *discordgo.VoiceConnection) {
	for packet := range vc.OpusRecv {
		fmt.Printf("Speaking: %v / Timestamp: %v\n", vc.UserID, packet.Timestamp)
	}
	fmt.Println("Opus channel closed")
}

func closeVoiceConnections(s *discordgo.Session) {
	for _, vc := range s.VoiceConnections {
		vc.Close()
	}
}

func disconnectVoiceConnection(vc *discordgo.VoiceConnection) (err error) {
	err = vc.Disconnect()
	if err == nil && vc.OpusRecv != nil {
		close(vc.OpusRecv)
	}
	return
}

func disconnectVoiceConnections(s *discordgo.Session) (err error) {
	for _, vc := range s.VoiceConnections {
		err = disconnectVoiceConnection(vc)
	}
	return
}

func findUserVoiceState(s *discordgo.Session, userID string) (*discordgo.VoiceState, error) {
	for _, guild := range s.State.Guilds {
		for _, vs := range guild.VoiceStates {
			if vs.UserID == userID {
				return vs, nil
			}
		}
	}
	return nil, errors.New("Could not find user's voice state")
}

func messageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	mc := strings.ToLower(m.Content)

	if strings.Contains(mc, "jarvis voice connections") {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Voice connection count: %v", len(s.VoiceConnections)))
		return
	}

	vc := s.VoiceConnections[m.GuildID]

	if strings.Contains(mc, "jarvis summon") {
		vs, err := findUserVoiceState(s, m.Author.ID)
		if err != nil {
			fmt.Println("Failed to find user voice channel,", err)
			return
		}

		if vc == nil {
			err = disconnectVoiceConnections(s)
			if err != nil {
				fmt.Println("Failed to disconnect some voice connections,", err)
			}

			fmt.Println("Creating voice connection")
			vc, err = s.ChannelVoiceJoin(vs.GuildID, vs.ChannelID, false, false)
			if err != nil {
				fmt.Println("Failed to join user's voice channel,", err)
			} else {
				go handleVoiceConnection(vc)
			}
		} else {
			fmt.Println("Changing voice channel")
			err = vc.ChangeChannel(vs.ChannelID, false, false)
			if err != nil {
				fmt.Println("Failed to change voice channel,", err)
			}
		}
	}

	if strings.Contains(mc, "jarvis dismiss") && vc != nil {
		err := disconnectVoiceConnections(s)
		if err != nil {
			fmt.Println("Failed to disconnect some voice connections,", err)
		}
	}
}

func main() {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session, ", err)
		return
	}

	s.AddHandler(messageCreateHandler)

	err = s.Open()
	if err != nil {
		fmt.Println("Error opening websocket connection, ", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sigchan

	closeVoiceConnections(s)
	s.Close()
}
