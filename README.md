# go-carshop-backend

## Initial plan
- car-service, which holds individual cars. The service has CRUD API endpoints. One of the endpoints can return a list of cars, which could be used by UI to show a list of available cars. Another endpoint can show a details of one car. Each car has availability status (available, reserved, sold).
- order-service, which accepts an order from customers. The service accepts an order only if the car is still available. The service has records of past orders. Each order has status, which is pending, confirmed, cancelled, perhaps failed. When an order is accepted, the service will sent a message to message-service to send an email to the customer.
- message-service, which receives a message from order-serice via a queue (I want to use Kafka, because I want to learn it). When the message is accepted, this service will send an email to the customer.

## Key learning opportunities to build in deliberately
- Make reserving a car concurrency-safe, so two customers cannot buy it.
- Use an idempotency key for order creation.
- Use Kafka events such as order.created.
- Implement the transactional outbox pattern eventually, so saving an order and publishing its event remain reliable even during failures.
- Make message-service idempotent, since Kafka consumers may receive messages more than once.
- Add Docker Compose early for local execution: PostgreSQL per service, Kafka, and the services.
- Add structured logging, health endpoints, and tracing once the basic workflow works.

## Future development plan
- Introduce versioned DB migration and remove repo.Migrate() / GORM AutoMigrate()
- payments, search, image storage, authentication, notifications, and a gateway

## Local development

Start PostgreSQL and Redis from the repository root:

```bash
docker compose up -d
```

Check that both containers are healthy:

```bash
docker compose ps
```

Then start the car service from its directory:

```bash
cd car-service
cp .env.example .env # only needed if .env does not already exist
go run ./cmd/web
```

Populate `cars` table by running this:
```bash
docker exec -i <postgres-container-name> psql -U car_service_user -d car_service < migrations/seed.sql
```

The service uses PostgreSQL on `localhost:5432` and Redis on `localhost:6379`.
Stop the dependencies with `docker compose down`. Their data persists in Docker volumes; use `docker compose down -v` only when you intentionally want to delete local database and cache data.

## Note:

### How to develop and run the app

Make sure the following ones are installed:

- Go (V1.27 or higher recommended)
- Docker

Create `.env` for each service by following `.env.example`.

To run a service locally for development purposes, run:
```bash
docker compose up postgres redis prometheus grafana -d
cd <service-folder-name>
go run ./cmd/web/
```
This will run the service and infra services separately.

To build the app's image and run it with the infra services by Docker Compose, run:
```bash
docker compose up --build -d
```

(Use `docker compose stop` for stopping containers, `docker compose start` for starting them again, and `docker compose down` for removing them. To remove containers and remove persistent volume, run `docker compose down -v`.)

### Docker command cheat sheet

```bash
# For example, you can examine Redis cache for car-service by:
docker exec -it car-service-redis redis-cli

# After running the command above, try following ones:
# KEYS cars:*
# KEYS car:*
# GET "<key>"
# MONITOR
# CONFIG GET maxmemory-policy
# CONFIG GET maxmemory

```

### Prometheus
- Go to http://localhost:9090/targets, make sure services like "http://car-service:8080/metrics" are UP.
- Go to http://localhost:9090, enter an item (e.g. app_cache_cars_hits_total) to the search box, hit Execute and click Graph

### Grafana
- Go to http://localhost:3000
- Use admin for both username and password.
- In the left side bar, click Connections -> Data sources -> Add data source -> Prometheus. Use http://host.docker.internal:9090 as Prometheus server URL -> Save & test.
- Then, create a dashboard. In the left sidebar, click Dashboards -> New -> New dashboard -> Add visualization. Select Prometheus as the data source.

### Swagger

Every time you change your annotations or DTOs, you need to regenerate the docs. Run this from the root of the service:
```
swag init -g cmd/web/main.go
```