package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

// DiscordManager holds program state and shares access to resources
type DiscordManager struct {
	app           *App
	DiscordClient *discordgo.Session
	Sender        chan Message
	Receiver      chan Message
	channels      []string
}

// NewDiscordManager sets up a Discord client and prepares it for starting
func NewDiscordManager(app *App, channels []string) *DiscordManager {
	var err error

	dm := DiscordManager{
		app:      app,
		Sender:   make(chan Message),
		Receiver: make(chan Message),
		channels: channels,
	}

	dm.DiscordClient, err = discordgo.New("Bot " + app.config.DiscordToken)
	if err != nil {
		logger.Fatal("failed to create discord client", zap.Error(err))
	}

	return &dm
}

// Close cleans up and shuts down the Discord client
func (dm *DiscordManager) Close() error {
	return dm.DiscordClient.Close()
}

// Connect prepares the Discord client library and connects to the API.
func (dm *DiscordManager) Connect(callback func()) {
	found := false
	// Once the connection is ready, the client is prepared and the daemon multiplexer is started
	dm.DiscordClient.AddHandler(func(s *discordgo.Session, event *discordgo.Ready) {
		dm.DiscordClient.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
			found = false
			for _, chn := range dm.channels {
				if chn == m.ChannelID {
					found = true
				}
			}
			if found {
				dm.Receiver <- Message{
					User:   m.Author.Username,
					Text:   m.Content,
					Origin: m.ChannelID,
				}
			}
		})
		go dm.Daemon()
		callback()
	})

	err := dm.DiscordClient.Open()
	if err != nil {
		logger.Fatal("discord client connection error", zap.Error(err))
	}
}

// Daemon passes messages between rediscord and the Discord API
func (dm *DiscordManager) Daemon() {
	for msg := range dm.Sender {
		_, err := dm.DiscordClient.ChannelMessageSend(msg.Destination, fmt.Sprintf("%s: %s", msg.User, msg.Text))
		if err != nil {
			logger.Error("ChannelMessageSend failed", zap.Error(err))
		}
	}
}
