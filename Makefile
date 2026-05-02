.PHONY: setup
setup:
	go work init . ./api && go mod download && go mod tidy && cd api && go mod download && go mod tidy

.PHONY: start
start:
	go run cmd/main.go

.PHONY: debug
debug:
	DUCKTAPE_LOG=debug go run cmd/main.go

.PHONY: test
test:
	go test -count 1 ./...

.PHONY: bench
bench:
	go test -bench=. ./... -benchmem

.PHONY: build
build:
	docker run --rm --privileged \
		-v $(PWD):/go/src/github.com/revo-pos/ducktape \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-w /go/src/github.com/revo-pos/ducktape \
		-e CGO_ENABLED=1 \
		goreleaser/goreleaser-cross:latest build --clean

.PHONY: build-images
build-images:
	docker run --rm --privileged \
		-v $(PWD):/go/src/github.com/revo-pos/ducktape \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-w /go/src/github.com/revo-pos/ducktape \
		-e CGO_ENABLED=1 \
		goreleaser/goreleaser-cross:latest release --snapshot --clean
	@echo "Docker images built successfully!"
	@echo "Available images:"
	@docker images | grep ducktape | head -10

.PHONY: release
release:
	@echo "Building release with goreleaser..."
	docker run --rm --privileged \
		-v $(PWD):/go/src/github.com/revo-pos/ducktape \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-w /go/src/github.com/revo-pos/ducktape \
		-e CGO_ENABLED=1 \
		-e GITHUB_TOKEN \
		goreleaser/goreleaser-cross:latest release --clean
	@echo ""
	@echo "Pushing Docker images..."
	@TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "latest"); \
	echo "Pushing architecture-specific images for $$TAG..."; \
	docker push revo-pos/ducktape:latest-amd64 && \
	docker push revo-pos/ducktape:latest-arm64 && \
	docker push revo-pos/ducktape:$$TAG-amd64 && \
	docker push revo-pos/ducktape:$$TAG-arm64 && \
	echo "" && \
	echo "Creating and pushing multi-arch manifests..." && \
	docker manifest rm revo-pos/ducktape:latest 2>/dev/null || true && \
	docker manifest create revo-pos/ducktape:latest \
		revo-pos/ducktape:latest-amd64 \
		revo-pos/ducktape:latest-arm64 && \
	docker manifest push revo-pos/ducktape:latest && \
	docker manifest rm revo-pos/ducktape:$$TAG 2>/dev/null || true && \
	docker manifest create revo-pos/ducktape:$$TAG \
		revo-pos/ducktape:$$TAG-amd64 \
		revo-pos/ducktape:$$TAG-arm64 && \
	docker manifest push revo-pos/ducktape:$$TAG
	@echo ""
	@echo "Release complete!"
