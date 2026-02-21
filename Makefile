.PHONY: build run test migrate-up migrate-down docker-up docker-down generate-keys swagger

build:
	go build -o bin/auth-service ./cmd/server

run:
	go run ./cmd/server

test:
	go test -v ./...

migrate-up:
	migrate -path migrations -database "postgres://auth_user:auth_secret@localhost:5433/auth_db?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://auth_user:auth_secret@localhost:5433/auth_db?sslmode=disable" down

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

swagger:
	swag init -g cmd/server/main.go

generate-keys:
	@if not exist keys mkdir keys
	openssl genrsa -out keys/private.pem 4096
	openssl rsa -in keys/private.pem -pubout -out keys/public.pem
