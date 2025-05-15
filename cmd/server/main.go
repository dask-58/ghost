package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"github.com/dask-58/ghost/internal/queue"
)

type PlayerRequest struct {
	PlayerId string `json:"playerId"`
}

func main() {
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
			http.Error(w, "Only POST method allowed", http.StatusBadRequest)
			return
		}

		defer r.Body.Close() // Prevent resource leaks

		var req PlayerRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Add player to Queue (duplicates not allowed)
		player := queue.Player{ID: req.PlayerId}
		added := queue.JoinQueue(player)
		if added {
			fmt.Printf("Player %s added to queue. Current queue: %v\n", req.PlayerId, queue.GetQueue())
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Player %s added to queue", req.PlayerId)
		} else {
			fmt.Printf("Duplicate player %s tried to join\n", req.PlayerId)
			http.Error(w, "Player already in queue", http.StatusConflict)
		}
	})

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
