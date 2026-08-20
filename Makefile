.PHONY: fmt lint sync validate

fmt:
	goimports -w .
	golines -w --max-len=80 .

lint:
	golangci-lint run ./...

sync:
	go run .

validate:
	go run . -provider none
