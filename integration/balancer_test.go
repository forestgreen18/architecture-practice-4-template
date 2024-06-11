package integration

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"

	"gopkg.in/check.v1"
)

const baseAddress = "http://balancer:8090"

var _ = check.Suite(&BalancerSuite{})

type BalancerSuite struct{}

func Test(t *testing.T) { check.TestingT(t) }

func (s *BalancerSuite) TestBalancerLogic(c *check.C) {
	if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
		c.Skip("Integration test is not enabled")
	}

	var totalRequests = 100
	var serverResponses = make(map[string]int)
	var mu sync.Mutex

	// Send multiple requests to the balancer and record the server each response comes from.
	for i := 0; i < totalRequests; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/some-data", baseAddress))
		c.Assert(err, check.IsNil)
		defer resp.Body.Close()

		// Check if the response status code is 200 OK.
		c.Assert(resp.StatusCode, check.Equals, http.StatusOK)

		// Record the server that responded.
		server := resp.Header.Get("lb-from")
		c.Assert(server, check.Not(check.Equals), "")

		mu.Lock()
		serverResponses[server]++
		mu.Unlock()
	}

	// Log the number of responses from each server.
	for server, count := range serverResponses {
		c.Logf("Server %s responded to %d requests", server, count)
	}

	// Check if the distribution of requests is as expected.
	// The server with the least traffic should have responded to the most requests.
	var maxResponses int
	for _, count := range serverResponses {
		if count > maxResponses {
			maxResponses = count
		}
	}
	for server, count := range serverResponses {
		// Use a boolean expression to check the condition.
		c.Assert(count <= maxResponses, check.Equals, true, check.Commentf("Server %s responded to too many requests", server))
	}
}


func BenchmarkBalancer(b *testing.B) {

}
