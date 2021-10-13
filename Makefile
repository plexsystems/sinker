.PHONY: build
build:
	@go build -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$$(git describe --tags --always --dirty)'"

.PHONY: test 
test:
	@go test -v ./... -count=1

.PHONY: acceptance
acceptance: build
	@bats acceptance.bats

.PHONY: all
all: build test acceptance

# When using the release target a version must be specified.
# e.g. make release version=v0.1.0
.PHONY: release
release:
	@test $(version)
	@docker build --build-arg SINKER_VERSION=$(version) -t plexsystems/sinker:$(version) .
	@GOOS=darwin GOARCH=amd64 go build -o sinker-darwin-amd64 -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$(version)'"
	@GOOS=windows GOARCH=amd64 go build -o sinker-windows-amd64 -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$(version)'"
	@GOOS=linux GOARCH=amd64 go build -o sinker-linux-amd64 -ldflags="-X 'github.com/plexsystems/sinker/internal/commands.sinkerVersion=$(version)'"
