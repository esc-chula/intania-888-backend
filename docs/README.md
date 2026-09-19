# Swagger documentation

> **Breaking backend release:** Money values are fixed two-decimal JSON strings, bill rates are server-owned, and bill/match lifecycle routes changed. The current frontend is incompatible until its separate migration.

The API documentation is generated from Go annotations with `swag` and served
by Fiber's Swagger UI middleware.

## Access

Development:

```text
http://localhost:8080/swagger/index.html
```

Production:

```text
https://888api.chula.engineering/swagger/index.html
```

The development configuration leaves the UI open. Production requires HTTP
Basic Auth using `SWAGGER_USERNAME` and `SWAGGER_PASSWORD`.

The runtime OpenAPI document is available at `/swagger/doc.json`.

After the UI loads, use its **Authorize** button with `Bearer <access-token>`
to call endpoints protected by the API's JWT middleware.

## Generate the specification

Regenerate the tracked files after changing Swagger annotations:

```bash
make swagger
```

Check that the tracked generated files are current without changing them:

```bash
make swagger-check
```

The API target used by **Try it out** comes from `SERVER_URL`, including the
`/api/v1` base path.
