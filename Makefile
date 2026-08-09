build:
	@go build -o bin/fs

run: build
	@./bin/fs

test:
	go test ./... -v -run $(t)

guiBuild:
	go build -ldflags="-H windowsgui" -o bin/gui

guiRun:guiBuild
	@./bin/gui -gui
