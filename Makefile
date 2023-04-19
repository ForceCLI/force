VERSION=$(shell git describe --abbrev=0 --always)
LDFLAGS = -ldflags "-X github.com/ForceCLI/force/lib.Version=${VERSION}"
GCFLAGS = -gcflags="all=-N -l"
EXECUTABLE=force
WINDOWS=$(EXECUTABLE)_windows_amd64.exe
LINUX=$(EXECUTABLE)_linux_amd64
OSX_AMD64=$(EXECUTABLE)_osx_amd64
OSX_ARM64=$(EXECUTABLE)_osx_arm64
ALL=$(WINDOWS) $(LINUX) $(OSX_AMD64) $(OSX_ARM64)

default:
	go build ${LDFLAGS}

install:
	go install ${LDFLAGS}

install-debug:
	go install ${LDFLAGS} ${GCFLAGS}

$(WINDOWS):
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -v -o $(WINDOWS) ${LDFLAGS}

$(LINUX):
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o $(LINUX) ${LDFLAGS}

$(OSX_AMD64):
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -v -o $(OSX_AMD64) ${LDFLAGS}

$(OSX_ARM64):
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -v -o $(OSX_ARM64) ${LDFLAGS}

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

.PHONY: default dist clean docs
