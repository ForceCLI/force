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
ZIPS=$(addsuffix _$(VERSION).zip,$(basename $(ALL)))
RELEASE_ASSETS=$(ZIPS) SHA256SUMS-$(VERSION)
# The license of force and the notices that the licenses of third-party
# material in the binary require to accompany it.  Every release zip includes
# these files.
LICENSES := LICENSE third-party-licenses.txt
# Source of the file templates in sf/templates.
SF_TEMPLATES_REPO=https://github.com/forcedotcom/salesforcedx-templates
SF_TEMPLATES_VERSION=66.9.1
# Checkout of https://github.com/forcedotcom/source-deploy-retrieve that
# sf/registry_data.go is generated from.
SF_REGISTRY_DIR=../source-deploy-retrieve

default:
	go build ${LDFLAGS}

install:
	go install ${LDFLAGS}

# Install force and create an sf symlink next to it.
install-sf: install
	ln -s -f force "$(shell go env GOPATH)/bin/sf"

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

$(basename $(WINDOWS))_$(VERSION).zip: $(WINDOWS) $(LICENSES)
	@rm -f $@
	zip $@ $< $(LICENSES)
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

$(basename $(WINDOWS_ARM64))_$(VERSION).zip: $(WINDOWS_ARM64) $(LICENSES)
	@rm -f $@
	zip $@ $< $(LICENSES)
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

$(basename $(OSX_AMD64))_$(VERSION).zip: $(OSX_AMD64) $(LICENSES)
	@rm -f $@
	zip $@ $< $(LICENSES)
	7za rn $@ $< $(EXECUTABLE)
	rcodesign notary-submit --api-key-file <(pass OctoberSwimmer/codesign/api-key) $@

$(basename $(OSX_ARM64))_$(VERSION).zip: $(OSX_ARM64) $(LICENSES)
	@rm -f $@
	zip $@ $< $(LICENSES)
	7za rn $@ $< $(EXECUTABLE)
	rcodesign notary-submit --api-key-file <(pass OctoberSwimmer/codesign/api-key) $@

%_$(VERSION).zip: % $(LICENSES)
	@rm -f $@
	zip $@ $< $(LICENSES)
	7za rn $@ $< $(EXECUTABLE)

docs:
	go run docs/mkdocs.go

# Rebuild the template archives in sf/templates and their attribution in
# licenses/ from a tag of the source repository, then the license report:
#   make update-sf-templates SF_TEMPLATES_VERSION=66.9.3
update-sf-templates:
	go run sf/templates/update.go $(SF_TEMPLATES_REPO) $(SF_TEMPLATES_VERSION) .
	$(MAKE) third-party-licenses.txt
	go test ./sf/...

# Regenerate sf/registry_data.go and its attribution in licenses/ from the
# metadata registry in a checkout of source-deploy-retrieve, then the license
# report:
#   make update-sf-registry SF_REGISTRY_DIR=~/git/source-deploy-retrieve
update-sf-registry:
	go run sf/registry/update.go $(SF_REGISTRY_DIR) .
	$(MAKE) third-party-licenses.txt
	go test ./sf/...

# Attributions for the Go modules compiled into the binary, followed by the
# attributions in licenses/ for third-party material that is not a Go module
# dependency, so go-licenses cannot find it by walking the module graph.
third-party-licenses.txt: go.mod licenses.tpl $(wildcard licenses/*.txt)
	go run github.com/google/go-licenses@v1.6.0 report ./... --ignore github.com/ForceCLI/force --template licenses.tpl > $@
	@if ls licenses/*.txt >/dev/null 2>&1; then cat licenses/*.txt >> $@; fi

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
	brew bump-cask-pr octoberswimmer/tap/force --version $(VERSION:v%=%)

fmt:
	go fmt ./...

test:
	test -z "$(go fmt)"
	go vet
	go test ./...
	go test -race ./...

clean:
	-rm -f $(EXECUTABLE) $(EXECUTABLE)_* *.zip SHA256SUMS-*

.PHONY: default dist clean docs checksum release tag install-sf update-sf-templates update-sf-registry $(ALL)

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
