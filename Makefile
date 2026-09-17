.PHONY: build run test clean install

APP_NAME := lintop

build:
	go build -o $(APP_NAME) main.go

run:
	go run main.go

test:
	go test -v ./internal/app/... ./internal/i18n/...

install:
	install -Dm755 $(APP_NAME) $(DESTDIR)/usr/local/bin/$(APP_NAME)

clean:
	rm -f $(APP_NAME)

modernize:
	go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...

sexy:
	go fmt ./...
	go vet ./...
	$$(go env GOPATH)/bin/gocyclo -over 15 .
	$$(go env GOPATH)/bin/ineffassign ./...
