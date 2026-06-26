SHELL := /bin/bash

VERSION=$(shell git describe --abbrev=0 --always)
LDFLAGS = -ldflags "-X github.com/ForceCLI/force/lib.Version=${VERSION}"
GCFLAGS = -gcflags="all=-N -l"
EXECUTABLE=force
WINDOWS=$(EXECUTABLE)_windows_amd64.exe
WINDOWS_ARM64=$(EXECUTABLE)_windows_arm64.exe
LINUX=$(EXECUTABLE)_linux_amd64
LINUX_ARM64=$(EXECUTABLE)_linux_arm64
OSX_AMD64=$(EXECUTABLE)_osx_amd64
OSX_ARM64=$(EXECUTABLE)_osx_arm64
ALL=$(WINDOWS) $(WINDOWS_ARM64) $(LINUX) $(LINUX_ARM64) $(OSX_AMD64) $(OSX_ARM64)
ZIPS=$(addsuffix .zip,$(basename $(ALL)))
RELEASE_ASSETS=$(ZIPS) SHA256SUMS-$(VERSION)

default:
	go build ${LDFLAGS}

install:
	go install ${LDFLAGS}

install-debug:
	go install ${LDFLAGS} ${GCFLAGS}

$(WINDOWS):
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -v -o $(WINDOWS) ${LDFLAGS}

$(WINDOWS_ARM64):
	env CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -v -o $(WINDOWS_ARM64) ${LDFLAGS}

$(LINUX):
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o $(LINUX) ${LDFLAGS}

$(LINUX_ARM64):
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -o $(LINUX_ARM64) ${LDFLAGS}

$(OSX_AMD64):
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -v -o $(OSX_AMD64) ${LDFLAGS}
	rcodesign sign --for-notarization --pem-file <(pass OctoberSwimmer/codesign/combined) $@

$(OSX_ARM64):
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -v -o $(OSX_ARM64) ${LDFLAGS}
	rcodesign sign --for-notarization --pem-file <(pass OctoberSwimmer/codesign/combined) $@

$(basename $(WINDOWS)).zip: $(WINDOWS)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

$(basename $(WINDOWS_ARM64)).zip: $(WINDOWS_ARM64)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

$(basename $(OSX_AMD64)).zip: $(OSX_AMD64)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)
	rcodesign notary-submit --api-key-file <(pass OctoberSwimmer/codesign/api-key) $@

$(basename $(OSX_ARM64)).zip: $(OSX_ARM64)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)
	rcodesign notary-submit --api-key-file <(pass OctoberSwimmer/codesign/api-key) $@

%.zip: %
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)

docs:
	go run docs/mkdocs.go

dist: test $(ZIPS)

checksum: dist
	shasum -a 256 $(ZIPS) > SHA256SUMS-$(VERSION)

release: checksum
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "gh CLI is required for 'make release'."; \
		exit 1; \
	fi
	@if ! git rev-parse --verify "refs/tags/$(VERSION)" >/dev/null 2>&1; then \
		echo "Tag '$(VERSION)' does not exist. Create the tag before running 'make release'."; \
		exit 1; \
	fi
	@if [ "$$(git describe --exact-match --tags HEAD 2>/dev/null)" != "$(VERSION)" ]; then \
		echo "HEAD is not exactly at tag '$(VERSION)'. Check out the tag before running 'make release'."; \
		exit 1; \
	fi
	git push force "$(VERSION)"
	gh release create "$(VERSION)" --title "force $(VERSION)" --notes-from-tag --verify-tag $(RELEASE_ASSETS)

fmt:
	go fmt ./...

test:
	test -z "$(go fmt)"
	go vet
	go test ./...
	go test -race ./...

clean:
	-rm -f $(EXECUTABLE) $(EXECUTABLE)_* *.zip SHA256SUMS-*

.PHONY: default dist clean docs checksum release tag

tag:
	@echo "Creating next tag..."
	@bash -c ' \
	PREV_TAG=$$(git tag --sort=-version:refname | head -1); \
	if [ -z "$$PREV_TAG" ]; then \
		NEXT_TAG="v0.0.1"; \
	else \
		VERSION=$$(echo $$PREV_TAG | sed "s/v//"); \
		MAJOR=$$(echo $$VERSION | cut -d. -f1); \
		MINOR=$$(echo $$VERSION | cut -d. -f2); \
		PATCH=$$(echo $$VERSION | cut -d. -f3); \
		NEXT_MINOR=$$((MINOR + 1)); \
		NEXT_TAG="v$$MAJOR.$$NEXT_MINOR.0"; \
	fi; \
	echo "Previous tag: $$PREV_TAG"; \
	echo "Next tag: $$NEXT_TAG"; \
	echo ""; \
	echo "Changelog:"; \
	git changelog $$PREV_TAG..; \
	echo ""; \
	read -p "Create tag $$NEXT_TAG? [y/N] " -n 1 -r; \
	echo ""; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		CHANGELOG=$$(git changelog $$PREV_TAG..); \
		git tag -a $$NEXT_TAG -m "Version $$NEXT_TAG" -m "" -m "$$CHANGELOG"; \
		echo "Tag $$NEXT_TAG created successfully"; \
		echo "Push with: git push force $$NEXT_TAG"; \
	else \
		echo "Tag creation cancelled"; \
	fi'
