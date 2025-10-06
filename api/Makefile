.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: build-verbose
build-verbose:
	GOFIPS140=v1.0.0 go build -v ./...

##@ Testing

.PHONY: test
test: fmt vet ## Run tests.
	GOFIPS140=v1.0.0 go test ./... -v --race -coverprofile cover.out
