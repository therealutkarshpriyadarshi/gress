package sql

import (
	"testing"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

func TestParser(t *testing.T) {
	catalog := NewCatalog()

	// Register test stream
	testSchema := Schema{
		Columns: []Column{
			{Name: "area", Type: DataTypeString},
			{Name: "price", Type: DataTypeFloat},
			{Name: "ride_id", Type: DataTypeString},
		},
	}
	eventChannel := make(chan *stream.Event, 10)
	err := catalog.RegisterStream("ride_requests", testSchema, eventChannel)
	if err != nil {
		t.Fatalf("Failed to register stream: %v", err)
	}

	parser := NewParser(catalog)

	tests := []struct {
		name     string
		sql      string
		wantErr  bool
		validate func(*testing.T, *Query)
	}{
		{
			name: "Simple SELECT",
			sql:  "SELECT area, price FROM ride_requests LIMIT 10",
			wantErr: false,
			validate: func(t *testing.T, q *Query) {
				if q.Type != QueryTypeSelect {
					t.Errorf("Expected SELECT query type")
				}
				if len(q.SelectCols) != 2 {
					t.Errorf("Expected 2 columns, got %d", len(q.SelectCols))
				}
				if q.FromStream != "ride_requests" {
					t.Errorf("Expected from 'ride_requests', got '%s'", q.FromStream)
				}
				if q.Limit != 10 {
					t.Errorf("Expected LIMIT 10, got %d", q.Limit)
				}
			},
		},
		{
			name: "SELECT with COUNT aggregate",
			sql:  "SELECT area, COUNT(*) as ride_count FROM ride_requests GROUP BY area",
			wantErr: false,
			validate: func(t *testing.T, q *Query) {
				if len(q.Aggregates) != 1 {
					t.Errorf("Expected 1 aggregate, got %d", len(q.Aggregates))
				}
				if q.Aggregates[0].Function != "COUNT" {
					t.Errorf("Expected COUNT aggregate, got %s", q.Aggregates[0].Function)
				}
				if len(q.GroupByKeys) != 1 {
					t.Errorf("Expected 1 GROUP BY key, got %d", len(q.GroupByKeys))
				}
			},
		},
		{
			name: "SELECT with window",
			sql:  "SELECT area, COUNT(*) as ride_count FROM ride_requests WINDOW TUMBLING (SIZE 5 MINUTES) GROUP BY area",
			wantErr: false,
			validate: func(t *testing.T, q *Query) {
				if q.WindowSpec == nil {
					t.Error("Expected window spec")
				}
				if q.WindowSpec.Type != WindowTypeTumbling {
					t.Errorf("Expected TUMBLING window, got %v", q.WindowSpec.Type)
				}
				if q.WindowSpec.Size != 5*time.Minute {
					t.Errorf("Expected 5 minute window, got %v", q.WindowSpec.Size)
				}
			},
		},
		{
			name: "SELECT with HAVING",
			sql:  "SELECT area, COUNT(*) as ride_count FROM ride_requests WINDOW TUMBLING (SIZE 5 MINUTES) GROUP BY area HAVING COUNT(*) > 100",
			wantErr: false,
			validate: func(t *testing.T, q *Query) {
				if q.HavingClause == "" {
					t.Error("Expected HAVING clause")
				}
			},
		},
		{
			name: "SELECT with AVG",
			sql:  "SELECT area, AVG(price) as avg_price FROM ride_requests GROUP BY area",
			wantErr: false,
			validate: func(t *testing.T, q *Query) {
				if len(q.Aggregates) != 1 {
					t.Errorf("Expected 1 aggregate, got %d", len(q.Aggregates))
				}
				if q.Aggregates[0].Function != "AVG" {
					t.Errorf("Expected AVG aggregate, got %s", q.Aggregates[0].Function)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := parser.Parse(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, query)
			}
		})
	}
}

func TestCatalog(t *testing.T) {
	catalog := NewCatalog()

	// Test stream registration
	schema := Schema{
		Columns: []Column{
			{Name: "id", Type: DataTypeString},
			{Name: "value", Type: DataTypeInt},
		},
	}
	eventChannel := make(chan *stream.Event, 10)

	err := catalog.RegisterStream("test_stream", schema, eventChannel)
	if err != nil {
		t.Fatalf("Failed to register stream: %v", err)
	}

	// Test stream retrieval
	stream, err := catalog.GetStream("test_stream")
	if err != nil {
		t.Fatalf("Failed to get stream: %v", err)
	}
	if stream.Name != "test_stream" {
		t.Errorf("Expected stream name 'test_stream', got '%s'", stream.Name)
	}

	// Test stream exists
	if !catalog.StreamExists("test_stream") {
		t.Error("Stream should exist")
	}

	// Test duplicate registration
	err = catalog.RegisterStream("test_stream", schema, eventChannel)
	if err == nil {
		t.Error("Expected error for duplicate stream registration")
	}

	// Test table registration
	err = catalog.RegisterTable("test_table", schema, []string{"id"})
	if err != nil {
		t.Fatalf("Failed to register table: %v", err)
	}

	// Test table retrieval
	table, err := catalog.GetTable("test_table")
	if err != nil {
		t.Fatalf("Failed to get table: %v", err)
	}
	if table.Name != "test_table" {
		t.Errorf("Expected table name 'test_table', got '%s'", table.Name)
	}

	// Test list operations
	streams := catalog.ListStreams()
	if len(streams) != 1 {
		t.Errorf("Expected 1 stream, got %d", len(streams))
	}

	tables := catalog.ListTables()
	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}
}

func TestUDFRegistry(t *testing.T) {
	catalog := NewCatalog()
	registry := NewUDFRegistry(catalog)

	// Test built-in UDF
	result, err := registry.CallUDF("upper", "hello")
	if err != nil {
		t.Fatalf("Failed to call UDF: %v", err)
	}
	if result != "HELLO" {
		t.Errorf("Expected 'HELLO', got '%v'", result)
	}

	// Test custom UDF registration
	err = registry.RegisterScalarUDF("double", []DataType{DataTypeInt}, DataTypeInt,
		func(args ...interface{}) (interface{}, error) {
			val := args[0].(int)
			return val * 2, nil
		})
	if err != nil {
		t.Fatalf("Failed to register UDF: %v", err)
	}

	// Test custom UDF call
	result, err = registry.CallUDF("double", 5)
	if err != nil {
		t.Fatalf("Failed to call custom UDF: %v", err)
	}
	if result != 10 {
		t.Errorf("Expected 10, got %v", result)
	}

	// Test UDF list
	udfs := registry.ListUDFs()
	if len(udfs) == 0 {
		t.Error("Expected UDFs to be registered")
	}
}

func TestPlanner(t *testing.T) {
	catalog := NewCatalog()
	planner := NewPlanner(catalog)

	// Register test stream
	schema := Schema{
		Columns: []Column{
			{Name: "area", Type: DataTypeString},
			{Name: "price", Type: DataTypeFloat},
		},
	}
	eventChannel := make(chan *stream.Event, 10)
	err := catalog.RegisterStream("ride_requests", schema, eventChannel)
	if err != nil {
		t.Fatalf("Failed to register stream: %v", err)
	}

	// Test query planning
	query := &Query{
		Type:       QueryTypeSelect,
		SelectCols: []string{"area", "ride_count"},
		FromStream: "ride_requests",
		GroupByKeys: []string{"area"},
		Aggregates: []AggregateSpec{
			{Function: "COUNT", Column: "*", Alias: "ride_count"},
		},
		WindowSpec: &WindowSpec{
			Type: WindowTypeTumbling,
			Size: 5 * time.Minute,
		},
	}

	plan, err := planner.Plan(query)
	if err != nil {
		t.Fatalf("Failed to plan query: %v", err)
	}

	if plan.Query != query {
		t.Error("Plan should reference original query")
	}

	if len(plan.Operators) == 0 {
		t.Error("Plan should have operators")
	}

	if plan.Cost <= 0 {
		t.Error("Plan should have positive cost")
	}
}

func TestOptimizer(t *testing.T) {
	catalog := NewCatalog()
	planner := NewPlanner(catalog)
	optimizer := NewOptimizer(catalog)

	// Register test stream
	schema := Schema{
		Columns: []Column{
			{Name: "area", Type: DataTypeString},
			{Name: "price", Type: DataTypeFloat},
		},
	}
	eventChannel := make(chan *stream.Event, 10)
	err := catalog.RegisterStream("ride_requests", schema, eventChannel)
	if err != nil {
		t.Fatalf("Failed to register stream: %v", err)
	}

	// Create plan
	query := &Query{
		Type:       QueryTypeSelect,
		SelectCols: []string{"area"},
		FromStream: "ride_requests",
		WhereClause: "price > 100",
	}

	plan, err := planner.Plan(query)
	if err != nil {
		t.Fatalf("Failed to plan query: %v", err)
	}

	// Optimize plan
	optimizedPlan, err := optimizer.Optimize(plan)
	if err != nil {
		t.Fatalf("Failed to optimize plan: %v", err)
	}

	if !optimizedPlan.Optimized {
		t.Error("Plan should be marked as optimized")
	}

	// Test plan explanation
	explanation := optimizer.ExplainPlan(optimizedPlan)
	if explanation == "" {
		t.Error("Explanation should not be empty")
	}
}

func TestStreamTableDuality(t *testing.T) {
	catalog := NewCatalog()
	std := NewStreamTableDuality(catalog, nil)

	// Register test stream
	schema := Schema{
		Columns: []Column{
			{Name: "id", Type: DataTypeString},
			{Name: "value", Type: DataTypeInt},
		},
	}
	eventChannel := make(chan *stream.Event, 10)
	err := catalog.RegisterStream("test_stream", schema, eventChannel)
	if err != nil {
		t.Fatalf("Failed to register stream: %v", err)
	}

	// Test stream materialization
	err = std.MaterializeStream("test_stream", "test_table", []string{"id"})
	if err != nil {
		t.Fatalf("Failed to materialize stream: %v", err)
	}

	// Verify table was created
	if !catalog.TableExists("test_table") {
		t.Error("Table should have been created")
	}

	// Test table snapshot
	snapshot, err := std.GetTableSnapshot("test_table")
	if err != nil {
		t.Fatalf("Failed to get table snapshot: %v", err)
	}
	if snapshot == nil {
		t.Error("Snapshot should not be nil")
	}

	// Test table size
	size, err := std.GetTableSize("test_table")
	if err != nil {
		t.Fatalf("Failed to get table size: %v", err)
	}
	if size != 0 {
		t.Errorf("Expected size 0, got %d", size)
	}
}

func TestTemporalTable(t *testing.T) {
	schema := Schema{
		Columns: []Column{
			{Name: "id", Type: DataTypeString},
			{Name: "value", Type: DataTypeInt},
		},
	}

	tt := NewTemporalTable("test_temporal", schema)

	// Insert versions
	now := time.Now()
	tt.Insert("key1", 100, now)
	time.Sleep(10 * time.Millisecond)
	tt.Insert("key1", 200, now.Add(1*time.Second))

	// Query as of first insert
	value, err := tt.QueryAsOf("key1", now)
	if err != nil {
		t.Fatalf("Failed to query as of: %v", err)
	}
	if value != 100 {
		t.Errorf("Expected value 100, got %v", value)
	}

	// Query as of second insert
	value, err = tt.QueryAsOf("key1", now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("Failed to query as of: %v", err)
	}
	if value != 200 {
		t.Errorf("Expected value 200, got %v", value)
	}

	// Query history
	history, err := tt.QueryHistory("key1")
	if err != nil {
		t.Fatalf("Failed to query history: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(history))
	}
}

func BenchmarkParser(b *testing.B) {
	catalog := NewCatalog()
	schema := Schema{
		Columns: []Column{
			{Name: "area", Type: DataTypeString},
			{Name: "price", Type: DataTypeFloat},
		},
	}
	eventChannel := make(chan *stream.Event, 10)
	catalog.RegisterStream("ride_requests", schema, eventChannel)

	parser := NewParser(catalog)
	sql := "SELECT area, COUNT(*) as ride_count, AVG(price) as avg_price FROM ride_requests WINDOW TUMBLING (SIZE 5 MINUTES) GROUP BY area HAVING COUNT(*) > 100"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.Parse(sql)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

func BenchmarkUDFCall(b *testing.B) {
	catalog := NewCatalog()
	registry := NewUDFRegistry(catalog)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := registry.CallUDF("upper", "hello")
		if err != nil {
			b.Fatalf("UDF call error: %v", err)
		}
	}
}
