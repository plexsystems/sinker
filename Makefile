.PHONY: release
release:
	GOOS=darwin GOARCH=amd64 go build -o imagesync-darwin-amd64
	GOOS=windows GOARCH=amd64 go build -o imagesync-windows-amd64
	GOOS=linux GOARCH=amd64 go build -o imagesync-linux-amd64