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
- payments, search, image storage, authentication, notifications, and a gateway