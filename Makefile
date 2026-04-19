APP=music-bot

.PHONY: tidy test build run docker-build docker-up deploy-debian13

tidy:
	go mod tidy

test:
	go test ./...

build:
	go build -o bin/$(APP) ./cmd/bot

run:
	go run ./cmd/bot

docker-build:
	docker compose build

docker-up:
	docker compose up -d

deploy-debian13:
	sudo bash deploy/debian13-oneclick.sh
