package main

import (
	"bytes"
	"fmt"
	"net/http"
)

func main() {
	url := "http://localhost:8080/join"
	data := []byte(`{"playerId":"player1"}`)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	fmt.Println("Joined queue, response:", resp.Status)
}
