test:
	go test ./...

check-env:
ifndef VERSION
	$(error VERSION is undefined)
endif

prep-release:
	make check-env
	go mod tidy
	make test
	git tag v$(VERSION)

publish:
	make check-env
	git push origin v$(VERSION)
	GOPROXY=proxy.golang.org go list -m github.com/Daskott/kronu@v$(VERSION)

release:
	make prep-release
	make publish
