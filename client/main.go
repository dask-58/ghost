package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
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

func simulateJoins(requests int, maxConcurrency int, delta int, url string, httpClient *http.Client) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrency)

	log.Printf("Starting SIMULATED ARRIVAL: %d join requests, max concurrency %d, mean arrival delay ~%dms...\n",
		requests, maxConcurrency, delta)
	startTime := time.Now()
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 1; i <= requests; i++ {
		wg.Add(1)
		playerId := fmt.Sprintf("sim-player-%d", i)

		go func(pId string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			joinRequest(httpClient, url, pId, &wg)
		}(playerId)

		if delta > 0 {
			randomFactor := 0.5 + rnd.Float64()
			delay := time.Duration(float64(delta) * randomFactor * float64(time.Millisecond))
			time.Sleep(delay)
		}
	}

	wg.Wait()
	close(semaphore)

	elapsedTime := time.Since(startTime)
	log.Printf("SIMULATED ARRIVAL: All %d requests completed in %s.\n", requests, elapsedTime)
	
	if elapsedTime.Seconds() > 0 {
		log.Printf("SIMULATED ARRIVAL: Effective throughput: %.2f req/sec\n", float64(requests) / elapsedTime.Seconds())
	}
}

func spamJoinRequests(requests int, maxConcurrency int, url string, httpClient *http.Client) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrency)

	log.Printf("Starting SPAM REQUESTS: %d join requests, max concurrency %d...\n", requests, maxConcurrency)
	startTime := time.Now()

	for i := 1; i <= requests; i++ {
		wg.Add(1)
		playerId := fmt.Sprintf("spam-player-%d", i)
		go func(pId string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			joinRequest(httpClient, url, pId, &wg)
		}(playerId)
	}

	wg.Wait()
	close(semaphore)

	elapsedTime := time.Since(startTime)
	log.Printf("SPAM REQUESTS: All %d requests completed in %s.\n", requests, elapsedTime)
	if elapsedTime.Seconds() > 0 {
		log.Printf("SPAM REQUESTS: Effective throughput: %.2f req/sec\n", float64(requests) / elapsedTime.Seconds())
	}
}

func main() {
	url := "http://localhost:8080/join"

	sharedMaxConcurrency := 100
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns: sharedMaxConcurrency + 10,
			MaxIdleConnsPerHost: sharedMaxConcurrency + 10,
			IdleConnTimeout: 90 * time.Second,
		},
	}

	numSimulatedRequests := 1000
	delta := 10
	simulateJoins(numSimulatedRequests, sharedMaxConcurrency, delta, url, httpClient)

	/*
	numSpamRequests := 10000
	time.Sleep(2 * time.Second) // if running both sequentially for logs
	spamJoinRequests(numSpamRequests, sharedMaxConcurrency, url, httpClient)
	*/
	

	log.Println("Client Simulation Finished")
}
