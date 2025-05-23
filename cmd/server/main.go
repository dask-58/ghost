package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dask-58/ghost/internal/matchmaker"
	"github.com/dask-58/ghost/internal/queue"
)

type PlayerRequest struct {
	PlayerId string `json:"playerId"`
}

var loggerAddedSinceLastLog int64
var apiAddedSinceLastRead int64

type Server struct {
	playerQueue     *queue.PlayerQueue
	matchmaker      *matchmaker.Matchmaker
	logger          *log.Logger
	statusTemplate  *template.Template
	startTime       time.Time
}

type StatusData struct {
	QueueSize         int    `json:"queueSize"`
	ActiveLobbies     int    `json:"activeLobbies"`
	ServerUptime      string `json:"serverUptime"`
	RecentlyJoined    int64  `json:"recentlyJoined"`
	CurrentServerTime string `json:"currentServerTime"`
}

func NewServer(pq *queue.PlayerQueue, m_maker *matchmaker.Matchmaker, lgr *log.Logger) (*Server, error) {
	templatePath := "cmd/templates/status.html"
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status template: %w", err)
	}
	return &Server{
		playerQueue:    pq,
		matchmaker:     m_maker,
		logger:         lgr,
		statusTemplate: tmpl,
		startTime:      time.Now(),
	}, nil
}

func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	fmt.Fprintln(w, "Ghost Matchmaking Server!")
}

func (s *Server) joinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST methods allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request: could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req PlayerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.logger.Printf("Error unmarshaling JSON for player %s: %v", req.PlayerId, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.PlayerId == "" {
		s.logger.Println("Attempt to join with empty playerId")
		http.Error(w, "PlayerId cannot be empty", http.StatusBadRequest)
		return
	}

	player := queue.Player{ID: req.PlayerId}
	added := s.playerQueue.JoinQueue(player)

	if added {
		atomic.AddInt64(&loggerAddedSinceLastLog, 1) // For logger summary
		atomic.AddInt64(&apiAddedSinceLastRead, 1)  // For API status
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Player %s added to queue", req.PlayerId)
		s.logger.Printf("Player %s joined queue. Current queue size: %d", req.PlayerId, s.playerQueue.Size())
	} else {
		s.logger.Printf("Duplicate player %s tried to join", req.PlayerId)
		http.Error(w, "Player already in queue", http.StatusConflict)
	}
}

func (s *Server) getStatusData() StatusData {
	uptime := time.Since(s.startTime).Round(time.Second)
	// For recently joined, read and reset the counter for the API.
	// This means "recently" is "since the last time this API was called".
	// If the API is called every 2s from JS, this is effectively "players in last 2s".
	recent := atomic.SwapInt64(&apiAddedSinceLastRead, 0)

	return StatusData{
		QueueSize:         s.playerQueue.Size(),
		ActiveLobbies:     s.matchmaker.GetActiveLobbyCount(),
		ServerUptime:      uptime.String(),
		RecentlyJoined:    recent,
		CurrentServerTime: time.Now().Format(time.RFC1123),
	}
}

// HTML status page
func (s *Server) statusPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET methods allowed", http.StatusMethodNotAllowed)
		return
	}
	data := s.getStatusData()
	err := s.statusTemplate.Execute(w, data)
	if err != nil {
		s.logger.Printf("Error executing status template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) statusApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET methods allowed", http.StatusMethodNotAllowed)
		return
	}
	data := s.getStatusData()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Printf("Error encoding status data to JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func main() {
	logger := log.New(os.Stdout, "SERVER: ", log.LstdFlags|log.Lmicroseconds)
	playerQueue := queue.NewQueue()

	matchmakerConfig := matchmaker.Config{
		LobbySize: 100,
		Frequency: 2 * time.Second,
	}

	ghostMatchmaker := matchmaker.NewMatchmaker(playerQueue, logger, matchmakerConfig)

	serverInstance, err := NewServer(playerQueue, ghostMatchmaker, logger)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		summaryTicker := time.NewTicker(2 * time.Second)
		defer summaryTicker.Stop()
		for range summaryTicker.C {
			count := atomic.SwapInt64(&loggerAddedSinceLastLog, 0)
			currentQueueSize := playerQueue.Size()
			activeLobbyCount := ghostMatchmaker.GetActiveLobbyCount()

			if count > 0 || currentQueueSize > 0 || activeLobbyCount > 0 {
				elapsed := time.Since(serverInstance.startTime).Round(time.Second)
				logger.Printf("[Summary after %v] %d players added in last 2s (logger count). Queue size: %d. Active Lobbies: %d\n", elapsed, count, currentQueueSize, activeLobbyCount)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serverInstance.rootHandler)
	mux.HandleFunc("/join", serverInstance.joinHandler)
	mux.HandleFunc("/status", serverInstance.statusPageHandler) // Serves HTML
	mux.HandleFunc("/api/status", serverInstance.statusApiHandler) // Serves JSON

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go ghostMatchmaker.Start(ctx)

	go func() {
		<-ctx.Done()
		logger.Println("Received shutdown signal. Shutting down HTTP server...")

		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Printf("HTTP server Shutdown error: %v", err)
		}
		logger.Println("HTTP Server shutdown complete.")
	}()

	logger.Println("Server running on http://localhost:8080")
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatalf("HTTP server ListenAndServe error: %v", err)
	}

	logger.Println("Application exiting.")
}
