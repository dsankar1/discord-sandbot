package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	speech "cloud.google.com/go/speech/apiv1"
	"github.com/bwmarrin/discordgo"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

var (
	token  string
	ctx    context.Context
	client *speech.Client
	stream speechpb.Speech_StreamingRecognizeClient
)

func init() {
	token = os.Getenv("DISCORD_JARVIS_TOKEN")
	ctx = context.Background()
}

func translateSpeech(vc *discordgo.VoiceConnection, bytes []byte) {
	fmt.Println("Collected Audio Sample", len(bytes))
	n, err := os.Stdin.Read(bytes)
	if n > 0 {
		if err := stream.Send(&speechpb.StreamingRecognizeRequest{
			StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
				AudioContent: bytes[:n],
			},
		}); err != nil {
			log.Printf("Could not send audio: %v", err)
		}
	}
	if err == io.EOF {
		// Nothing else to pipe, close the stream.
		if err := stream.CloseSend(); err != nil {
			log.Fatalf("Could not close stream: %v", err)
		}
		return
	}
	if err != nil {
		log.Printf("Could not read from stdin: %v", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Cannot stream results: %v", err)
		}
		if err := resp.Error; err != nil {
			log.Fatalf("Could not recognize: %v", err)
		}
		for _, result := range resp.Results {
			fmt.Printf("Result: %+v\n", result)
		}
	}
}

func handleVoiceSpeaking(vc *discordgo.VoiceConnection) {
	bytes := []byte{}
	if vc != nil {
	vcLoop:
		for {
			select {
			case packet, open := <-vc.OpusRecv:
				if open {
					bytes = append(bytes, packet.Opus...)
				} else {
					break vcLoop
				}
			case <-time.After(time.Second):
				if len(bytes) > 0 {
					go translateSpeech(vc, bytes)
					bytes = []byte{}
				}
			}
		}
	}
	fmt.Println("Voice channel closed")
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
		if err != nil {
			return
		}
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
				go handleVoiceSpeaking(vc)
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

	// Creates a client.
	newClient, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	fmt.Println("GCP Client Connected")
	client = newClient
	newStream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
		return
	}
	// Send the initial configuration message.
	if err := newStream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_OGG_OPUS,
					SampleRateHertz: 12000,
					LanguageCode:    "en-US",
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
		return
	}
	stream = newStream

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
