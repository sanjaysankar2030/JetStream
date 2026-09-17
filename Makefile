# .PHONY: build run test guiBuild guiRun

build:
	@go build -o bin/fs

run: build
	@./bin/fs

test:
	go test ./... -v -run $(t)

git:
	git add .
	git commit -m "Encrypting and Decrypting with tests"
	git push -u origin main 

