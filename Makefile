.PHONY: build run clean

build:
	go build -o codeindex-mcp main.go

run:
	go run main.go

clean:
	rm -f codeindex-mcp
