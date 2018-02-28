package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Message struct {
	PixelX int64 `json:"pixelX"`
	PixelY int64 `json:"pixelY"`
}

type MessageProcessed struct {
	Messageprocessed string `json:"messageprocessed"`
}

var templates *template.Template

//to be globally accessable by multiple routes
var client *redis.Client

func main() {
	// flag.Parse()

	Init()

	templates = template.Must(template.ParseGlob("index.html"))

	h := newHub()

	r := mux.NewRouter()
	r.HandleFunc("/", indexGetHandler)
	r.HandleFunc("/ws", h.ServeHTTP)

	http.Handle("/", r) //use the mux router as the default handler

	log.Printf("serving on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

//Init serves clients from redis ??? not sure advantage over direct
func Init() {
	client = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", //default port of redis-server; lo-host when same machine
	})
}

func indexGetHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", nil)
}

// ###############################################################################
// ###############################################################################

type connection struct {
	// unBuffered channel of outbound messages.
	send chan MessageProcessed

	// The hub.
	h *hub
}

type hub struct {
	// the mutex to protect connections
	connectionsMx sync.RWMutex

	// Registered connections.
	connections map[*connection]struct{}

	// Inbound messages from the connections.
	broadcast chan MessageProcessed

	process chan Message

	// logMx sync.RWMutex
	// log [][]byte
}

func newHub() *hub {
	h := &hub{
		connectionsMx: sync.RWMutex{},
		connections:   make(map[*connection]struct{}),
		broadcast:     make(chan MessageProcessed),
		process:       make(chan Message),
	}

	go func() {
		for {
			msg := <-h.broadcast
			h.connectionsMx.RLock()
			for connections := range h.connections {
				select {
				case connections.send <- msg: //send msg to connection type on connection channel
				// stop trying to send to this connection after trying for 1 second.
				// if we have to stop, it means that a reader died so remove the connection also.
				case <-time.After(1 * time.Second):
					log.Printf("shutting down connection %s", connections)
					h.removeConnection(connections)
				}
			}
			h.connectionsMx.RUnlock()
		}
	}()
	return h
}

func (h *hub) addConnection(conn *connection) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	h.connections[conn] = struct{}{}
}

func (h *hub) removeConnection(conn *connection) {
	h.connectionsMx.Lock()
	defer h.connectionsMx.Unlock()
	if _, ok := h.connections[conn]; ok {
		delete(h.connections, conn)
		close(conn.send)
	}
}

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

func (wsh *hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	wsConn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("error upgrading %s", err)
		return
	}

	c := &connection{send: make(chan MessageProcessed), h: wsh}
	c.h.addConnection(c)
	defer c.h.removeConnection(c)

	var wg sync.WaitGroup
	wg.Add(3)
	go c.writer(&wg, wsConn)
	go c.process(&wg, wsConn)
	go c.reader(&wg, wsConn)
	wg.Wait()
	wsConn.Close()
}

// ###############################################################################
// ###############################################################################

func (c *connection) reader(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	defer wg.Done()

	//read message from clients
	for {
		var message Message
		err := wsConn.ReadJSON(&message)
		if err != nil {
			break
		}
		c.h.process <- message
	}
}

func (c *connection) writer(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	defer wg.Done()
	for message := range c.send {
		err := wsConn.WriteJSON(message)
		if err != nil {
			break
		}
	}
}

// ###############################################################################
// ###############################################################################

func (c *connection) process(wg *sync.WaitGroup, wsConn *websocket.Conn) {
	defer wg.Done()
	for {

		message := <-c.h.process

		mID, err := client.Incr("message:next-id").Result() //assign id to assignment to redis
		if err != nil {
			return
		}
		key := fmt.Sprintf("message:%d", mID) //prefix id to create distinct namespace

		var processedMessage MessageProcessed

		processedMessage.Messageprocessed = concatenate(message)

		var m = make(map[string]interface{})
		m["pixelX"] = message.PixelX
		m["pixelY"] = message.PixelY

		client.HMSet(key, m)
		client.LPush("id", key)

		// messageList, err := client.LRange(key, 0, client.LLen("id").Val()).Result()
		// if err != nil {
		// 	return
		// }

		// var messageDB Message

		// for _, i := range messageList {
		// 	messageDB.Name = client.HMGet(i, "name").String()
		// 	messageDB.Number = 0
		// 	messageDB.TestMessage = client.HMGet(i, "testMessage").String()
		// }

		c.h.broadcast <- processedMessage
	}

}

func concatenate(message Message) string {

	messageString := fmt.Sprintf("%d%d", message.PixelX, message.PixelY)

	return messageString
}

// ###############################################################################
// ###############################################################################
