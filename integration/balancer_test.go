package integration

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/roman-mazur/architecture-practice-4-template/config"
	"gopkg.in/check.v1"
)

const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 4* time.Second,
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
	requestsNum := 24
	rc := ResponseCounter{
		cts: map[string]struct {
			actual, expected int
		}{
			"server1:8080": {expected: 50},
			"server2:8080": {expected: 40},
			"server3:8080": {expected: 30},
		},
	}

	wg := sync.WaitGroup{}
	for i := 0; i < clientsNum; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < requestsNum/clientsNum; j++ {

				url := fmt.Sprintf("%s/api/v1/some-data", baseAddress)
				resp, err := client.Get(url)
				c.Assert(err, check.IsNil)
				c.Assert(resp.StatusCode, check.Equals, http.StatusOK)

				server := resp.Header.Get("lb-from")
				c.Assert(server, check.Not(check.Equals), "")
				rc.Increment(server)
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()



}

var _ = check.Suite(&IntegrationSuite{})

func (s *IntegrationSuite) TestSpecificKeyRequest(c *check.C) {
    if _, exists := os.LookupEnv("INTEGRATION_TEST"); !exists {
        c.Skip("Integration test is not enabled")
    }

    key := config.TeamName
    expectedValue := time.Now().Format("2006-01-02")

    url := fmt.Sprintf("%s/api/v1/some-data?key=%s", baseAddress, key)
    resp, err := client.Get(url)
    c.Assert(err, check.IsNil)
    defer resp.Body.Close()

    c.Assert(resp.StatusCode, check.Equals, http.StatusOK)

    body, err := io.ReadAll(resp.Body)
    c.Assert(err, check.IsNil)

    // Check the body is not empty and contains the expected value
    c.Assert(string(body), check.Not(check.Equals), "")
    c.Assert(string(body), check.Matches, fmt.Sprintf(`.*"%s".*`, expectedValue))
}


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
