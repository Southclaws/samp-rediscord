package main

import (
	"container/ring"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

// Router handles routing messages between Discord channels and game chats
type Router struct {
	app               *App
	dcSender          func(message, channel string) error // discord sender
	gsSender          func(message, channel string) error // gameserver sender
	MessagesToDiscord chan Message                        // queue of messages waiting to be sent to Discord
	MessagesToGame    chan Message                        // queue of messages waiting to be sent to the game server
	MessageHistory    *ring.Ring                          // ring-list of the last n messages processed to block duplicates
}

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

// NewRouter creates a new router, connects to Discord/Redis and starts routing
func NewRouter(app *App, dcSender func(string, string) error, gsSender func(string, string) error) *Router {
	router := &Router{
		app:               app,
		dcSender:          dcSender,
		gsSender:          gsSender,
		MessagesToDiscord: make(chan Message),
		MessagesToGame:    make(chan Message),
		MessageHistory:    ring.New(32),
	}

	return router
}

// Start adds a handler for messages from Discord and a handler for messages from the game then fires up the app.
// blocks until fatal error
func (r *Router) Start() {
	r.AddDiscordHandler()
	r.AddGameHandler()
	r.Daemon()
}

// Daemon is a simple mutiplex select across four processes:
// - messages coming from Discord and waiting to get sent to the game
// - messages coming from the game and waiting to get sent to Discord
// - messages being consumed and sent to the game
// - messages being consumed and sent to Discord
// This is a blocking function and exits on fatal error.
func (r *Router) Daemon() {
	for {
		select {
		case message := <-r.MessagesToDiscord:
			logger.Debug("send to discord",
				zap.String("username", message.User),
				zap.String("message", message.Text),
				zap.String("destination", message.Destination))

			r.MessageHistory.Value = message.Text
			r.MessageHistory.Next()
			raw := fmt.Sprintf("%s: %s", message.User, message.Text)

			err := r.dcSender(message.Destination, raw)
			if err != nil {
				logger.Warn("discord send error",
					zap.Error(err))
			}

		case message := <-r.MessagesToGame:
			logger.Info("send to game",
				zap.String("username", message.User),
				zap.String("message", message.Text),
				zap.String("destination", message.Destination))

			r.MessageHistory.Value = message.Text
			r.MessageHistory.Next()
			raw := fmt.Sprintf("%s: %s", message.User, message.Text)

			err := r.gsSender(message.Destination, raw)
			if err != nil {
				logger.Warn("rm.SendMessage failed", zap.Error(err))
			}
		}
	}
}

// AddDiscordHandler uses the Discord client gateway event system to bind the Discord MessageCreate
// event to a function that consumes a message and places it on the message queue for being sent to
// the game.
func (r *Router) AddDiscordHandler() {
	dm.DiscordClient.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.Bot {
			return
		}

		Destination := dm.GetOutgoingKeyFromChannelID(m.Message.ChannelID)

		if Destination != "" {
			duplicate := false
			dm.MessageHistory.Do(func(i interface{}) {
				if i == m.Message.Content {
					duplicate = true
				}
			})

			if !duplicate {
				logger.Debug("received non duplicate message from discord",
					zap.String("user", m.Message.Author.Username),
					zap.String("message", m.Message.Content),
					zap.String("destination", Destination))

				dm.MessagesToGame <- Message{m.Message.Author.Username, m.Message.Content, Destination}
			}
		}
	})
}

// AddGameHandler uses the samp-go library to bind to a Redis list that contains chat messages sent
// from the game and places them on a message queue to be sent to the Discord channel.
func (r *Router) AddGameHandler() {
	for _, bind := range dm.app.config.DiscordBinds {
		logger.Debug("adding game handler", zap.String("input_queue", bind.InputQueue), zap.String("output_queue", bind.OutputQueue), zap.String("discord_channel", bind.DiscordChannel))
		dm.app.sc.BindMessage(dm.GetFullRedisKey(bind.InputQueue), func(message string) {
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
				logger.Debug("received non duplicate message from game",
					zap.String("user", split[0]),
					zap.String("message", split[1]),
					zap.String("input_queue", bind.InputQueue),
					zap.String("output_queue", bind.OutputQueue),
					zap.String("discord_channel", bind.DiscordChannel))

				dm.MessagesToDiscord <- Message{split[0], split[1], bind.DiscordChannel}
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
