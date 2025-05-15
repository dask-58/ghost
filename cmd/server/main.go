package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
	"github.com/dask-58/ghost/internal/queue"
)

type PlayerRequest struct {
	PlayerId string `json:"playerId"`
}

var addedSinceLastLog int64

func main() {
	startTime := time.Now() // Capture server start time

	// Log summary every 1 second.
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			count := atomic.SwapInt64(&addedSinceLastLog, 0)
			if count > 0 {
				elapsed := time.Since(startTime).Round(time.Second)
				fmt.Printf("[Summary after %v] %d players added in last 1 second. Queue size: %d\n",
					elapsed, count, len(queue.GetQueue()))
			}
		}
	}()

	// Handle URL
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "RoyaleQueue matchmaking server")
	})

	// join
	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req PlayerRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		player := queue.Player{ID: req.PlayerId}
		added := queue.JoinQueue(player)
		if added {
			atomic.AddInt64(&addedSinceLastLog, 1)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Player %s added to queue", req.PlayerId)
		} else {
			fmt.Printf("Duplicate player %s tried to join\n", req.PlayerId) // still logs duplicates
			http.Error(w, "Player already in queue", http.StatusConflict)
		}
	})

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
