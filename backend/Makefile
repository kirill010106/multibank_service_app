include .env
export

.PHONY: migrate-up migrate-down migrate-create migrate-force

# Накатить все миграции
migrate-up:
    migrate -path migrations -database "$(DATABASE_URL)" up

# Откатить последнюю миграцию
migrate-down:
    migrate -path migrations -database "$(DATABASE_URL)" down 1

# Откатить все миграции
migrate-down-all:
    migrate -path migrations -database "$(DATABASE_URL)" down -all

# Создать новую миграцию (usage: make migrate-create name=add_bank_connections)
migrate-create:
    migrate create -ext sql -dir migrations -seq $(name)

# Форсировать версию (для фикса битых миграций)
migrate-force:
    migrate -path migrations -database "$(DATABASE_URL)" force $(version)

# Показать текущую версию
migrate-version:
    migrate -path migrations -database "$(DATABASE_URL)" version