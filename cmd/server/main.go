package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		defer r.Body.Close()

		var req PlayerRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		fmt.Printf("Received player ID: %s\n", req.PlayerId)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Player %s added to the queue", req.PlayerId)
	})

	fmt.Println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
