package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

func joinRequest(httpClient *http.Client, url string, playerId string, wg *sync.WaitGroup) {
	defer wg.Done()

	data := make([]byte, 0, 64)
	data = fmt.Appendf(data, `{"playerId":"%s"}`, playerId)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))

	if err != nil {
		log.Printf("Player %s: Error creating request: %v\n", playerId, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Player %s: Error sending request: %v\n", playerId, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// log.Printf("Player %s joined queue, response: %s\n", playerId, resp.Status)
	} else if resp.StatusCode == http.StatusConflict {
		log.Printf("Player %s already in queue, response: %s\n", playerId, resp.Status)
	} else {
		log.Printf("Player %s failed to join, status: %s\n", playerId, resp.Status)
	}

}

func main() {
	url := "http://localhost:8080/join"

	numRequests := 10000
	maxConcurrent := 100 // max no of go routines sending requests at once

	var wg sync.WaitGroup

	httpClient := &http.Client{
		Timeout: 10 * time.Second, // Set a timeout for requests
	}
	
	semaphore := make(chan struct{}, maxConcurrent) // limit concurrency

	log.Printf("Starting to send %d join requests with max concurrency %d...\n", numRequests, maxConcurrent)
	startTime := time.Now()

	for i := 1; i <= numRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire a spot
		playerId := fmt.Sprintf("playerId:%d", i)

		go func(pId string) {
			defer func() { <-semaphore }() // Release spot
			joinRequest(httpClient, url, pId, &wg)
		}(playerId)
	}

	wg.Wait() // Wait for all goroutines to complete
	close(semaphore)

	elapsedTime := time.Since(startTime)
	log.Printf("All %d requests completed in %s.\n", numRequests, elapsedTime)
	// not exactly average but maybe the no of requests at some instance which could be completed in a second
	log.Printf("Average requests per second: %.2f\n", float64(numRequests) / elapsedTime.Seconds())
}
