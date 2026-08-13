.PHONY: fmt lint sync coverage

fmt:
	goimports -w .
	golines -w --max-len=80 .

lint:
	golangci-lint run ./...

sync:
	go run .

coverage:
	go run . coverage
