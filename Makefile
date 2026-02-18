.PHONY: build test vet run clean

build:
	go build -o workq ./cmd/workq

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

run: build
	./workq

clean:
	rm -f workq workq_state.json coverage.out

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
