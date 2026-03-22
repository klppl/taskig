.PHONY: run build generate css

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

generate:
	templ generate

css:
	npx @tailwindcss/cli -i static/css/app.css -o static/css/dist.css --watch

dev:
	air
