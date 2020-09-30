.PHONY: build
build:
	@go build

.PHONY: remove-images
remove-images:
	@docker rmi `docker images -a -q`

.PHONY: test
test:
	@go test -v ./... -count=1

.PHONY: acceptance
acceptance: build
	@bats acceptance.bats

.PHONY: release
release:
	@test $(version)
	GOOS=darwin GOARCH=amd64 go build -o sinker-darwin-amd64 -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$(version)'"
	GOOS=windows GOARCH=amd64 go build -o sinker-windows-amd64 -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$(version)'"
	GOOS=linux GOARCH=amd64 go build -o sinker-linux-amd64 -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$(version)'"
