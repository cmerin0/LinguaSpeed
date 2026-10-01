# Default target: build the server binary
.PHONY: build
build:
	go build -o linguaspeed ./cmd/server

# test-unit: runs fast unit tests with no external dependencies (no Docker required)
.PHONY: test-unit
test-unit:
	go test -v -count=1 ./internal/scoring/... ./internal/domain/... ./internal/handler/...

# test-property: runs property-based tests using pgregory.net/rapid (no Docker required)
.PHONY: test-property
test-property:
	go test -v -count=1 -run Property ./...

# test-integration: runs integration tests that require Docker via testcontainers
.PHONY: test-integration
test-integration:
	go test -v -count=1 -tags integration ./...

# test: runs the full test suite — unit, property, and integration tests in sequence
.PHONY: test
test: test-unit test-property test-integration

# lint: runs go vet to catch common mistakes (no extra linter dependency required)
.PHONY: lint
lint:
	go vet ./...

# clean: removes the compiled server binary
.PHONY: clean
clean:
	rm -f linguaspeed
