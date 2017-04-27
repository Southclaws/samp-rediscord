package main

import (
	"container/ring"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

// Bind represents a link between a Discord channel and a Redis queue
type Bind struct {
	DiscordChannel string `json:"channel_id"`
	InputQueue     string `json:"input_queue"`
	OutputQueue    string `json:"output_queue"`
}

// Message represents a message moving through this app
type Message struct {
	User string
	Text string
	// when in the `MessagesToDiscord` queue, this represents a Discord channel
	// and when in the `MessagesToGame` queue, it represents a queue name.
	Destination string
}

// DiscordManager holds program state and shares access to resources
type DiscordManager struct {
	app               *App
	DiscordClient     *discordgo.Session
	MessagesToDiscord chan Message // queue of messages waiting to be sent to Discord
	MessagesToGame    chan Message // queue of messages waiting to be sent to the game server
	MessageHistory    *ring.Ring   // ring-list of the last n messages processed to block duplicates
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

	dm.MessagesToDiscord = make(chan Message)
	dm.MessagesToGame = make(chan Message)
	dm.MessageHistory = ring.New(32)

	dm.Connect()

	return &dm
}

// Close cleans up and shuts down the Discord client
func (dm *DiscordManager) Close() error {
	return dm.DiscordClient.Close()
}

// Connect prepares the Discord client library and connects to the API.
func (dm *DiscordManager) Connect() {
	// Once the connection is ready, the client is prepared and the daemon multiplexer is started
	dm.DiscordClient.AddHandler(func(s *discordgo.Session, event *discordgo.Ready) {
		dm.Prepare()
		dm.Daemon()
	})

	err := dm.DiscordClient.Open()
	if err != nil {
		logger.Fatal("discord client connection error", zap.Error(err))
	}
}

// Prepare adds a handler for messages from Discord and a handler for messages from the game.
func (dm *DiscordManager) Prepare() {
	dm.AddDiscordHandler()
	dm.AddGameHandler()
}

// Daemon is a simple mutiplex select across four processes:
// - messages coming from Discord and waiting to get sent to the game
// - messages coming from the game and waiting to get sent to Discord
// - messages being consumed and sent to the game
// - messages being consumed and sent to Discord
// This is a blocking function and exits on fatal error.
func (dm *DiscordManager) Daemon() {
	for {
		select {
		case message := <-dm.MessagesToDiscord:
			logger.Debug("send to discord",
				zap.String("username", message.User),
				zap.String("message", message.Text),
				zap.String("Destination", message.Destination))

			dm.MessageHistory.Value = message.Text
			dm.MessageHistory.Next()
			Text := fmt.Sprintf("%s: %s", message.User, message.Text)

			sr, err := dm.DiscordClient.ChannelMessageSend(message.Destination, Text)
			if err != nil {
				logger.Warn("ChannelMessageSend error",
					zap.Error(err),
					zap.Any("sr", sr))
			}

		case message := <-dm.MessagesToGame:
			logger.Info("send to game",
				zap.String("username", message.User),
				zap.String("message", message.Text),
				zap.String("Destination", message.Destination))

			dm.MessageHistory.Value = message.Text
			dm.MessageHistory.Next()
			raw := fmt.Sprintf("%s:%s", message.User, message.Text)

			err := dm.app.sc.SendMessage(message.Destination, raw)
			if err != nil {
				logger.Warn("rm.SendMessage failed", zap.Error(err))
			}
		}
	}
}

// AddDiscordHandler uses the Discord client gateway event system to bind the Discord MessageCreate
// event to a function that consumes a message and places it on the message queue for being sent to
// the game.
func (dm DiscordManager) AddDiscordHandler() {
	dm.DiscordClient.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		Destination := dm.GetOutgoingKeyFromChannelID(m.Message.ChannelID)

		if Destination != "" {
			split := strings.SplitN(m.Message.Content, ": ", 2)
			duplicate := false
			if len(split) >= 2 {
				dm.MessageHistory.Do(func(i interface{}) {
					if i == split[1] {
						duplicate = true
					}
				})
			}

			if !duplicate {
				dm.MessagesToGame <- Message{m.Message.Author.Username, m.Message.Content, Destination}
			}
		}
	})
}

// AddGameHandler uses the samp-go library to bind to a Redis list that contains chat messages sent
// from the game and places them on a message queue to be sent to the Discord channel.
func (dm DiscordManager) AddGameHandler() {
	for i := range dm.app.config.DiscordBinds {
		dm.app.sc.BindMessage(dm.GetFullRedisKey(dm.app.config.DiscordBinds[i].InputQueue), func(message string) {
			split := strings.SplitN(message, ":", 2)

			if len(split) != 2 {
				logger.Warn("received from game: message malformed, no colon delimiter",
					zap.String("message", message))
				return
			}

			duplicate := false
			dm.MessageHistory.Do(func(i interface{}) {
				if i == split[1] {
					duplicate = true
				}
			})

			if !duplicate {
				dm.MessagesToDiscord <- Message{split[0], split[1], dm.app.config.DiscordBinds[i].DiscordChannel}
			}
		})
	}
}

// GetOutgoingKeyFromChannelID takes a Discord channel ID and returns a Redis queue if it is
// associated with one, otherwise returns an empty string.
func (dm DiscordManager) GetOutgoingKeyFromChannelID(channel string) string {
	for i := range dm.app.config.DiscordBinds {
		if channel == dm.app.config.DiscordBinds[i].DiscordChannel {
			return dm.GetFullRedisKey(dm.app.config.DiscordBinds[i].OutputQueue)
		}
	}
	return ""
}

// GetChannelFromInQueue takes a Redis queue name and returns a Discord channel ID if it is
// associated with one, otherwise returns an empty string.
func (dm DiscordManager) GetChannelFromInQueue(queue string) string {
	for i := range dm.app.config.DiscordBinds {
		if queue == dm.GetFullRedisKey(dm.app.config.DiscordBinds[i].InputQueue) {
			return dm.app.config.DiscordBinds[i].DiscordChannel
		}
	}
	return ""
}

// GetFullRedisKey returns a full redis key for naming queues in the form: myserver.rediscord.queue
func (dm DiscordManager) GetFullRedisKey(name string) string {
	return dm.app.config.Domain + ".rediscord." + name
}
