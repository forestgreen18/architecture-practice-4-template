package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/roman-mazur/architecture-practice-4-template/datastore"
	"github.com/roman-mazur/architecture-practice-4-template/httptools"
	"github.com/roman-mazur/architecture-practice-4-template/signal"
)

var port = flag.Int("port", 8083, "server port")
var db *datastore.Db

func main() {
	flag.Parse()

	var err error
	// Initialize the database

	tempDir, err := ioutil.TempDir("", "temporaryDir")
	if err != nil {
		log.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err = datastore.NewDb(tempDir, 250)
	if err != nil {
		log.Fatalf("Failed to initialize the database: %v", err)
	}

	h := http.NewServeMux()

	h.HandleFunc("/db", dbHandler)
	h.HandleFunc("/db/", dbHandler)

	server := httptools.CreateServer(*port, h)
	go server.Start()
	log.Printf("Server started on port %d", *port)

	signal.WaitForTerminationSignal()
	db.Close()
}

func dbHandler(res http.ResponseWriter, req *http.Request) {
	key := path.Base(req.URL.Path)
	if key == "/" || key == "" {
		http.Error(res, "Key is missing", http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "GET":
		value, err := db.Get(key)
		if err == datastore.ErrNotFound {
			http.NotFound(res, req)
			return
		} else if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		response, _ := json.Marshal(map[string]string{"key": key, "value": value})
		res.Header().Set("Content-Type", "application/json")
		res.Write(response)

	case "POST":
		var data struct {
			Value string `json:"value"`
		}
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(res, "Invalid request", http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(body, &data)
		if err != nil {
			http.Error(res, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		err = db.Put(key, data.Value)
		if err != nil {
			http.Error(res, "Failed to store the data", http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusCreated)

	default:
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
