package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dask-58/ghost/internal/queue"
)

type PlayerRequest struct {
	PlayerId string `json:"playerId"`
}

var addedSinceLastLog int64 // counter

type Server struct {
	playerQueue 	*queue.PlayerQueue
	logger 			*log.Logger
}

func NewServer(pq *queue.PlayerQueue, lgr *log.Logger) *Server {
	return &Server {
		playerQueue: pq,
		logger: lgr,
	}
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

func main() {
	logger := log.New(os.Stdout, "SERVER: ", log.LstdFlags|log.Lmicroseconds)
	playerQueue := queue.NewQueue()
	serverInstance := NewServer(playerQueue, logger)

	startTime := time.Now()

	go func ()  {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <- ticker.C: 
				count := atomic.SwapInt64(&addedSinceLastLog, 0)
				if count > 0 || playerQueue.Size() > 0 { // Log even if no new players but queue isn't empty
					elapsed := time.Since(startTime).Round(time.Second)
					logger.Printf("[Summary after %v] %d players added in last 2 seconds. Queue size: %d\n", elapsed, count, playerQueue.Size())
				}
			case <-time.After(5 * time.Second): // Fallback 
				return
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serverInstance.rootHandler)
	mux.HandleFunc("/join", serverInstance.joinHandler)

	httpServer := &http.Server{
		Addr: ".8080",
		Handler: mux,
	}

	// Graceful shutdown
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		receivedSignal := <-sigint
		logger.Printf("Received signal: %s. Shutting down server...", receivedSignal)

		// Create a context with a timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Printf("HTTP server Shutdown: %v", err)
		}
		logger.Println("Server shutdown complete.")
		close(idleConnsClosed)
	}()

	logger.Println("Server running on http://localhost:8080")
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed // Wait
	logger.Println("Application exiting.")
}
