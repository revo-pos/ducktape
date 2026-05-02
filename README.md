
## What is ducktape?

Fork of [artie-labs/ducktape](https://github.com/artie-labs/ducktape) used internally at Revo for feature implementation on a more rapid cadence than the parent.

## Quick start

### Development

```bash
make start
# Or with debug logging
make debug
# Or manually
PORT=8080 DUCKTAPE_LOG=debug go run cmd/main.go

# Health check
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready
```

Server runs on port 8080 by default.

## API usage

### Execute

Execute one or more SQL statements in a transaction:

```bash
curl -X POST http://localhost:8080/api/execute \
  -H "X-DuckDB-Connection-String: duck.db" \
  -H "Content-Type: application/json" \
  -d '{"statements": [
    {"query": "CREATE TABLE users (name TEXT, age INTEGER)"},
    {"query": "INSERT INTO users VALUES (?, ?)", "args": ["Alice", 30]},
    {"query": "INSERT INTO users VALUES (?, ?)", "args": ["Bob", 25]}
  ]}'
```

### Query

```bash
curl -X POST http://localhost:8080/api/query \
  -H "X-DuckDB-Connection-String: duck.db" \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT * FROM users WHERE name = ?", "args": ["Alice"]}'
```

#### Query after running an init script

* This example uses an init script stored in the container to set up a connection to a self-hosted ducklake and then query it.

`/sensitive/.duckdbrc`
```sql
--put your own info in where there are <>

INSTALL httpfs;
LOAD httpfs;   

CREATE OR REPLACE SECRET ducklake_storage (
    TYPE s3,
    PROVIDER config,
    KEY_ID '<KEY_ID>,
    SECRET '<KEY_SECRET>',
    REGION 'us-east-1',
    ENDPOINT '<ENDPOINT>',
    URL_STYLE 'path',
    USE_SSL 'true'
);

INSTALL ducklake;
INSTALL postgres;

ATTACH 'ducklake:postgres:dbname=reporting_ducklake host=<HOST> port=<PORT> user=reporting_ducklake password=<PW>' 
AS reporting__prod
( DATA_PATH 's3://revo-reporting/ducklake/reporting/prod', METADATA_SCHEMA 'reporting__prod', OVERRIDE_DATA_PATH true ) 
;

USE reporting__prod;

```
* Call with connection string set as `rcfile:<PATH TO FILE>`

```bash
# this will run the given file to set up the ducklake connection prior to running 'show schemas;'
curl -sS -X POST 'http://localhost:8080/api/query' \
--header 'X-DuckDB-Connection-String: rcfile:/sensitive/.duckdbrc' \
--header 'Content-Type: application/json' \
--data '{
    "Query": "show schemas;"
}'
```

### Append

Streams NDJSON data over HTTP/2. Each line is a `RowMessage` with a `rv` (row values) array. Use the Go client for streaming large datasets.

## Configuration

- `PORT`: Server port (default: `8080`)
- `DUCKTAPE_LOG`: Log level (`debug`, `info`, `warn`, `error`)
- `AUTH_USERNAME`: Optional username for HTTP Basic Auth
- `AUTH_PASSWORD`: Optional password for HTTP Basic Auth

For authentication to be enforced both AUTH_USERNAME and AUTH_PASSWORD must be set.

## Go client

- Install Go module for client.
  ```bash
  go get github.com/revo-pos/ducktape/api
  ```
- Usage:

  ```go
  import "github.com/revo-pos/ducktape/api/pkg/ducktape"

  client := ducktape.NewClient("http://localhost:8080")
  ```

- [Client source code](api/pkg/ducktape/client.go)

## License

MIT License. See [LICENSE](https://github.com/revo-pos/ducktape/blob/master/LICENSE.txt) for details.
