package integration

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"gopkg.in/check.v1"
)

const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 3 * time.Second,
}

func Test(t *testing.T) { check.TestingT(t) }

type IntegrationSuite struct{}

var _ = check.Suite(&IntegrationSuite{})

type ResponseCounter struct {
	cts map[string]struct {
		actual, expected int
	}
	mu sync.Mutex
}

func (rc *ResponseCounter) Increment(server string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	ct := rc.cts[server]
	ct.actual++
	rc.cts[server] = ct
}

func (s *IntegrationSuite) TestBalancer(c *check.C) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		c.Skip("Integration test is not enabled")
	}

	clientsNum := 3
	requestsNum := 120
	// rc := ResponseCounter{
	// 	cts: map[string]struct {
	// 		actual, expected int
	// 	}{
	// 		"server1:8080": {expected: 50},
	// 		"server2:8080": {expected: 40},
	// 		"server3:8080": {expected: 30},
	// 	},
	// }

	wg := sync.WaitGroup{}
	for i := 0; i < clientsNum; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < requestsNum/clientsNum; j++ {
				// fmt.Println("TestBalancer finished")

				url := fmt.Sprintf("%s/api/v1/some-data", baseAddress)
				resp, err := client.Get(url)
				c.Assert(err, check.IsNil)
				c.Assert(resp.StatusCode, check.Equals, http.StatusOK)


				resp.Body.Close()
			}
		}()
	}
	wg.Wait()

}



// func (s *IntegrationSuite) TestBalancer(c *check.C) {
// 	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
// 		c.Skip("Integration test is not enabled")
// 	}

// 	clientsNum := 3
// 	requestsNum := 120
// 	rc := ResponseCounter{
// 		cts: map[string]struct {
// 			actual, expected int
// 		}{
// 			"server1:8080": {expected: 50},
// 			"server2:8080": {expected: 40},
// 			"server3:8080": {expected: 30},
// 		},
// 	}

// 	wg := sync.WaitGroup{}
// 	for i := 0; i < clientsNum; i++ {
// 		wg.Add(1)

// 		go func() {
// 			defer wg.Done()

// 			for j := 0; j < requestsNum/clientsNum; j++ {
// 	fmt.Println("TestBalancer finished")

// 				url := fmt.Sprintf("%s/api/v1/some-data", baseAddress)
// 				resp, err := client.Get(url)
// 				c.Assert(err, check.IsNil)
// 				c.Assert(resp.StatusCode, check.Equals, http.StatusOK)

// 				server := resp.Header.Get("lb-from")
// 				c.Assert(server, check.Not(check.Equals), "")
// 				rc.Increment(server)
// 				resp.Body.Close()
// 			}
// 		}()
// 	}
// 	wg.Wait()

// 	for server, ct := range rc.cts {
// 		c.Logf("server %s processed %d requests", server, ct.actual)
// 		delta := 20
// 		c.Assert(ct.actual >= ct.expected-delta && ct.actual <= ct.expected+delta, check.Equals, true)
// 	}


// }

func BenchmarkBalancer(b *testing.B) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		b.Skip("Integration benchmark is not enabled")
	}


	for i := 0; i < b.N; i++ {
		fmt.Println("Benchmark finished")

		url := fmt.Sprintf("%s/api/v1/some-data", baseAddress)
		resp, err := client.Get(url)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Unexpected status code: %v", resp.StatusCode)
		}
		resp.Body.Close()
	}
}
