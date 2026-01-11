.PHONY: test test-integration test-unit test-coverage test-up test-down

# Run all tests (requires MongoDB, Redis and Meilisearch)
test:
	go test -v ./...

# Run unit tests only (no external dependencies)
test-unit:
	go test -v -short ./...

# Run integration tests with real MongoDB, Redis and Meilisearch
test-integration:
	TEST_MONGO_URI=mongodb://localhost:27017 \
	TEST_REDIS_ADDR=localhost:6379 \
	TEST_MEILI_HOST=http://localhost:7700 \
	TEST_MEILI_KEY= \
	go test -v -count=1 ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Start test containers
test-up:
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 8
	@echo "Test services are ready!"
	@echo "MongoDB: localhost:27018"
	@echo "Redis: localhost:6380"
	@echo "Meilisearch: localhost:7701"

# Stop test containers
test-down:
	docker compose -f docker-compose.test.yml down -v

# Run tests with test containers (using different ports)
test-docker:
	TEST_MONGO_URI=mongodb://localhost:27018 \
	TEST_REDIS_ADDR=localhost:6380 \
	TEST_MEILI_HOST=http://localhost:7701 \
	TEST_MEILI_KEY=test-master-key \
	go test -v -count=1 ./...

# Clean up test artifacts
clean:
	rm -f coverage.out coverage.html
	docker compose -f docker-compose.test.yml down -v 2>/dev/null || true
