package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
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

	teamName := config.TeamName
	currentDate := time.Now().Format("2006-01-02")
	jsonData := []byte(fmt.Sprintf(`{"value":"%s"}`, currentDate))
	_, err := client.Post(fmt.Sprintf("%s/%s", dbServiceURL, teamName), "application/json", bytes.NewBuffer(jsonData))
	fmt.Println("err", err)
	if err != nil {
		log.Fatalf("Failed to initialize database with date: %v", err)
	}

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
		keys, ok := r.URL.Query()["key"]
		if !ok || len(keys[0]) < 1 {
			log.Println("Url Param 'key' is missing")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		key := keys[0]

		respDelayString := os.Getenv(confResponseDelaySec)
		if delaySec, parseErr := strconv.Atoi(respDelayString); parseErr == nil && delaySec > 0 && delaySec < 300 {
			time.Sleep(time.Duration(delaySec) * time.Second)
		}

		response, err := client.Get(fmt.Sprintf("%s/%s", dbServiceURL, key))
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Println("Response: ", response)
		defer response.Body.Close()

		if response.StatusCode == http.StatusNotFound {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.Header().Set("content-type", "application/json")
		rw.WriteHeader(http.StatusOK)
		rw.Write(body)
	})

	server := httptools.CreateServer(*port, h)
	go server.Start()
	log.Printf("Server started on port %d", *port)

	signal.WaitForTerminationSignal()
}
