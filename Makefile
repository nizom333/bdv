# bdv: сборка и установка.
#   make install          собрать и положить в ~/.local/bin
#   make install PREFIX=/usr/local
#   make test             vet + тесты

PREFIX ?= $(HOME)/.local
BIN := $(PREFIX)/bin

# Версия для bdv --version: тег из git (0.1.0-3-gabc1234 — после тега); без тега — dev, коммит всё равно в скобках.
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build install test

build:
	go build -ldflags="$(LDFLAGS)" -o bdv .

install: build
	mkdir -p $(BIN)
	mv -f bdv $(BIN)/bdv
	@echo "установлено: $(BIN)/bdv"
	@case ":$$PATH:" in *":$(BIN):"*) ;; *) echo "добавьте в ~/.zshrc: export PATH=\"$(BIN):\$$PATH\"";; esac
	@echo "дальше: cd <проект> && bdv"

test:
	go vet ./...
	go test ./...
