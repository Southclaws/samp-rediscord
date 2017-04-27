package main

import (
	"context"

	"github.com/Southclaws/samp-go"
)

// App stores and controls program state
type App struct {
	config Config
	sc     *sampgo.Client
	dm     *DiscordManager
	ctx    context.Context
	cancel context.CancelFunc
}

// Start fires up listeners and daemons then blocks until fatal error
func Start(config Config) {
	app := App{
		config: config,
	}

	app.ctx, app.cancel = context.WithCancel(context.Background())

	app.sc = sampgo.NewSAMPClient(config.RedisHost, config.RedisPort, config.RedisAuth, config.RedisDBID, config.Domain)
	app.dm = NewDiscordManager(&app)

	<-app.ctx.Done()

	logger.Info("shutting down")
	app.dm.Close()
}
