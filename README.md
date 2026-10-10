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
- Introduce refresh tokens to user-service. When a user logs in, user-service creates an access token and a refresh token and saves the refresh token's hash in a refresh_tokens table with the user ID and an expiry. When the access token is expired, the UI sends the refresh token to POST /refresh-token. Then, user-service issues a new access token. This requires a refresh_tokens table, /login-user returning a refresh token, POST /refresh-token, POST /logout revoking an existing refresh token, and an access-token TTL of about 15 minutes.
- Add structured logging, health endpoints, and tracing once the basic workflow works.
- Add a centralized key-distribution endpoint: user-service exposes something like `GET /.well-known/jwks.json` serving its current public key(s) in a standard format. Instead of reading a static file once at startup, car-service, order-service, etc periodically fetch and cache that endpoint (say, refresh every few minutes, or re-fetch whenever they see a token signed with an unfamiliar kid).
- Once Kubernets is introduced, try centralized secret-management tooling (e.g., Vault, AWS Secrets Manager) for key rotation and replace the centralized key-distribution endpoint with it. 

## Future development plan
- Introduce versioned DB migration and remove repo.Migrate() / GORM AutoMigrate()
- payments, search, image storage, authentication, notifications, and a gateway

## Local development

### Keys for access tokens

user-service signs access tokens (JWTs) with a private key. The other services verify those tokens with the matching public key, so they never need the private key.

Generate the key pair from the root of the `user-service` folder:
```bash
mkdir -p keys
openssl genrsa -out keys/private.pem 2048
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
```

Then copy the public key into every service that verifies access tokens. From the repository root:
```bash
mkdir -p car-service/keys order-service/keys
cp user-service/keys/public.pem car-service/keys/public.pem
cp user-service/keys/public.pem order-service/keys/public.pem
```

You should end up with:
- `user-service/keys/private.pem` and `user-service/keys/public.pem`
- `car-service/keys/public.pem`
- `order-service/keys/public.pem`

The `keys/` folders are gitignored. Docker Compose mounts these files into the containers, so they are not baked into any image. If you regenerate the key pair, copy the new `public.pem` again and restart the services; tokens signed with the old key stop being valid.

### Running a service locally

Start the databases and Redis from the repository root:

```bash
docker compose up -d car-service-db order-service-db user-service-db redis
```

Check that the containers are healthy:

```bash
docker compose ps
```

Then start the car service from its directory:

```bash
cd car-service
cp .env.example .env # only needed if .env does not already exist
go run ./cmd/web
```

Populate `cars` table by running this from the `car-service` directory:
```bash
docker exec -i car-service-db psql -U car_service_user -d car_service < migrations/seed.sql
```

The service uses PostgreSQL on `localhost:5432` and Redis on `localhost:6379`.
Stop the dependencies with `docker compose down`. Their data persists in Docker volumes; use `docker compose down -v` only when you intentionally want to delete local database and cache data.

## Note:

### How to develop and run the app

Make sure the following ones are installed:

- Go (V1.27 or higher recommended)
- Docker

Create `.env` for each service by following `.env.example`.

There are two ways to run the services. Either way, the databases are reachable from your machine on `localhost:5432` (car-service), `localhost:5433` (order-service) and `localhost:5434` (user-service).

**Everything in Docker** (for example, to try the endpoints with Postman):
```bash
docker compose up --build -d
```
car-service is then at http://localhost:8080, order-service at http://localhost:8081 and user-service at http://localhost:8082.

**One service on your machine, the rest in Docker** (for example, to debug it or to iterate quickly): start everything, stop the container of the service you are working on, and run that service from its folder:
```bash
docker compose up --build -d
docker compose stop car-service   # the container would otherwise hold port 8080
cd car-service
go run ./cmd/web/
```
When you are done, `docker compose start car-service` brings the container back.

The `.env` files copied from `.env.example` use `localhost` and the host-side database ports, which is what a locally-run service needs. Inside Docker, `docker-compose.yaml` overrides them (`DB_HOST`, `DB_PORT`, `CAR_SERVICE_URL`, and the key paths) so the containers reach each other by service name.

To run *all* services on your machine instead, start only the infra:
```bash
docker compose up -d car-service-db order-service-db user-service-db redis prometheus grafana
```

Note that Prometheus scrapes `car-service:8080` inside the Docker network, so its target shows as DOWN while car-service runs on your machine instead of in a container.

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
