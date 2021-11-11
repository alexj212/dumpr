package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"time"
)

// EventName enum to define events names - need to be used on client to subscribe with new EventSource in js.
type EventName int

const (
	// KeepAlive keep alive to be sent peridocially by background routine
	KeepAlive EventName = iota

	// SessionCreated event when a new session is created on server, payload is the session info
	SessionCreated
	// SessionDeleted event when a new session is deleted on server, payload is the session key
	SessionDeleted
	// SessionUpdated event when a session is updated on server, payload is the session info
	SessionUpdated
)

// String function to clean event name
func (s EventName) String() string {
	switch s {
	case KeepAlive:
		return "keepalive"
	case SessionCreated:
		return "sessionCreated"
	case SessionDeleted:
		return "sessionDeleted"
	case SessionUpdated:
		return "sessionUpdated"
	}
	return "unknown"
}

// SetupSSERouter setup a broker for a path
func SetupSSERouter(router gin.IRouter, path string) {
	broker = NewBroker()
	// Set it running - listening and broadcasting events
	go broker.Listen()
	go broker.KeepAlive()

	router.GET(path, broker.ServeHTTP)
}

// Broadcast an event to clients listening
func Broadcast(name EventName, payload interface{}) {
	if broker == nil {
		return
	}

	// fmt.Printf("Emitting: %s::%s\n", url, name)
	broker.Notifier <- NotificationEvent{
		Name:    name,
		Payload: payload,
	}
}

// patience the amount of time to wait when pushing a message to
// a slow client or a client that closed after `range clients` started.
const patience time.Duration = time.Second * 1

type (

	// NotificationEvent event name and payload to be sent back to the client.
	NotificationEvent struct {
		Name    EventName
		Payload interface{}
	}

	// NotifierChan channel for events to be passed
	NotifierChan chan NotificationEvent

	// Broker the main broker struct
	Broker struct {
		//path string
		// Events are pushed to this channel by the main events-gathering routine
		Notifier NotifierChan

		// New client connections
		newClients chan NotifierChan

		// Closed client connections
		closingClients chan NotifierChan

		// Client connections registry
		clients map[NotifierChan]struct{}
	}
)

// String nice display of NotificationEvent
func (e NotificationEvent) String() interface{} {
	return fmt.Sprintf("NotificationEvent{ Name: %s Payload: %+v}", e.Name.String(), e.Payload)
}

var broker *Broker

// NewBroker create broker for sse events to be sent to clients
func NewBroker() (broker *Broker) {
	// Instantiate a broker
	return &Broker{
		Notifier:       make(NotifierChan, 1),
		newClients:     make(chan NotifierChan),
		closingClients: make(chan NotifierChan),
		clients:        make(map[NotifierChan]struct{}),
	}
}

// ServeHTTP main handler of clients.
func (broker *Broker) ServeHTTP(c *gin.Context) {
	url := c.FullPath()
	fmt.Printf("[%s] Requested topic: %s\n", c.ClientIP(), url)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Each connection registers its own message channel with the Broker's connections registry
	messageChan := make(NotifierChan)

	// Signal the broker that we have a new connection
	broker.newClients <- messageChan

	// Remove this client from the map of connected clients
	// when this handler exits.
	defer func() {
		fmt.Printf("[%s] Closing client down\n", c.ClientIP())
		broker.closingClients <- messageChan
	}()

	c.Stream(func(w io.Writer) bool {
		// Emit Server Sent Events compatible
		event := <-messageChan
		c.SSEvent(event.Name.String(), event.Payload)
		// Flush the data immediately instead of buffering it for later.
		c.Writer.Flush()
		return true
	})
}

// Listen for new notifications and redistribute them to clients
func (broker *Broker) Listen() {
	for {
		select {
		case s := <-broker.newClients:

			// A new client has connected.
			// Register their message channel
			broker.clients[s] = struct{}{}
			fmt.Printf("Client added. %d registered clients\n", len(broker.clients))
		case s := <-broker.closingClients:

			// A client has dettached and we want to
			// stop sending them messages.
			delete(broker.clients, s)
			fmt.Printf("Removed client. %d registered clients\n", len(broker.clients))
		case event := <-broker.Notifier:

			// fmt.Printf("broker.Notifier: %s\n", event.String())
			// We got a new event from the outside!
			// Send event to all connected clients
			for clientMessageChan := range broker.clients {
				select {
				case clientMessageChan <- event:
				case <-time.After(patience):
					fmt.Print("Skipping client.\n")
				}
			}
		}
	}
}

// KeepAlive routine to periodically message keep alive messages.
func (broker *Broker) KeepAlive() {

	for {
		event := NotificationEvent{KeepAlive, ""}

		for clientMessageChan := range broker.clients {
			select {
			case clientMessageChan <- event:
			case <-time.After(patience):
				fmt.Print("Skipping client.\n")
			}
		}

		time.Sleep(30 * time.Second)
	}
}
