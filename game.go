package main

import (
	"strings"

	sampgo "github.com/Southclaws/samp-go"
	"go.uber.org/zap"
)

// GameManager holds program state and shares access to resources
type GameManager struct {
	app        *App
	GameClient *sampgo.Client
	sender     chan Message
	receiver   chan Message
}

// NewGameManager sets up a Game client and prepares it for starting
func NewGameManager(app *App, channels []string) *GameManager {
	gm := GameManager{
		app: app,
	}

	gm.GameClient = sampgo.NewSAMPClient(
		app.config.RedisHost,
		app.config.RedisPort,
		app.config.RedisAuth,
		app.config.RedisDBID,
		app.config.Domain)

	for _, channel := range channels {
		gm.GameClient.BindMessage(gm.app.GetFullRedisKey(channel), func(message string) {
			split := strings.SplitN(message, ":", 2)

			if len(split) != 2 {
				logger.Warn("received from game: message malformed, no colon delimiter",
					zap.String("message", message))
				return
			}

			logger.Debug("received message from game",
				zap.String("user", split[0]),
				zap.String("message", split[1]),
				zap.String("origin", channel))

			gm.receiver <- Message{
				User:   split[0],
				Text:   split[1],
				Origin: channel,
			}
		})
	}

	return &gm
}

// Send simply sends `message` to `channel`
func (gm *GameManager) Send(message Message) {
}

// Receive returns a channel to send messages
func (gm *GameManager) Receive() <-chan Message {
	return gm.receiver
}

// Daemon passes messages between rediscord and the Discord API
func (gm *GameManager) Daemon() {
}