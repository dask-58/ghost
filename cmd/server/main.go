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

var addedSinceLastLog int64 // counter

type Server struct {
	playerQueue 	*queue.PlayerQueue
	matchmaker 		*matchmaker.Matchmaker
	logger 			*log.Logger
	statusTemplate 	*template.Template
}

func NewServer(pq *queue.PlayerQueue, m_maker *matchmaker.Matchmaker,lgr *log.Logger) (*Server, error) {
	templatePath := "cmd/templates/status.html"
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse status template: %w", err)
	}
	return &Server {
		playerQueue: pq,
		matchmaker: m_maker,
		logger: lgr,
		statusTemplate: tmpl,
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
		atomic.AddInt64(&addedSinceLastLog, 1)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Player %s added to queue", req.PlayerId)
		s.logger.Printf("Player %s joined queue. Current queue size: %d", req.PlayerId, s.playerQueue.Size())
	} else {
		s.logger.Printf("Duplicate player %s tried to join", req.PlayerId)
		http.Error(w, "Player already in queue", http.StatusConflict)
	}
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET methods allowed", http.StatusMethodNotAllowed)
		return
	}
	data := struct {
		QueueSize     int
		ActiveLobbies int
	}{
		QueueSize:     s.playerQueue.Size(),
		ActiveLobbies: s.matchmaker.GetActiveLobbyCount(),
	}
	err := s.statusTemplate.Execute(w, data)
	if err != nil {
		s.logger.Printf("Error executing status template: %v", err)
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

	startTime := time.Now()

	go func() {
		summaryTicker := time.NewTicker(2 * time.Second)
		defer summaryTicker.Stop()
		for range summaryTicker.C {
			count := atomic.SwapInt64(&addedSinceLastLog, 0)
			currentQueueSize := playerQueue.Size()
			activeLobbyCount := ghostMatchmaker.GetActiveLobbyCount()

			if count > 0 || currentQueueSize > 0 || activeLobbyCount > 0 {
				elapsed := time.Since(startTime).Round(time.Second)
				logger.Printf("[Summary after %v] %d players added in last 2s. Queue size: %d. Active Lobbies (by Matchmaker): %d\n", elapsed, count, currentQueueSize, activeLobbyCount)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serverInstance.rootHandler)
	mux.HandleFunc("/join", serverInstance.joinHandler)
	mux.HandleFunc("/status", serverInstance.statusHandler)

	httpServer := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go ghostMatchmaker.Start(ctx)

	// Graceful shutdown
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
