# go-carshop-backend

## Initial plan
- car-service, which holds individual cars. The service has CRUD API endpoints. One of the endpoints can return a list of cars, which could be used by UI to show a list of available cars. Another endpoint can show a details of one car. Each car has availability status (available, reserved, sold).
- order-service, which accepts an order from customers. The service accepts an order only if the car is still available. The service has records of past orders. Each order has status, which is pending, confirmed, cancelled, perhaps failed. When an order is accepted, the service will sent a message to message-service to send an email to the customer.
- message-service, which receives a message from order-serice via a queue (I want to use Kafka, because I want to learn it). When the message is accepted, this service will send an email to the customer.

## Key learning opportunities to build in deliberately
- Make reserving a car concurrency-safe, so two customers cannot buy it. -> DONE
- Make `POST /orders` safe to retry. A unique index already allows only one live (pending or confirmed) order per car. A retry by the same customer returns their original order, and completes it if it was still pending, instead of failing with 409. A car is one of a kind, so the car ID acts as the idempotency key. An explicit `Idempotency-Key` header would be needed if a customer could legitimately order the same thing twice. -> DONE
- Allow car-service to release a car upon order cancellation. -> DONE
- Use Kafka events such as order.created.
- Implement the transactional outbox pattern eventually, so saving an order and publishing its event remain reliable even during failures.
- Make message-service idempotent, since Kafka consumers may receive messages more than once.
- Introduce refresh tokens to user-service. When a user logs in, user-service creates an access token and a refresh token and saves the refresh token's hash in a refresh_tokens table with the user ID and an expiry. When the access token is expired, the UI sends the refresh token to POST /refresh-token. Then, user-service issues a new access token. This requires a refresh_tokens table, /login-user returning a refresh token, POST /refresh-token, POST /logout revoking an existing refresh token, and an access-token TTL of about 15 minutes.
- Add structured logging, health endpoints, and tracing once the basic workflow works.
- Add a centralized key-distribution endpoint: user-service exposes something like `GET /.well-known/jwks.json` serving its current public key(s) in a standard format. Instead of reading a static file once at startup, car-service, order-service, etc periodically fetch and cache that endpoint (say, refresh every few minutes, or re-fetch whenever they see a token signed with an unfamiliar kid).
- Once Kubernets is introduced, try centralized secret-management tooling (e.g., Vault, AWS Secrets Manager) for key rotation and replace the centralized key-distribution endpoint with it. 
- Find out how to test that reserving a car is concurrency-safe end-to-end.
- Refactor test cases that are duplicated.

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

### Tokens for service-to-service calls

When one service calls an internal endpoint of another (for example order-service reserving a car in car-service), it does not use a customer's token. It authenticates as itself with the OAuth2 client-credentials grant and receives a short-lived token with scopes such as `cars:reserve`. user-service issues these tokens at `POST /oauth/token`, signed with the same key as user tokens, so car-service verifies them with the public key it already has. The endpoint it calls checks the scope, so a customer or admin token can't use it.

Give each client a secret. Generate one with `openssl rand -hex 32` and put it in `user-service/.env` (see `.env.example`):
```bash
ORDER_SERVICE_CLIENT_SECRET=<output of openssl rand -hex 32>
```
user-service refuses to start without it. order-service gets the same value as its client secret. Stick to hex or alphanumeric secrets: HTTP clients that don't form-encode the Basic credentials, as RFC 6749 section 2.3.1 requires, mangle characters such as `+` and `%`.

Request a token (`-u` sends the client ID and secret as HTTP Basic credentials). The secret lives in `user-service/.env`, which your terminal does not read, so load it into your shell first. If the variable is empty, curl sends an empty password and you get `invalid_client`:
```bash
export ORDER_SERVICE_CLIENT_SECRET=$(grep '^ORDER_SERVICE_CLIENT_SECRET=' user-service/.env | cut -d= -f2-)
curl -s -u "order-service:$ORDER_SERVICE_CLIENT_SECRET" \
  -d grant_type=client_credentials -d scope=cars:reserve \
  http://localhost:8082/oauth/token
```
The response has `access_token`, `token_type`, `expires_in` (seconds) and `scope`. Without a `scope` parameter the token gets every scope the client is allowed. Use it as `Authorization: Bearer <access_token>` on the internal endpoint, for example `POST /internal/cars/{id}/reservations` in car-service.

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

### Running the tests

Most tests need nothing but Go:

```bash
cd car-service
go test ./...
```

The tests about concurrency (for example, many customers reserving the same car at once) need a real PostgreSQL, because an in-memory SQLite database serializes every write and would hide the race. They are skipped unless `TEST_POSTGRES_DSN` points at a Postgres. With the databases from `docker compose up -d` running:

```bash
cd car-service
TEST_POSTGRES_DSN="postgres://car_service_user:password@localhost:5432/car_service?sslmode=disable" go test ./...
```

Each such test creates its own throwaway schema and drops it when it finishes, so the data the app uses is never touched. In CI, run a Postgres container next to the job (for example a `services:` block in GitHub Actions) and set the same variable.

### Testing concurrent orders

Replace `$TOKEN` with an actual token and `$CAR_ID` with a car ID.

```
for i in $(seq 1 10); do
  curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8081/orders \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"car_id": $CAR_ID}' &
done; wait
```


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
