.PHONY: fmt lint sync

fmt:
	goimports -w .
	golines -w --max-len=80 .

lint:
	golangci-lint run ./...

sync:
	go run .
