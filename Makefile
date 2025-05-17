MIGRATIONS_PATH=./cmd/migrate/migrations
DB_MIGRATION_ADDR=postgres://admin:password@localhost:5431/birdlens?sslmode=disable

.PHONY: migrate-create
migration:
	@migrate create -seq -ext sql -dir $(MIGRATIONS_PATH) $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_MIGRATION_ADDR) up

.PHONY: migrate-down
migrate-down:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_MIGRATION_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

