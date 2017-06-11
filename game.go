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
	Sender     chan Message
	Receiver   chan Message
}

// NewGameManager sets up a Game client and prepares it for starting
func NewGameManager(app *App, channels []string) *GameManager {
	gm := GameManager{
		app:      app,
		Sender:   make(chan Message),
		Receiver: make(chan Message),
	}

	gm.GameClient = sampgo.NewSAMPClient(
		app.config.RedisHost,
		app.config.RedisPort,
		app.config.RedisAuth,
		app.config.RedisDBID,
		app.config.Domain)

	for _, channel := range channels {
		gm.GameClient.BindMessage(gm.app.GetFullRedisKey(channel)+".outgoing", func(message string) {
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

			gm.Receiver <- Message{
				User:   split[0],
				Text:   split[1],
				Origin: channel,
			}
		})
	}

	return &gm
}

// Daemon passes messages between rediscord and the Discord API
func (gm *GameManager) Daemon() {
}
