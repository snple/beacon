.PHONY: fmt fmt-fix vet lint test test-race check

fmt:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "下列文件未格式化："; echo "$$out"; exit 1; fi

fmt-fix:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

check: fmt vet lint test-race
