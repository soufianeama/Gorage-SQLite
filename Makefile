BINARY   = atelier-go
HTMX_VER = 2.0.4

.PHONY: all deps build build-windows run clean

all: build

## Télécharge htmx.min.js (nécessaire une seule fois)
deps:
	@echo ">>> Téléchargement de HTMX $(HTMX_VER)..."
	curl -sL https://unpkg.com/htmx.org@$(HTMX_VER)/dist/htmx.min.js \
	     -o static/htmx.min.js
	@echo ">>> Dépendances OK."

## Build Linux
build: deps
	go mod tidy
	go build -ldflags="-s -w" -o $(BINARY) .
	@echo ">>> Build Linux : ./$(BINARY)"

## Cross-compile Windows (.exe) — nécessite mingw-w64 pour CGO si sqlite mattn
build-windows: deps
	go mod tidy
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY).exe .
	@echo ">>> Build Windows : ./$(BINARY).exe"

## Lancement en mode développement
run: deps
	go mod tidy
	go run .

clean:
	rm -f $(BINARY) $(BINARY).exe gorage.db
