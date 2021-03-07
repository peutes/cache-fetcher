
update: FORCE ## udpate vendor
	go mod vendor -v
	go mod tidy

lint: FORCE ## lint
	golangci-lint run ./... --out-format tab || true
	golangci-lint run ./... --out-format tab -p bugs -D errcheck || true
	golangci-lint run ./... --out-format tab -p format || true
	golangci-lint run ./... --out-format tab -p unused || true
	golangci-lint run ./... --out-format tab -p performance -D prealloc -D maligned || true
	golangci-lint run ./... --out-format tab -p complexity -D nestif || true
	golangci-lint run ./... --out-format tab -p style -D godot -D funlen -D ifshort -D paralleltest -D godox -D gomnd -D exhaustivestruct -D wsl -D gochecknoglobals -D nolintlint -D goprintffuncname -D nlreturn -D wrapcheck || true

test: FORCE ## test
	go test --race -v ./cachefetcher/...

run-test: FORCE ## run
	air -c .air.toml

help: FORCE
	@awk -F ':|##' '/^[^\t].+?:.*?##/ {printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF}' $(MAKEFILE_LIST)

FORCE:
