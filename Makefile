.PHONY: verify test integration-test lint-go lint-frontend tidy

verify: tidy lint-go test lint-frontend

tidy:
	go work sync
	cd controller && go mod tidy
	cd agent && go mod tidy
	cd cli && go mod tidy
	cd bench && go mod tidy
	cd tests/integration && go mod tidy
	cd agent/cmd/doctor && go mod tidy

lint-go:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "install golangci-lint: https://golangci-lint.run"; exit 1; }
	for d in controller agent cli bench; do \
		(cd $$d && golangci-lint run --timeout=5m --config=../.golangci.yml ./...); \
	done
	cd agent/cmd/doctor && golangci-lint run --timeout=5m --config=../../../.golangci.yml ./...

test:
	cd controller && go test -race ./...
	cd agent && go test -race ./...
	cd cli && go test -race ./...

integration-test:
	cd tests/integration && go test -tags=integration -race -v ./...

lint-frontend:
	cd dashboard-v2 && npm ci && npm run lint && npm run type-check && npm run test
