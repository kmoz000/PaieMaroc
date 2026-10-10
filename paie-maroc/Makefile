.PHONY: dev test lint build clean

dev:
	wails dev

test:
	go test -race ./...
	cd frontend && npm test
	cd frontend && npm run build

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
	cd frontend && npm audit --audit-level=moderate
	cd frontend && npm test
	cd frontend && npm run build

build:
	wails build -clean

clean:
	go clean -testcache
	rm -rf build/bin frontend/dist
