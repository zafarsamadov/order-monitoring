# ---- app -----------------------------------------------------------------

.PHONY: run build test test-db-up test-db-down

run:
	go run ./cmd

build:
	go build -o bin/server ./cmd

# ---- tests ---------------------------------------------------------------
# Sync testlari real Postgres talab qiladi (TEST_DATABASE_URL). Usiz ular skip bo'ladi.

TEST_DATABASE_URL ?= postgres://postgres:test@localhost:55432/om_test?sslmode=disable

test:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

test-db-up:
	docker run -d --rm --name om-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=om_test -p 55432:5432 postgres:17-alpine

test-db-down:
	docker stop om-test-pg
