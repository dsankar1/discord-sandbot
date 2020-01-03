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

func mCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	vc := s.VoiceConnections[m.GuildID]

	mc := strings.ToLower(m.Content)

	if strings.Contains(mc, "jarvis summon") {
		vs, err := findUserVoiceState(s, m.Author.ID)
		if err != nil {
			fmt.Println("Failed to find user voice channel, ", err)
			return
		}

		if vc == nil || !vc.Ready {
			fmt.Println("Creating new voice connection")
			_, err := s.ChannelVoiceJoin(vs.GuildID, vs.ChannelID, false, false)
			if err != nil {
				fmt.Println("Failed to join user's voice channel, ", err)
				s.ChannelMessageSend(m.ChannelID, "I had trouble joining your voice channel")
				return
			}
		} else {
			fmt.Println("Changing voice channel")
			err = vc.ChangeChannel(vs.ChannelID, false, false)
			if err != nil {
				fmt.Println("Failed to change voice channel, ", err)
				s.ChannelMessageSend(m.ChannelID, "I had trouble joining your voice channel")
				return
			}
		}
	}

	if strings.Contains(mc, "jarvis dismiss") {
		if vc != nil {
			err := vc.Disconnect()
			if err != nil {
				fmt.Println("Failed to disconnect voice connection, ", err)
				s.ChannelMessageSend(m.ChannelID, "I had trouble leaving the voice channel")
				return
			}
		}
	}
}

func voiceStateUpdateHandler(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	if s.State.User.ID == vs.UserID {
		fmt.Println("Jarvis's voice state was updated")
	}
}

func processAudio(voiceConnection *discordgo.VoiceConnection) {
	for packet := range voiceConnection.OpusRecv {
		fmt.Println("Packet: ", packet.Timestamp)
	}
	fmt.Println("Opus channel closed")
}

// func voiceHandler() {
// 	<-voiceConnected
// 	for {
// 		packet := <-voiceConnection.OpusRecv
// 		fmt.Println("Packet: ", packet.Timestamp)
// 	}
// }

func main() {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord s, ", err)
		return
	}

	s.AddHandler(mCreateHandler)
	s.AddHandler(voiceStateUpdateHandler)

	err = s.Open()
	if err != nil {
		fmt.Println("Error opening websocket connection, ", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sigchan
	s.Close()
}
