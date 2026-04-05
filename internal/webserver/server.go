package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
)

// AppBridge defines the interface that the main App must implement
// for the web server to call its methods.
type AppBridge interface {
	SendMessage(userMsg string) string
	NewChat() string
	WebListChats() interface{}
	SwitchChat(id string) error
	DeleteChat(id string) error
	WebGetActiveMessages() interface{}
	GetActiveChatID() string
	WebCheckConnection() interface{}
	GetMemoryCount() int
	ToggleIncognito(enabled bool)
}

type Server struct {
	mu       sync.Mutex
	srv      *http.Server
	bridge   AppBridge
	assets   fs.FS
	running  bool
	port     int
	localIPs []string
}

func New(bridge AppBridge, assets fs.FS) *Server {
	return &Server{
		bridge: bridge,
		assets: assets,
	}
}

func (s *Server) Start(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running on port %d", s.port)
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/send", s.handleSend)
	mux.HandleFunc("/api/chats", s.handleChats)
	mux.HandleFunc("/api/chats/new", s.handleNewChat)
	mux.HandleFunc("/api/chats/switch", s.handleSwitchChat)
	mux.HandleFunc("/api/chats/delete", s.handleDeleteChat)
	mux.HandleFunc("/api/chats/active", s.handleActiveChat)
	mux.HandleFunc("/api/messages", s.handleMessages)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/incognito", s.handleIncognito)

	// Serve frontend static files
	fileServer := http.FileServer(http.FS(s.assets))
	mux.Handle("/", fileServer)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: corsMiddleware(mux),
	}
	s.port = port
	s.localIPs = getLocalIPs()

	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", port, err)
	}

	s.running = true
	go func() {
		log.Printf("Remote access server started on port %d", port)
		for _, ip := range s.localIPs {
			log.Printf("  → http://%s:%d", ip, port)
		}
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("Remote server error: %v", err)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.srv == nil {
		return nil
	}

	log.Println("Stopping remote access server...")
	err := s.srv.Shutdown(context.Background())
	s.running = false
	return err
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) GetPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

func (s *Server) GetAddresses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var addrs []string
	for _, ip := range s.localIPs {
		addrs = append(addrs, fmt.Sprintf("http://%s:%d", ip, s.port))
	}
	return addrs
}

// ─── Handlers ───────────────────────────────────────────────────

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	reply := s.bridge.SendMessage(req.Message)
	writeJSON(w, map[string]string{"reply": reply})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.bridge.WebListChats())
}

func (s *Server) handleNewChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := s.bridge.NewChat()
	writeJSON(w, map[string]string{"id": id})
}

func (s *Server) handleSwitchChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.SwitchChat(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleDeleteChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.bridge.DeleteChat(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleActiveChat(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"id": s.bridge.GetActiveChatID()})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	msgs := s.bridge.WebGetActiveMessages()
	if msgs == nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, msgs)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"connection":   s.bridge.WebCheckConnection(),
		"memory_count": s.bridge.GetMemoryCount(),
	})
}

func (s *Server) handleIncognito(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.bridge.ToggleIncognito(req.Enabled)
	writeJSON(w, map[string]string{"ok": "true"})
}

// ─── Helpers ────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getLocalIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"localhost"}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ips = append(ips, ipnet.IP.String())
		}
	}
	if len(ips) == 0 {
		ips = []string{"localhost"}
	}
	return ips
}
