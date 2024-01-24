VERSION=$(shell git describe --abbrev=0 --always)
LDFLAGS = -ldflags "-s -w -X github.com/ForceCLI/force/lib.Version=${VERSION}"
LINUX_LDFLAGS = -ldflags "-s -w -extldflags '-static' -X github.com/ForceCLI/force/lib.Version=${VERSION}"
GCFLAGS = -gcflags="all=-N -l"
EXECUTABLE=force
PACKAGE=.
WINDOWS=$(EXECUTABLE)-windows-amd64.exe
LINUX=$(EXECUTABLE)-linux-amd64
OSX_AMD64=$(EXECUTABLE)-darwin-amd64
OSX_ARM64=$(EXECUTABLE)-darwin-arm64
ALL=$(WINDOWS) $(LINUX) $(OSX_AMD64) $(OSX_ARM64)

default:
	go build ${LDFLAGS}

install:
	go install ${LDFLAGS}

install-debug:
	go install ${LDFLAGS} ${GCFLAGS}

$(WINDOWS): checkcmd-xgo
	xgo -go 1.21 -out $(EXECUTABLE) -dest . ${LDFLAGS} -buildmode default -trimpath -targets windows/amd64 -pkg ${PACKAGE} -x .

# Build static binaries on linux
$(LINUX): checkcmd-x86_64-linux-gnu-gcc checkcmd-x86_64-linux-gnu-g++
	env \
		GOOS=linux \
		GOARCH=amd64 \
		CC=x86_64-linux-gnu-gcc \
		CXX=x86_64-linux-gnu-g++ \
		CGO_ENABLED=1 \
		CGO_FLAGS="-static"
		go build -v -tags 'netgo osusergo' -o $(LINUX) ${LINUX_LDFLAGS} ${PACKAGE}

# Build macOS binaries using docker images that contain SDK
# See https://github.com/crazy-max/xgo and https://github.com/tpoechtrager/osxcross
$(OSX_ARM64): checkcmd-xgo
	xgo -go 1.21 -out $(EXECUTABLE) -dest . ${LDFLAGS} -buildmode default -trimpath -targets darwin/arm64 -pkg ${PACKAGE} -x .

$(OSX_AMD64): checkcmd-xgo
	xgo -go 1.21 -out $(EXECUTABLE) -dest . ${LDFLAGS} -buildmode default -trimpath -targets darwin/amd64 -pkg ${PACKAGE} -x .

$(basename $(WINDOWS)).zip: $(WINDOWS)
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

%.zip: %
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)

docs:
	go run docs/mkdocs.go

dist: test $(addsuffix .zip,$(basename $(ALL)))

fmt:
	go fmt ./...

test:
	test -z "$(go fmt)"
	go vet
	go test ./...
	go test -race ./...

clean:
	-rm -f $(EXECUTABLE) $(EXECUTABLE)_*

checkcmd-%:
	@hash $(*) > /dev/null 2>&1 || \
		(echo "ERROR: '$(*)' must be installed and available on your PATH."; exit 1)

.PHONY: default dist clean docs
