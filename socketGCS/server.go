package main

import (
	"log"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/kr/pretty"
)

type Message struct {
	Name        string `json:"name"`
	Number      int    `json:"number"`
	TestMessage string `json:"message"`
}

var templates *template.Template

func indexGetHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", nil)
}

func main() {
	// flag.Parse()

	templates = template.Must(template.ParseGlob("index.html"))

	h := newHub()

	r := mux.NewRouter()
	r.HandleFunc("/", indexGetHandler)
	r.HandleFunc("/ws", h.ServeHTTP)

	http.Handle("/", r) //use the mux router as the default handler

	log.Printf("serving on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func homeHandler(tpl *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tpl.Execute(w, r)
	})
}

// ###############################################################################
// ###############################################################################

type connection struct {
	// Buffered channel of outbound messages.
	send chan Message

	// The hub.
	h *hub
}

type hub struct {
	// the mutex to protect connections
	connectionsMx sync.RWMutex

	// Registered connections.
	connections map[*connection]struct{}

	// Inbound messages from the connections.
	broadcast chan Message

	logMx sync.RWMutex
	log   [][]byte
}

func newHub() *hub {
	h := &hub{
		connectionsMx: sync.RWMutex{},
		broadcast:     make(chan Message),
		connections:   make(map[*connection]struct{}),
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

	c := &connection{send: make(chan Message), h: wsh}
	c.h.addConnection(c)
	defer c.h.removeConnection(c)

	var wg sync.WaitGroup
	wg.Add(2)
	go c.writer(&wg, wsConn)
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
		pretty.Println(message.Name, message.Number, message.TestMessage)

		c.h.broadcast <- message
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
