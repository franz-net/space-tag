package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

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

// allowedOrigins is populated from the ALLOWED_ORIGINS env var (comma-separated).
// If empty or unset, all origins are allowed (development mode).
var allowedOrigins []string

func initAllowedOrigins() {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return
	}
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedOrigins = append(allowedOrigins, o)
		}
	}
	log.Printf("Allowed WebSocket origins: %v", allowedOrigins)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		if len(allowedOrigins) == 0 {
			return true // dev mode — allow all
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		log.Printf("Rejected WebSocket from origin: %s", origin)
		return false
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
	staticDir := flag.String("static", "./client/out", "directory of static client files to serve (set to empty string to disable)")
	flag.Parse()

	if Debug {
		log.Println("Debug logging ENABLED")
	}

	initAllowedOrigins()

	hub := newHub()
	go hub.run()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Serve the static Next.js export at /. The dev server (npm run dev)
	// owns this in development; in production the Go binary serves both
	// the WebSocket and the static client on the same port.
	if *staticDir != "" {
		if _, err := os.Stat(*staticDir); err == nil {
			fs := http.FileServer(http.Dir(*staticDir))
			mux.Handle("/", fs)
			log.Printf("Serving static client from %s", *staticDir)
		} else {
			log.Printf("Static client directory not found at %s — only /ws and /health will respond", *staticDir)
		}
	}

	// Railway sets PORT dynamically. Fall back to 8080 in dev.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port
	log.Println("SpaceTag server starting on", addr)
	logLANAddresses(port)
	if err := http.ListenAndServe(addr, mux); err != nil {
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
