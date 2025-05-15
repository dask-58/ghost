package main

import (
	"bytes"
	"fmt"
	"net/http"
)

func main() {
	url := "http://localhost:8080/join"

	// Enough for ~10000 requests to run under 10s
	// For higher numbers, use go-routines.
	for i := 1; i < 9010; i++ {
		playerId := fmt.Sprintf("playerId:%d", i)
		data := []byte(fmt.Sprintf(`{"playerId":"%s"}`, playerId))
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	
		if resp.StatusCode == 200 {
			fmt.Printf("Player %s joined queue, response: %s\n", playerId, resp.Status)
		} else {
			fmt.Printf("Player %s failed to join, response: %s\n", playerId, resp.Status)
		}

		resp.Body.Close()
	}
}
