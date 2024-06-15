package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/roman-mazur/architecture-practice-4-template/config"
	"github.com/roman-mazur/architecture-practice-4-template/httptools"
	"github.com/roman-mazur/architecture-practice-4-template/signal"
)

var port = flag.Int("port", config.ServerPort, "server port")
const confResponseDelaySec = "CONF_RESPONSE_DELAY_SEC"
const confHealthFailure = "CONF_HEALTH_FAILURE"
const dbServiceURL = "http://db:8083/db"

func main() {
	flag.Parse()

	h := new(http.ServeMux)
	client := http.DefaultClient

	initializeDatabaseConnection()

	h.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-type", "text/plain")
		if failConfig := os.Getenv(confHealthFailure); failConfig == "true" {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("FAILURE"))
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte("OK"))
		}
	})

h.HandleFunc("/api/v1/some-data", func(rw http.ResponseWriter, r *http.Request) {
    keys, keyPresent := r.URL.Query()["key"]

    if respDelayString := os.Getenv(confResponseDelaySec); respDelayString != "" {
        if delaySec, err := strconv.Atoi(respDelayString); err == nil && delaySec > 0 && delaySec < 300 {
            time.Sleep(time.Duration(delaySec) * time.Second)
        }
    }

    // Different behavior based on the presence of the "key" parameter
    if keyPresent && len(keys[0]) > 0 {
        key := keys[0]
        response, err := client.Get(fmt.Sprintf("%s/%s", dbServiceURL, key))
        if err != nil {
            log.Printf("Error fetching data from DB service: %v", err)
            rw.WriteHeader(http.StatusInternalServerError)
            return
        }
        defer response.Body.Close()

        if response.StatusCode == http.StatusNotFound {
            rw.WriteHeader(http.StatusNotFound)
            return
        }

        body, err :=  io.ReadAll(response.Body)
        if err != nil {
            log.Printf("Error reading response body: %v", err)
            rw.WriteHeader(http.StatusInternalServerError)
            return
        }

        rw.Header().Set("content-type", "application/json")
        rw.WriteHeader(http.StatusOK)
        rw.Write(body)
    } else {
        // No key provided, return a predefined response

        rw.Header().Set("content-type", "application/json")
        rw.WriteHeader(http.StatusOK)
        _ = json.NewEncoder(rw).Encode([]string{"1", "2"})
    }
})



	server := httptools.CreateServer(*port, h)
	go server.Start()
	log.Printf("Server started on port %d", *port)

	signal.WaitForTerminationSignal()
}


const maxRetries = 5
const retryInterval = 2 * time.Second

func postWithRetry(url string, jsonData []byte) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err == nil  {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("Attempt %d failed, retrying in %v...", i+1, retryInterval)
		time.Sleep(retryInterval)
	}
	return fmt.Errorf("failed to post data after %d attempts: %v", maxRetries, err)
}



func initializeDatabaseConnection() {
	teamName := config.TeamName
	currentDate := time.Now().Format("2006-01-02")
	jsonData := []byte(fmt.Sprintf(`{"value":"%s"}`, currentDate))

	dbServiceURL := fmt.Sprintf("http://db:8083/db/%s", teamName)
	err := postWithRetry(dbServiceURL, jsonData)
	if err != nil {
		log.Fatalf("Failed to initialize database with date: %v", err)
	}
}
