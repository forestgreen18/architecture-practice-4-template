package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/roman-mazur/architecture-practice-4-template/httptools"
	"github.com/roman-mazur/architecture-practice-4-template/signal"
)

var (
	port        = flag.Int("port", 8090, "load balancer port")
	timeoutSec  = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https       = flag.Bool("https", false, "whether backends support HTTPs")
	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

type Server struct {
	Address string
	Traffic uint64 // Cumulative bytes served
}

var (
	timeout      = time.Duration(*timeoutSec) * time.Second
	serversPool  = []Server{
		{Address: "server1:8080", Traffic: 0},
		{Address: "server2:8080", Traffic: 0},
		{Address: "server3:8080", Traffic: 0},
	}
	scheme = func() string {
		if *https {
			return "https"
		}
		return "http"
	}
)

func health(dst string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s/health", scheme(), dst), nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, fmt.Errorf("health check failed with status code: %d", resp.StatusCode)
}

func forward(dst string, rw http.ResponseWriter, r *http.Request) error {
	ctx, _ := context.WithTimeout(r.Context(), timeout)
	fwdRequest := r.Clone(ctx)
	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = dst
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = dst

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err == nil {
		// Copy all headers from the response
		for k, values := range resp.Header {
			for _, value := range values {
				rw.Header().Add(k, value)
			}
		}

		// Update the traffic for the server that served the request
		for i := range serversPool {
			if serversPool[i].Address == dst {
				// Inside the forward function, after updating the server's traffic
				log.Printf("Server %s has served %d bytes of traffic.", serversPool[i].Address, serversPool[i].Traffic)

				if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
					if bytesServed, err := strconv.ParseUint(contentLength, 10, 64); err == nil {
						serversPool[i].Traffic += bytesServed
					}
				}
				break
			}
		}

		// Set tracing information if enabled
		if *traceEnabled {
			rw.Header().Set("lb-from", dst)
		}
		log.Println("fwd", resp.StatusCode, resp.Request.URL)
		rw.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		_, err := io.Copy(rw, resp.Body)
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
		return nil
	} else {
		log.Printf("Failed to get response from %s: %s", dst, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
		return err
	}
}

func main() {
	flag.Parse()

	// Initialize the timeout for health checks
	timeout := time.Duration(*timeoutSec) * time.Second

	// Maintain the list of healthy servers
	for _, server := range serversPool {
		server := server
		go func() {
			for range time.Tick(10 * time.Second) {
				healthy, err := health(server.Address, timeout)
				if err != nil {
					log.Printf("Health check failed for server %s: %v", server.Address, err)
				} else {
					log.Printf("Server %s health: %v", server.Address, healthy)
				}
			}
		}()
	}

	// Create the server and define the handler
	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// Select the server with the lowest traffic volume
		selectedServer, err := selectServer(serversPool)
		if err != nil {
			// Handle the case where no healthy servers are found
			log.Printf("Failed to select server: %s", err)
			http.Error(rw, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		// Forward the request to the selected server
		err = forward(selectedServer.Address, rw, r)
		if err != nil {
			log.Printf("Failed to forward request: %s", err)
			http.Error(rw, "Service Unavailable", http.StatusServiceUnavailable)
		}
	}))

	// Start the load balancer
	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}

func selectServer(serversPool []Server) (*Server, error) {
	var minTrafficServer *Server
	for i := range serversPool {
		server := &serversPool[i]
		log.Printf("Server %s traffic: %d", server.Address, server.Traffic)
		healthy, err := health(server.Address, timeout)
		if err != nil {
			// Handle the error, for example, log it or continue with the next server
			log.Printf("Health check failed for server %s: %v", server.Address, err)
			continue
		}
		if healthy {
			if minTrafficServer == nil || server.Traffic < minTrafficServer.Traffic {
				minTrafficServer = server
			}
		}
	}
	if minTrafficServer == nil {
		return nil, fmt.Errorf("no healthy servers found")
	}
	return minTrafficServer, nil
}
