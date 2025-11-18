package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/sql"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

func main() {
	fmt.Println("=== Gress SQL Query Demo ===")
	fmt.Println()

	// Create stream engine
	engine := stream.NewEngine(stream.EngineConfig{
		BufferSize:     10000,
		MaxConcurrency: 100,
	})

	// Create SQL catalog
	catalog := sql.NewCatalog()

	// Register ride_requests stream
	rideRequestsChannel := make(chan *stream.Event, 1000)
	rideSchema := sql.Schema{
		Columns: []sql.Column{
			{Name: "ride_id", Type: sql.DataTypeString},
			{Name: "area", Type: sql.DataTypeString},
			{Name: "price", Type: sql.DataTypeFloat},
			{Name: "distance", Type: sql.DataTypeFloat},
			{Name: "timestamp", Type: sql.DataTypeTimestamp},
		},
	}
	err := catalog.RegisterStream("ride_requests", rideSchema, rideRequestsChannel)
	if err != nil {
		log.Fatalf("Failed to register stream: %v", err)
	}

	// Start generating ride request events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go generateRideRequests(ctx, rideRequestsChannel)

	// Create SQL executor
	executor := sql.NewExecutor(catalog, engine)

	// Demo 1: Simple SELECT with LIMIT
	fmt.Println("Demo 1: Simple SELECT query")
	fmt.Println("SQL: SELECT * FROM ride_requests LIMIT 5")
	fmt.Println()
	result, err := executor.Execute("SELECT ride_id, area, price FROM ride_requests LIMIT 5")
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		printResult(result)
	}

	// Demo 2: SELECT with aggregation and GROUP BY
	fmt.Println("\nDemo 2: Aggregation query")
	fmt.Println("SQL: SELECT area, COUNT(*) as ride_count, AVG(price) as avg_price")
	fmt.Println("     FROM ride_requests")
	fmt.Println("     WINDOW TUMBLING (SIZE 5 MINUTES)")
	fmt.Println("     GROUP BY area")
	fmt.Println()

	// Start continuous query
	query := `SELECT area, COUNT(*) as ride_count, AVG(price) as avg_price
		FROM ride_requests
		WINDOW TUMBLING (SIZE 5 MINUTES)
		GROUP BY area`

	result, err = executor.Execute(query)
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		fmt.Printf("Continuous query started: %v\n", result)

		// Subscribe to results
		if len(result.Rows) > 0 {
			queryID := result.Rows[0][0].(string)
			go subscribeToQueryResults(executor, queryID)
		}
	}

	// Demo 3: EXPLAIN query
	fmt.Println("\nDemo 3: Query execution plan")
	fmt.Println("SQL: EXPLAIN SELECT area, COUNT(*) FROM ride_requests GROUP BY area")
	fmt.Println()

	explanation, err := executor.ExplainQuery("SELECT area, COUNT(*) as count FROM ride_requests GROUP BY area")
	if err != nil {
		log.Printf("Explain error: %v", err)
	} else {
		fmt.Println(explanation)
	}

	// Demo 4: UDF usage
	fmt.Println("\nDemo 4: User Defined Functions")
	fmt.Println()

	udfRegistry := sql.NewUDFRegistry(catalog)
	result1, _ := udfRegistry.CallUDF("upper", "downtown")
	fmt.Printf("upper('downtown') = %v\n", result1)

	result2, _ := udfRegistry.CallUDF("concat", "Hello", " ", "World")
	fmt.Printf("concat('Hello', ' ', 'World') = %v\n", result2)

	// Register custom UDF
	err = udfRegistry.RegisterScalarUDF("surge_price",
		[]sql.DataType{sql.DataTypeFloat},
		sql.DataTypeFloat,
		func(args ...interface{}) (interface{}, error) {
			basePrice := args[0].(float64)
			return basePrice * 1.5, nil // 50% surge
		})
	if err != nil {
		log.Printf("UDF registration error: %v", err)
	} else {
		result3, _ := udfRegistry.CallUDF("surge_price", 20.0)
		fmt.Printf("surge_price(20.0) = %v\n", result3)
	}

	// Demo 5: Stream-Table Duality
	fmt.Println("\n\nDemo 5: Stream-Table Duality")
	fmt.Println()

	streamTable := sql.NewStreamTableDuality(catalog, nil)

	// Materialize stream into table
	err = streamTable.MaterializeStream("ride_requests", "ride_requests_table", []string{"ride_id"})
	if err != nil {
		log.Printf("Materialization error: %v", err)
	} else {
		fmt.Println("Stream materialized into table: ride_requests_table")
	}

	// Wait a bit for some data to accumulate
	time.Sleep(2 * time.Second)

	// Query table
	size, _ := streamTable.GetTableSize("ride_requests_table")
	fmt.Printf("Table size: %d rows\n", size)

	// Demo 6: Interactive Console (would be run separately)
	fmt.Println("\n\nDemo 6: Interactive Console")
	fmt.Println("To start the interactive console, run:")
	fmt.Println("  console := sql.NewConsole(executor, catalog)")
	fmt.Println("  console.Start()")
	fmt.Println()

	// Demo 7: List all available features
	fmt.Println("\nDemo 7: Available Streams and UDFs")
	fmt.Println()

	fmt.Println("Streams:")
	for _, name := range catalog.ListStreams() {
		fmt.Printf("  - %s\n", name)
	}

	fmt.Println("\nTables:")
	for _, name := range catalog.ListTables() {
		fmt.Printf("  - %s\n", name)
	}

	fmt.Println("\nUDFs:")
	for _, name := range udfRegistry.ListUDFs() {
		fmt.Printf("  - %s\n", name)
	}

	// Keep running for a bit to see continuous query results
	fmt.Println("\n\nWaiting for continuous query results...")
	time.Sleep(10 * time.Second)

	fmt.Println("\nDemo completed!")
}

// generateRideRequests generates synthetic ride request events
func generateRideRequests(ctx context.Context, channel chan *stream.Event) {
	areas := []string{"downtown", "airport", "suburbs", "university", "mall"}
	rand.Seed(time.Now().UnixNano())

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	rideID := 1
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			area := areas[rand.Intn(len(areas))]
			price := 10.0 + rand.Float64()*50.0
			distance := 1.0 + rand.Float64()*20.0

			event := &stream.Event{
				Key: fmt.Sprintf("ride_%d", rideID),
				Value: map[string]interface{}{
					"ride_id":   fmt.Sprintf("ride_%d", rideID),
					"area":      area,
					"price":     price,
					"distance":  distance,
					"timestamp": time.Now(),
				},
				EventTime: time.Now(),
				Headers: map[string]string{
					"source": "ride-service",
				},
			}

			select {
			case channel <- event:
				rideID++
			default:
				// Channel full, skip
			}
		}
	}
}

// subscribeToQueryResults subscribes to continuous query results
func subscribeToQueryResults(executor *sql.Executor, queryID string) {
	cq, err := executor.GetContinuousQuery(queryID)
	if err != nil {
		log.Printf("Failed to get continuous query: %v", err)
		return
	}

	fmt.Printf("\nSubscribed to continuous query: %s\n", queryID)
	fmt.Println("Results will be printed as they arrive...")
	fmt.Println()

	for event := range cq.OutputChannel {
		if valueMap, ok := event.Value.(map[string]interface{}); ok {
			fmt.Printf("[%s] Result: %v\n", event.EventTime.Format("15:04:05"), valueMap)
		}
	}
}

// printResult prints query results in a formatted table
func printResult(result *sql.QueryResult) {
	if result == nil || len(result.Rows) == 0 {
		fmt.Println("No results")
		return
	}

	// Print header
	fmt.Print("|")
	for _, col := range result.Columns {
		fmt.Printf(" %-15s |", col)
	}
	fmt.Println()

	// Print separator
	fmt.Print("|")
	for range result.Columns {
		fmt.Print("-----------------|")
	}
	fmt.Println()

	// Print rows
	for _, row := range result.Rows {
		fmt.Print("|")
		for _, cell := range row {
			fmt.Printf(" %-15v |", cell)
		}
		fmt.Println()
	}

	// Print footer
	fmt.Print("|")
	for range result.Columns {
		fmt.Print("-----------------|")
	}
	fmt.Println()

	fmt.Printf("\n%d rows returned\n", result.Count)
}
