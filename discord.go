package main

import (
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

// DiscordManager holds program state and shares access to resources
type DiscordManager struct {
	app           *App
	DiscordClient *discordgo.Session
}

// NewDiscordManager sets up a Discord client and prepares it for starting
func NewDiscordManager(app *App) *DiscordManager {
	var err error

	dm := DiscordManager{
		app: app,
	}

	dm.app.config = app.config

	dm.DiscordClient, err = discordgo.New("Bot " + dm.app.config.DiscordToken)
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
	// Once the connection is ready, the client is prepared and the daemon multiplexer is started
	dm.DiscordClient.AddHandler(func(s *discordgo.Session, event *discordgo.Ready) {
		callback()
	})

	err := dm.DiscordClient.Open()
	if err != nil {
		logger.Fatal("discord client connection error", zap.Error(err))
	}
}

// Send simply sends `message` to `channel`
func (dm *DiscordManager) Send(message, channel string) error {
	_, err := dm.DiscordClient.ChannelMessageSend(channel, message)
	if err != nil {
		return err
	}
}
