compose := docker compose -f .docker/docker-compose.dev.yml

up:
	$(compose) up --build -d

logs:
	$(compose) logs -f gateway auth

down:
	$(compose) down -v

# Manually re-run migrations (optional helper)
migrate-up:
	$(compose) run --rm migrate-auth \
		-path=/migrations -database 'postgres://app:secret@postgres:5432/appdb?sslmode=disable' up

migrate-down:
	$(compose) run --rm migrate-auth \
		-path=/migrations -database 'postgres://app:secret@postgres:5432/appdb?sslmode=disable' down 1
