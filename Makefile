# heya — build, test and package helpers

BINARY := heya
FONT_SHOTS ?= build/font-review
GOFLAGS ?=
PREFIX ?= /usr
DESTDIR ?=

.PHONY: all build test vet fmt dump dump-fonts check-fonts arm64 clean install

all: build

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/heya

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

# Render every page to PNG without a display (SDL dummy driver).
dump:
	go run ./cmd/heya --dump build/frames --theme themes/default/theme.json --apps examples/apps

# Render the CJK font review set (specimens + app lists, both themes, scales 8/16/32).
dump-fonts:
	rm -rf $(FONT_SHOTS)
	@mkdir -p $(FONT_SHOTS)
	go run ./cmd/heya --dump $(FONT_SHOTS)/default --theme themes/default/theme.json --apps examples/font-samples
	go run ./cmd/heya --dump $(FONT_SHOTS)/light --theme themes/light/theme.json --apps examples/font-samples
	go run ./cmd/heya --scale 8 --dump $(FONT_SHOTS)/scale-8 --theme themes/default/theme.json --apps examples/font-samples
	go run ./cmd/heya --scale 32 --dump $(FONT_SHOTS)/scale-32 --theme themes/default/theme.json --apps examples/font-samples
	@echo "wrote $(FONT_SHOTS)"

# Verify the font-review screenshots are not blank.
check-fonts:
	python3 scripts/check_font_shots.py $(FONT_SHOTS)

# Cross-compile for ARM64 handhelds. Requires an aarch64 SDL2 sysroot:
#   CC=aarch64-linux-gnu-gcc PKG_CONFIG_PATH=<sysroot>/usr/lib/pkgconfig make arm64
arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o $(BINARY)-arm64 ./cmd/heya

install: build
	install -Dm755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	install -Dm644 assets/icons.json $(DESTDIR)$(PREFIX)/share/heya/assets/icons.json
	install -Dm644 assets/JetBrainsMonoNerdFont-Regular.ttf $(DESTDIR)$(PREFIX)/share/heya/assets/JetBrainsMonoNerdFont-Regular.ttf
	install -Dm644 themes/default/theme.json $(DESTDIR)$(PREFIX)/share/heya/themes/default/theme.json

clean:
	rm -rf $(BINARY) $(BINARY)-arm64 build
