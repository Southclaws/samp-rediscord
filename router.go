package main

import (
	"container/ring"

	"go.uber.org/zap"
)

// Router handles routing messages between Discord channels and game chats
type Router struct {
	app            *App
	dcSender       chan<- Message // discord sender
	gsSender       chan<- Message // gameserver sender
	dcReceiver     <-chan Message // discord receiver
	gsReceiver     <-chan Message // gameserver receiver
	MessageHistory *ring.Ring     // ring-list of the last n messages processed to block duplicates
}

// NewRouter creates a new router, connects to Discord/Redis and starts routing
func NewRouter(app *App, dcSender chan<- Message, gsSender chan<- Message, dcReceiver <-chan Message, gsReceiver <-chan Message) *Router {
	router := &Router{
		app:            app,
		dcSender:       dcSender,
		gsSender:       gsSender,
		dcReceiver:     dcReceiver,
		gsReceiver:     gsReceiver,
		MessageHistory: ring.New(32),
	}

	return router
}

// Start adds a handler for messages from Discord and a handler for messages from the game then fires up the app.
// blocks until fatal error
func (r *Router) Start() {
	r.Daemon()
}

// Daemon is a simple mutiplex select across four processes:
// - messages coming from Discord and waiting to get sent to the game
// - messages coming from the game and waiting to get sent to Discord
// - messages being consumed and sent to the game
// - messages being consumed and sent to Discord
// This is a blocking function and exits on fatal error.
func (r *Router) Daemon() {
	var duplicate bool
	for {
		select {
		case message := <-r.gsReceiver:
			duplicate = false
			r.MessageHistory.Do(func(i interface{}) {
				if i == message.Text {
					duplicate = true
				}
			})
			if duplicate {
				continue
			}

			logger.Debug("send to discord",
				zap.String("username", message.User),
				zap.String("message", message.Text),
				zap.String("origin", message.Origin))

			r.MessageHistory.Value = message.Text
			r.MessageHistory.Next()

			r.dcSender <- message

		case message := <-r.dcReceiver:
			duplicate = false
			r.MessageHistory.Do(func(i interface{}) {
				if i == message.Text {
					duplicate = true
				}
			})
			if duplicate {
				continue
			}

			logger.Info("send to game",
				zap.String("username", message.User),
				zap.String("message", message.Text),
				zap.String("origin", message.Origin))

			r.MessageHistory.Value = message.Text
			r.MessageHistory.Next()

			r.gsSender <- message
		}
	}
}
