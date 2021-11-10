package main

import (
    "fmt"
    "github.com/gin-gonic/gin"
    "io"
    "time"
)


type EventName int
const (
    KeepAlive EventName = iota
    SessionCreated
    SessionDeleted
    SessionUpdated
)


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



// curl -N http://localhost:3000/subscription/topic%20A
func SetupSSERouter(router gin.IRouter, path string) {
    broker = NewBroker()
    // Set it running - listening and broadcasting events
    go broker.Listen()
    go broker.KeepAlive()

    router.GET(path, broker.ServeHTTP)
}

func BroadcastNotifier(url string, name EventName, payload interface{})  {
    if broker == nil {
        return
    }

    fmt.Printf("Emitting: %s::%s\n", url, name)
    broker.Notifier <- NotificationEvent{
        URL: url,
        Name:   name,
        Payload:   payload,
    }
}





// the amount of time to wait when pushing a message to
// a slow client or a client that closed after `range clients` started.
const patience time.Duration = time.Second * 1

type (
    NotificationEvent struct {
        URL string
        Name EventName
        Payload   interface{}
    }

    NotifierChan chan NotificationEvent

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
var broker *Broker

func NewBroker() (broker *Broker) {
    // Instantiate a broker
    return &Broker{
        Notifier:       make(NotifierChan, 1),
        newClients:     make(chan NotifierChan),
        closingClients: make(chan NotifierChan),
        clients:        make(map[NotifierChan]struct{}),
    }
}

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
        fmt.Printf("[%s] Closing DOWN\n", c.ClientIP() )
        broker.closingClients <- messageChan
    }()

    c.Stream(func(w io.Writer) bool {
        // Emit Server Sent Events compatible
        event := <-messageChan

        fmt.Printf("Client URL: %s event.URL: %s\n", url, event.URL)

        switch event.URL {

        case "*", url:
            fmt.Printf("Sending SSEvent: %s::%s\n", event.URL, event.Name)

            c.SSEvent( event.Name.String(), event.Payload)
            break
        }
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

            fmt.Printf("broker.Notifier: %v\n", event)
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

func (broker *Broker) KeepAlive() {

    for {
        event := NotificationEvent{
            "*",KeepAlive, "",
        }
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