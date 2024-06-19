## Load Balancer Integration Testing

---

### Development Team:

- Illia Kornyk
- Anastasya Holubenko
- Vadym Didur
- Andriy Tarasiuk

### Lab №5

Completed Tasks

#### Task 1: Segment Support and Merging

Implemented dynamic segment creation and automated merging to optimize data storage and retrieval.

#### Task 2: Competitiveness

Enhanced data access speeds and concurrency handling to support high throughput and multiple simultaneous requests.

#### Task 4: Integration with Other Components

Integrated the database with a web server and a load balancer using Docker Compose, allowing for interaction between components.

### Project Overview:

In this project, our team has developed a load balancer as per the specifications of variant 8. The load balancer is designed to distribute HTTP requests across a pool of servers using the "Least Amount of Traffic" algorithm. This algorithm selects the server that has served the least amount of data, measured in bytes, rather than the number of connections. This strategy ensures a fair distribution of network load and optimizes the overall server response times.

For testing our implementation, we have chosen the `gocheck` library, which aligns with the requirements of variant 8. `Gocheck` provides rich testing features that enable us to write more expressive and comprehensive integration tests, ensuring our load balancer performs robustly under different scenarios.

### Usage Instructions:

1. **Environment Setup**:

   - Ensure Docker and Docker Compose are installed on your system.
   - Clone the repository using:
     ```
     git clone https://github.com/forestgreen18/architecture-practice-4-template.git
     ```

2. **Running the Load Balancer**:

   - Navigate to the project directory and start the services using:
     ```
     docker-compose up
     ```
   - The load balancer will be accessible on `localhost:8090`.

3. **Running Integration Tests**:

   - To execute integration tests, run:
     ```
     INTEGRATION_TEST=1 docker-compose -f docker-compose.yaml -f docker-compose.test.yaml up --exit-code-from test
     ```
   - Integration tests will verify the correct distribution of requests across servers.

4. **Running Benchmarks**:

   - Benchmarks can be run to measure the performance of the load balancer:
     ```
     INTEGRATION_TEST=1 go test -v -bench=. ./integration
     ```
   - Ensure the load balancer and servers are running before executing benchmarks.

5. **Shutting Down**:
   - To stop and remove all running services, use:
     ```
     docker-compose down
     ```
