package main

import (
	"flag"
	"log"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Debug controls verbose per-event logging across the server.
// Enable with `go run . -debug`.
var Debug bool

func dbg(format string, args ...interface{}) {
	if Debug {
		log.Printf("[DBG] "+format, args...)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins in dev
	},
}

func generateID() string {
	return uuid.New().String()[:8]
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
		id:   generateID(),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func main() {
	flag.BoolVar(&Debug, "debug", false, "enable verbose debug logging")
	flag.Parse()

	if Debug {
		log.Println("Debug logging ENABLED")
	}

	hub := newHub()
	go hub.run()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	const addr = "0.0.0.0:8080"
	log.Println("SpaceTag server starting on", addr)
	logLANAddresses("8080")
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

// logLANAddresses prints reachable URLs at startup so it's obvious what
// IP to test from a phone or other device on the same network.
func logLANAddresses(port string) {
	log.Println("Reachable WebSocket endpoints:")
	log.Printf("  ws://localhost:%s/ws  (this machine only)", port)

	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil {
				continue
			}
			log.Printf("  ws://%s:%s/ws  (LAN — open port %s in your firewall)", ip.String(), port, port)
		}
	}
}
