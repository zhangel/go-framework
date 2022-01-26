BIN=framework
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o $(BIN) -ldflags="-r ." .
run:build
	./$(BIN)
