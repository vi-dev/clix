.PHONY: test
test:
	env -u GOROOT go vet ./...
	env -u GOROOT golangci-lint run ./...
	env -u GOROOT go test -race -shuffle=on ./...

.PHONY: hooks
hooks:
	git config core.hooksPath .githooks

.PHONY: cover
cover:
	env -u GOROOT go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: docs-captures
docs-captures:
	docs/tools/capture.sh docs/captures

.PHONY: docs-check
docs-check:
	env -u GOROOT go run ./docs/tools/check -compare -verify
