export GO111MODULE=on

default: test

ci: depsdev test

test:
	go test ./... -race -coverprofile=coverage.out -covermode=atomic -count=1

test-integration:
	go test ./... -tags integration -race -count=1 -timeout 600s -v

lint:
	golangci-lint run ./...
	go vet -vettool=`which gostyle` -gostyle.config=$(PWD)/.gostyle.yml ./...

depsdev:
	go install github.com/k1LoW/gostyle@latest

# Phony because a case-insensitive filesystem takes CREDITS as this target's output.
.PHONY: credits
credits:
	go install github.com/Songmu/gocredits/cmd/gocredits@v1.0.0
	gocredits . > CREDITS
	cat _EXTRA_CREDITS >> CREDITS

prerelease_for_tagpr:
	$(MAKE) credits
	git add CHANGELOG.md CREDITS go.mod go.sum
