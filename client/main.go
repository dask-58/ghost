package main

import (
	"bytes"
	"fmt"
	"net/http"
)

func main() {
	playerId := "player1"

	url := "http://localhost:8080/join"
	data := []byte(fmt.Sprintf(`{"playerId":"%s"}`, playerId))
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	defer resp.Body.Close() // Prevent resource leaks

	if resp.StatusCode == 200 {
		fmt.Printf("Player %s joined queue, response: %s\n", playerId, resp.Status)
	} else {
		fmt.Printf("Player %s failed to join, response: %s\n", playerId, resp.Status)
	}
}
