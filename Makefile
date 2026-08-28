GO_IMAGE ?= golang:1.26-bookworm
NODE_IMAGE ?= node:24-bookworm-slim
ROOT := $(CURDIR)

.PHONY: test dashboard build windows macos

test:
	docker run --rm -v "$(ROOT):/src" -w /src $(GO_IMAGE) sh -c 'go test ./...'

dashboard:
	docker run --rm -v "$(ROOT)/dashboard:/app" -w /app $(NODE_IMAGE) sh -c 'npm ci && npm run build'
	mkdir -p internal/localapi/web
	cp -R dashboard/dist/. internal/localapi/web/

build: dashboard
	mkdir -p bin
	docker run --rm -v "$(ROOT):/src" -w /src $(GO_IMAGE) sh -c 'CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o bin/fiberpulse ./cmd/fiberpulse'

windows: dashboard
	docker build -f packaging/windows/Dockerfile.build -t fiberpulse-windows-builder .
	mkdir -p dist
	docker run --rm -v "$(ROOT)/dist:/artifacts" fiberpulse-windows-builder

macos: dashboard
	sh packaging/macos/build-app.sh
