BIN=go-framework
build:
	go build -o $(BIN) *.go
	./$(BIN)
