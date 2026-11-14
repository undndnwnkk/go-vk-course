# db_explorer

A small HTTP service that exposes CRUD access to any MySQL database. It uses only the Go standard library (`net/http`, `database/sql`, `encoding/json`) plus the `github.com/go-sql-driver/mysql` driver imported in `main.go`.

## API

| Method   | Path                          | Description                                                    |
|----------|-------------------------------|----------------------------------------------------------------|
| `GET`    | `/`                           | Lists all tables                                               |
| `GET`    | `/$table?limit=5&offset=7`    | Lists records (default `limit=5`, `offset=0`)                  |
| `GET`    | `/$table/$id`                 | Returns one record by primary key, or `404`                    |
| `PUT`    | `/$table`                     | Creates a record from the JSON body and returns the new primary key |
| `POST`   | `/$table/$id`                 | Updates a record from the JSON body and returns the number of changed rows |
| `DELETE` | `/$table/$id`                 | Deletes a record and returns the number of deleted rows        |

Successful responses look like `{"response": ...}`. Errors look like `{"error": "..."}`:

- `404 unknown table`: the table does not exist
- `404 record not found`: no record has that primary key
- `400 field <name> have invalid type`: validation failed
- `405 method not allowed`: the method is not supported for this path
- `500 internal error`: a database error (the details go to the server log only)

## Project layout

- `main.go`: the provided entry point (not modified).
- `db_explorer.go`: the whole solution in one file: `NewDBExplorer`, routing, request handlers, JSON helpers, schema loading at startup, and the column logic (type detection, validation, scanning, NULL handling).
- `main_test.go`: the provided integration tests.
- `sample_db.sql`: the sample database dump.

## How it works

### Dynamic schema

`NewDBExplorer` reads the database structure once, at startup:

1. `SHOW TABLES` gives the table list. Each `rows` object is closed before the next query, so initialization never holds more than one connection.
2. `SHOW FULL COLUMNS FROM` is run for each table. Its result is read by column name through `rows.Columns()`, so the code does not depend on the column order of that output. For each field it stores:
   - a **kind** (`int`, `float` or `string`) taken from the SQL type (`int(11)`, `bigint unsigned`, `decimal(10,2)`, `varchar(255)`, …);
   - whether it is **nullable**, has a **default**, is the **primary key**, and is **auto_increment**.

The schema is stored in a `DBExplorer` struct that implements `http.Handler`, so the program uses no global variables. The service never hardcodes table or column names. If you add a new table and restart, it works with no code changes.

### Routing

Routing is done by hand. The path is trimmed of `/` and split into segments: zero segments means the table list, one means `/$table`, and two mean `/$table/$id`. The first segment is looked up in the loaded schema, so an unknown table is rejected before any SQL runs. A trailing slash is accepted (`PUT /items/` works).

### Reading rows with an unknown set of columns

Each `SELECT` names its columns explicitly, in the order the schema loaded them. A scan target is built per column from its kind (`sql.NullInt64`, `sql.NullFloat64` or `sql.NullString`) and passed to `rows.Scan` as `[]interface{}`. The values are then unwrapped into a `map[string]interface{}`, with SQL `NULL` turned into JSON `null`. This returns real JSON numbers for integer columns, whether the driver used the text protocol or the binary one. Every `rows` is closed with `defer`, so no connections leak (the tests check that exactly one connection stays open).

### Validation

The JSON body is decoded into `map[string]interface{}`, so all JSON numbers arrive as `float64`. Each value is checked against its column:

- `null` is allowed only if the column is nullable;
- `string` columns need a JSON string;
- `int` columns need a JSON number with no fractional part (it is sent to the DB as `int64`);
- `float` columns need a JSON number.

Fields that are not in the table are ignored.

### Create (`PUT`)

- An auto_increment primary key in the body is ignored. Its value comes from `LastInsertId()`, and the response uses the primary key's real name (for example `{"user_id": 2}`).
- A NOT NULL column that has no default and is missing from the body gets the zero value for its kind (`""`, `0`, `0.0`). Nullable columns and columns with a default are left to MySQL.

### Update (`POST`)

- Changing the primary key is not allowed and returns `400 field <pk> have invalid type`.
- The `UPDATE` statement sets only the fields given in the body.

### SQL injection protection

- All values (`limit`, `offset`, `id`, and field values) are passed as `?` placeholders and are never concatenated into the SQL.
- Table and column names cannot be placeholders. They always come from the loaded schema, never straight from the request, and are wrapped in backticks with any inner backtick doubled.
- If `limit` or `offset` is not a valid non-negative integer (for example `limit=1'`), the default value is used.

## Running

```bash
docker run -p 3306:3306 -v $(PWD):/docker-entrypoint-initdb.d \
  -e MYSQL_ROOT_PASSWORD=1234 -e MYSQL_DATABASE=golang -d mysql

go test -v ./...   # integration tests
go run .           # server on :8082
```

Example:

```bash
curl localhost:8082/
curl "localhost:8082/items?limit=1&offset=1"
curl -X PUT  localhost:8082/items -d '{"title":"new","description":"text"}'
curl -X POST localhost:8082/items/3 -d '{"updated":null}'
curl -X DELETE localhost:8082/items/3
```
