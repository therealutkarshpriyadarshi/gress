# SQL Query Layer for Gress

## Overview

The SQL Query Layer enables SQL-like querying capabilities over streaming data in Gress. It provides a familiar SQL syntax for complex stream processing operations, making it easier to perform analytics on real-time data streams.

## Features

- ✅ **SQL Parser**: Custom parser supporting streaming SQL syntax
- ✅ **Query Planner**: Converts SQL queries to stream operators
- ✅ **Query Optimizer**: Rule-based optimization for efficient execution
- ✅ **Stream-Table Duality**: Bidirectional conversion between streams and tables
- ✅ **Continuous Queries**: Long-running queries that process data as it arrives
- ✅ **JOIN Support**: Inner, Left, Right, and Full Outer joins
- ✅ **GROUP BY & Aggregations**: COUNT, SUM, AVG, MIN, MAX, STDDEV
- ✅ **Window Support**: Tumbling, Sliding, Session, and Hopping windows
- ✅ **UDF (User Defined Functions)**: Extensible function system
- ✅ **Interactive Console**: CLI for executing queries interactively

## Architecture

```
┌─────────────┐
│   SQL Query │
└──────┬──────┘
       │
       v
┌─────────────┐
│   Parser    │  ← Parses SQL into AST
└──────┬──────┘
       │
       v
┌─────────────┐
│  Validator  │  ← Validates against catalog
└──────┬──────┘
       │
       v
┌─────────────┐
│   Planner   │  ← Creates execution plan
└──────┬──────┘
       │
       v
┌─────────────┐
│  Optimizer  │  ← Applies optimization rules
└──────┬──────┘
       │
       v
┌─────────────┐
│  Executor   │  ← Executes query
└──────┬──────┘
       │
       v
┌─────────────┐
│   Results   │
└─────────────┘
```

## Quick Start

### 1. Setup

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/sql"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// Create stream engine
engine := stream.NewEngine(stream.EngineConfig{
    BufferSize:     10000,
    MaxConcurrency: 100,
})

// Create SQL catalog
catalog := sql.NewCatalog()

// Register a stream
eventChannel := make(chan *stream.Event, 1000)
schema := sql.Schema{
    Columns: []sql.Column{
        {Name: "user_id", Type: sql.DataTypeString},
        {Name: "amount", Type: sql.DataTypeFloat},
        {Name: "timestamp", Type: sql.DataTypeTimestamp},
    },
}
catalog.RegisterStream("transactions", schema, eventChannel)

// Create executor
executor := sql.NewExecutor(catalog, engine)
```

### 2. Execute Queries

```go
// Simple SELECT
result, err := executor.Execute(`
    SELECT user_id, amount
    FROM transactions
    LIMIT 10
`)

// Aggregation with windowing
result, err := executor.Execute(`
    SELECT user_id, COUNT(*) as tx_count, AVG(amount) as avg_amount
    FROM transactions
    WINDOW TUMBLING (SIZE 5 MINUTES)
    GROUP BY user_id
    HAVING COUNT(*) > 10
`)
```

## SQL Syntax

### SELECT Statement

```sql
SELECT <columns>
FROM <stream>
[JOIN <stream> ON <condition>]
[WHERE <condition>]
[WINDOW <window_spec>]
[GROUP BY <columns>]
[HAVING <condition>]
[ORDER BY <columns>]
[LIMIT <number>]
```

### Supported Features

#### 1. Projections

```sql
-- Select specific columns
SELECT user_id, amount FROM transactions

-- Select all columns
SELECT * FROM transactions

-- Column aliases
SELECT user_id, amount as price FROM transactions
```

#### 2. Filtering (WHERE)

```sql
SELECT * FROM transactions
WHERE amount > 100

SELECT * FROM transactions
WHERE status = 'completed' AND amount > 50
```

#### 3. Aggregations

Supported aggregate functions:
- `COUNT(*)` / `COUNT(column)`
- `SUM(column)`
- `AVG(column)`
- `MIN(column)`
- `MAX(column)`
- `STDDEV(column)`

```sql
SELECT
    area,
    COUNT(*) as ride_count,
    AVG(price) as avg_price,
    MIN(price) as min_price,
    MAX(price) as max_price
FROM ride_requests
GROUP BY area
```

#### 4. Windows

**Tumbling Windows** (fixed, non-overlapping):
```sql
SELECT area, COUNT(*) as count
FROM ride_requests
WINDOW TUMBLING (SIZE 5 MINUTES)
GROUP BY area
```

**Sliding Windows** (overlapping):
```sql
SELECT area, COUNT(*) as count
FROM ride_requests
WINDOW SLIDING (SIZE 10 MINUTES, SLIDE 5 MINUTES)
GROUP BY area
```

**Session Windows** (gap-based):
```sql
SELECT user_id, COUNT(*) as activity_count
FROM user_events
WINDOW SESSION (GAP 30 MINUTES)
GROUP BY user_id
```

#### 5. Joins

**Inner Join**:
```sql
SELECT o.order_id, c.name, o.amount
FROM orders o
JOIN customers c ON o.customer_id = c.id
```

**Left Outer Join**:
```sql
SELECT o.order_id, c.name
FROM orders o
LEFT JOIN customers c ON o.customer_id = c.id
```

**Right Outer Join**:
```sql
SELECT o.order_id, c.name
FROM orders o
RIGHT JOIN customers c ON o.customer_id = c.id
```

**Full Outer Join**:
```sql
SELECT o.order_id, c.name
FROM orders o
FULL JOIN customers c ON o.customer_id = c.id
```

#### 6. HAVING Clause

```sql
SELECT area, COUNT(*) as ride_count
FROM ride_requests
WINDOW TUMBLING (SIZE 5 MINUTES)
GROUP BY area
HAVING COUNT(*) > 100
```

#### 7. ORDER BY and LIMIT

```sql
SELECT user_id, amount
FROM transactions
ORDER BY amount DESC
LIMIT 10
```

## Continuous Queries

Continuous queries run indefinitely and process data as it arrives.

### Starting a Continuous Query

```go
result, err := executor.Execute(`
    SELECT area, COUNT(*) as count
    FROM ride_requests
    WINDOW TUMBLING (SIZE 5 MINUTES)
    GROUP BY area
`)

// Get query ID from result
queryID := result.Rows[0][0].(string)

// Subscribe to results
cq, _ := executor.GetContinuousQuery(queryID)
for event := range cq.OutputChannel {
    fmt.Printf("Result: %v\n", event.Value)
}
```

### Stopping a Continuous Query

```go
err := executor.StopContinuousQuery(queryID)
```

### Listing Continuous Queries

```go
queries := executor.ListContinuousQueries()
for _, cq := range queries {
    fmt.Printf("Query ID: %s, Events: %d\n", cq.ID, cq.EventsProcessed)
}
```

## Stream-Table Duality

### Materialize Stream to Table

Convert a stream into a continuously updated table:

```go
streamTable := sql.NewStreamTableDuality(catalog, stateBackend)

// Materialize stream to table
err := streamTable.MaterializeStream(
    "transactions",      // source stream
    "transactions_table", // target table
    []string{"tx_id"},   // primary keys
)
```

### Streamify Table

Convert table changes into a stream:

```go
err := streamTable.StreamifyTable(
    "transactions_table", // source table
    "tx_changes",         // target stream
)
```

### Query Table Snapshot

```go
// Get current table state
snapshot, err := streamTable.GetTableSnapshot("transactions_table")

// Get table size
size, err := streamTable.GetTableSize("transactions_table")

// Query with predicate
results, err := streamTable.QueryTable("transactions_table",
    func(key string, value interface{}) bool {
        // Custom filter logic
        return true
    })
```

## User Defined Functions (UDFs)

### Built-in UDFs

String Functions:
- `upper(string)` - Convert to uppercase
- `lower(string)` - Convert to lowercase
- `concat(string...)` - Concatenate strings
- `substring(string, start, length)` - Extract substring

Math Functions:
- `abs(number)` - Absolute value
- `round(number)` - Round to nearest integer

Utility Functions:
- `coalesce(value...)` - Return first non-null value
- `to_string(value)` - Convert to string

### Registering Custom UDFs

**Scalar UDF**:
```go
udfRegistry := sql.NewUDFRegistry(catalog)

err := udfRegistry.RegisterScalarUDF(
    "surge_multiplier",                    // name
    []sql.DataType{sql.DataTypeFloat},    // input types
    sql.DataTypeFloat,                     // output type
    func(args ...interface{}) (interface{}, error) {
        basePrice := args[0].(float64)
        return basePrice * 1.5, nil
    },
)
```

**Aggregate UDF**:
```go
err := udfRegistry.RegisterAggregateUDF(
    "median",                      // name
    sql.DataTypeFloat,             // input type
    sql.DataTypeFloat,             // output type
    func(acc interface{}, val interface{}) (interface{}, error) {
        // Aggregate logic
        return acc, nil
    },
)
```

### Calling UDFs

```go
result, err := udfRegistry.CallUDF("upper", "hello")
// result = "HELLO"
```

In SQL:
```sql
SELECT upper(area) as area_upper, surge_multiplier(price) as surge_price
FROM ride_requests
```

## Interactive Query Console

Start an interactive SQL console:

```go
console := sql.NewConsole(executor, catalog)
err := console.Start()
```

### Console Commands

- `\help` - Show help
- `\quit` - Exit console
- `\list` - List streams and tables
- `\describe <name>` - Describe schema
- `\udfs` - List UDFs
- `\queries` - List running continuous queries
- `\stop <query_id>` - Stop a continuous query
- `\explain <query>` - Show execution plan
- `\history` - Show command history
- `\clear` - Clear screen

### Console Example

```
gress> SELECT * FROM ride_requests LIMIT 5

+------------------+------------------+------------------+
| ride_id          | area             | price            |
+------------------+------------------+------------------+
| ride_1           | downtown         | 25.50            |
| ride_2           | airport          | 45.00            |
| ride_3           | suburbs          | 18.75            |
| ride_4           | downtown         | 32.00            |
| ride_5           | university       | 12.50            |
+------------------+------------------+------------------+

5 rows returned in 123ms

gress>
```

## Query Optimization

The optimizer applies several rules to improve query performance:

1. **Predicate Pushdown**: Push filters closer to data sources
2. **Projection Pushdown**: Minimize columns read early
3. **Join Reordering**: Optimize join order
4. **Window Optimization**: Merge adjacent windows
5. **Aggregate Optimization**: Combine multiple aggregates

### Viewing Execution Plans

```go
explanation, err := executor.ExplainQuery(`
    SELECT area, COUNT(*)
    FROM ride_requests
    GROUP BY area
`)
fmt.Println(explanation)
```

Output:
```
Query Plan (Cost: 5.50, Optimized: true)
==============================================
1. Project (Cost: 5.50)
  Columns: [area, count]
  Aggregate (Cost: 5.00)
    GroupBy: [area]
    Aggregates: [COUNT(*) as count]
    Filter (Cost: 2.50)
      Predicate: area IS NOT NULL
      Scan (Cost: 1.00)
        Source: ride_requests
```

## Performance Considerations

### Best Practices

1. **Use LIMIT for one-time queries**: Prevents unbounded result sets
2. **Choose appropriate window sizes**: Balance latency vs. completeness
3. **Index primary keys**: Improves join and table lookup performance
4. **Use WHERE before GROUP BY**: Reduces data volume early
5. **Monitor continuous queries**: Check memory usage and throughput

### Benchmarks

From `sql_test.go`:

```
BenchmarkParser-8         50000    25678 ns/op
BenchmarkUDFCall-8      1000000     1234 ns/op
```

## Example Use Cases

### 1. Real-time Analytics Dashboard

```sql
SELECT
    area,
    COUNT(*) as total_rides,
    AVG(price) as avg_price,
    SUM(price) as revenue
FROM ride_requests
WINDOW TUMBLING (SIZE 1 MINUTE)
GROUP BY area
```

### 2. Fraud Detection

```sql
SELECT
    user_id,
    COUNT(*) as tx_count,
    SUM(amount) as total_amount
FROM transactions
WINDOW SLIDING (SIZE 10 MINUTES, SLIDE 1 MINUTE)
GROUP BY user_id
HAVING COUNT(*) > 50 AND SUM(amount) > 10000
```

### 3. User Session Analysis

```sql
SELECT
    user_id,
    COUNT(*) as event_count,
    MAX(timestamp) - MIN(timestamp) as session_duration
FROM user_events
WINDOW SESSION (GAP 30 MINUTES)
GROUP BY user_id
```

### 4. Stream Enrichment (JOIN)

```sql
SELECT
    o.order_id,
    o.amount,
    c.name,
    c.email
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.status = 'pending'
```

## API Reference

### Core Types

```go
// Query represents a parsed SQL query
type Query struct {
    ID          string
    Type        QueryType
    SelectCols  []string
    FromStream  string
    JoinSpec    *JoinSpec
    WhereClause string
    GroupByKeys []string
    WindowSpec  *WindowSpec
    Aggregates  []AggregateSpec
    Continuous  bool
}

// QueryResult represents query results
type QueryResult struct {
    Columns []string
    Rows    [][]interface{}
    Schema  Schema
    Count   int64
}
```

### Executor Methods

```go
// Execute a SQL query
func (e *Executor) Execute(sql string) (*QueryResult, error)

// Explain query execution plan
func (e *Executor) ExplainQuery(sql string) (string, error)

// Stop a continuous query
func (e *Executor) StopContinuousQuery(queryID string) error

// List all continuous queries
func (e *Executor) ListContinuousQueries() []*ContinuousQuery

// Get specific continuous query
func (e *Executor) GetContinuousQuery(queryID string) (*ContinuousQuery, error)
```

### Catalog Methods

```go
// Register a stream
func (c *Catalog) RegisterStream(name string, schema Schema, eventChannel chan *stream.Event) error

// Register a table
func (c *Catalog) RegisterTable(name string, schema Schema, primaryKey []string) error

// Register a UDF
func (c *Catalog) RegisterUDF(name string, inputTypes []DataType, outputType DataType, function interface{}) error

// Get stream/table schema
func (c *Catalog) GetSchema(name string) (Schema, error)
```

## Testing

Run SQL layer tests:

```bash
cd pkg/sql
go test -v
go test -bench=. -benchmem
```

## Examples

See the complete example in `examples/sql-query-demo/`:

```bash
cd examples/sql-query-demo
go run main.go
```

## Limitations and Future Enhancements

### Current Limitations

- No support for nested subqueries
- Limited expression evaluation in predicates
- No support for UNION/INTERSECT/EXCEPT
- Window functions (ROW_NUMBER, RANK) not yet implemented

### Planned Enhancements

- [ ] Subquery support
- [ ] Window functions (ROW_NUMBER, RANK, DENSE_RANK)
- [ ] Advanced join strategies (broadcast joins)
- [ ] Query result caching
- [ ] Cost-based optimization
- [ ] Partitioned tables
- [ ] Late data handling (watermarks)
- [ ] Query profiling and statistics

## References

- [Apache Flink SQL](https://nightlies.apache.org/flink/flink-docs-stable/docs/dev/table/sql/overview/)
- [ksqlDB](https://docs.ksqldb.io/en/latest/)
- [Materialize SQL](https://materialize.com/docs/sql/)
- [SQL Stream Processing](https://arxiv.org/abs/1601.06213)

## Contributing

Contributions are welcome! Please see the main [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](../LICENSE) file.
