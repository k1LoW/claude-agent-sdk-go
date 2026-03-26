export GO111MODULE=on

default: test

ci: depsdev test

test:
	go test ./... -race -coverprofile=coverage.out -covermode=atomic -count=1

lint:
	golangci-lint run ./...
	go vet -vettool=`which gostyle` -gostyle.config=$(PWD)/.gostyle.yml ./...

depsdev:
	go install github.com/Songmu/gocredits/cmd/gocredits@latest
	go install github.com/k1LoW/gostyle@latest

prerelease_for_tagpr: depsdev
	go mod download
	gocredits -w .
	git add CHANGELOG.md CREDITS go.mod go.sum
