package main

import (
	"context"
)

// App stores and controls program state
type App struct {
	config    Config
	gm        *GameManager
	dm        *DiscordManager
	router    *Router
	gsChannel map[string]string
	dcChannel map[string]string
	ctx       context.Context
	cancel    context.CancelFunc
}

// Message represents a message moving through this app
type Message struct {
	User        string // either in-game name or discord username
	Text        string // either game message text or discord message content
	Origin      string // either a game channel or discord channel
	Destination string // either a game channel or discord channel
}

// Bind represents a link between a Discord channel and a Redis queue
type Bind struct {
	DiscordChannel string `json:"discord_channel"`
	GameChannel    string `json:"game_channel"`
}

// Start fires up listeners and daemons then blocks until fatal error
func Start(config Config) {
	app := App{
		config:    config,
		gsChannel: make(map[string]string),
		dcChannel: make(map[string]string),
	}

	app.ctx, app.cancel = context.WithCancel(context.Background())

	channels := []string{}
	for _, bind := range app.config.DiscordBinds {
		channels = append(channels, bind.GameChannel)
		app.gsChannel[bind.DiscordChannel] = bind.GameChannel
		app.dcChannel[bind.GameChannel] = bind.DiscordChannel
	}

	app.gm = NewGameManager(&app, channels)
	app.dm = NewDiscordManager(&app, channels)
	app.dm.Connect(func() {
		app.router = NewRouter(&app, app.dm.Sender, app.gm.Sender, app.dm.Receiver, app.gm.Receiver)
		app.router.Start()
	})

	<-app.ctx.Done()

	logger.Info("shutting down")
	app.dm.Close()
}

// GetOutgoingKeyFromChannelID takes a Discord channel ID and returns a Redis queue if it is
// associated with one, otherwise returns an empty string.
func (app App) GetOutgoingKeyFromChannelID(channel string) (string, bool) {
	result, ok := app.gsChannel[channel]
	return app.GetFullRedisKey(result), ok
}

// GetChannelFromInQueue takes a Redis queue name and returns a Discord channel ID if it is
// associated with one, otherwise returns an empty string.
func (app App) GetChannelFromInQueue(queue string) (string, bool) {
	result, ok := app.dcChannel[queue]
	return result, ok
}

// GetFullRedisKey returns a full redis key for naming queues in the form: myserver.rediscord.queue
func (app App) GetFullRedisKey(name string) string {
	return app.config.Domain + ".rediscord." + name
}
