package sql

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// Console provides an interactive SQL query console
type Console struct {
	executor   *Executor
	catalog    *Catalog
	udfRegistry *UDFRegistry
	history    []string
	reader     *bufio.Reader
	writer     io.Writer
}

// NewConsole creates a new interactive console
func NewConsole(executor *Executor, catalog *Catalog) *Console {
	return &Console{
		executor:    executor,
		catalog:     catalog,
		udfRegistry: NewUDFRegistry(catalog),
		history:     []string{},
		reader:      bufio.NewReader(os.Stdin),
		writer:      os.Stdout,
	}
}

// Start starts the interactive console
func (c *Console) Start() error {
	c.printWelcome()

	for {
		// Print prompt
		fmt.Fprint(c.writer, "gress> ")

		// Read input
		input, err := c.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(c.writer, "\nExiting...")
				return nil
			}
			return fmt.Errorf("error reading input: %w", err)
		}

		// Trim whitespace
		input = strings.TrimSpace(input)

		// Skip empty input
		if input == "" {
			continue
		}

		// Add to history
		c.history = append(c.history, input)

		// Handle special commands
		if strings.HasPrefix(input, "\\") {
			c.handleCommand(input)
			continue
		}

		// Execute SQL query
		c.executeQuery(input)
	}
}

// printWelcome prints the welcome message
func (c *Console) printWelcome() {
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "╔════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(c.writer, "║          Gress SQL Console - Interactive Query Tool        ║")
	fmt.Fprintln(c.writer, "║              Stream Processing with SQL Syntax             ║")
	fmt.Fprintln(c.writer, "╚════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Type \\help for help, \\quit to exit")
	fmt.Fprintln(c.writer, "")
}

// handleCommand handles special console commands
func (c *Console) handleCommand(command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "\\help", "\\h", "\\?":
		c.printHelp()

	case "\\quit", "\\q", "\\exit":
		fmt.Fprintln(c.writer, "Exiting...")
		os.Exit(0)

	case "\\list", "\\l":
		c.listStreamsAndTables()

	case "\\describe", "\\d":
		if len(parts) < 2 {
			fmt.Fprintln(c.writer, "Usage: \\describe <stream_or_table>")
			return
		}
		c.describeStreamOrTable(parts[1])

	case "\\udfs":
		c.listUDFs()

	case "\\queries", "\\cq":
		c.listContinuousQueries()

	case "\\stop":
		if len(parts) < 2 {
			fmt.Fprintln(c.writer, "Usage: \\stop <query_id>")
			return
		}
		c.stopContinuousQuery(parts[1])

	case "\\explain":
		if len(parts) < 2 {
			fmt.Fprintln(c.writer, "Usage: \\explain <query>")
			return
		}
		query := strings.Join(parts[1:], " ")
		c.explainQuery(query)

	case "\\history":
		c.printHistory()

	case "\\clear":
		// Clear screen (Unix-like systems)
		fmt.Fprint(c.writer, "\033[H\033[2J")
		c.printWelcome()

	default:
		fmt.Fprintf(c.writer, "Unknown command: %s\n", cmd)
		fmt.Fprintln(c.writer, "Type \\help for available commands")
	}
}

// printHelp prints help information
func (c *Console) printHelp() {
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Available Commands:")
	fmt.Fprintln(c.writer, "==================")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Console Commands:")
	fmt.Fprintln(c.writer, "  \\help, \\h, \\?          - Show this help message")
	fmt.Fprintln(c.writer, "  \\quit, \\q, \\exit       - Exit the console")
	fmt.Fprintln(c.writer, "  \\clear                 - Clear the screen")
	fmt.Fprintln(c.writer, "  \\history               - Show command history")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Catalog Commands:")
	fmt.Fprintln(c.writer, "  \\list, \\l             - List all streams and tables")
	fmt.Fprintln(c.writer, "  \\describe, \\d <name>  - Describe a stream or table")
	fmt.Fprintln(c.writer, "  \\udfs                  - List all registered UDFs")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Query Commands:")
	fmt.Fprintln(c.writer, "  \\queries, \\cq          - List running continuous queries")
	fmt.Fprintln(c.writer, "  \\stop <query_id>       - Stop a continuous query")
	fmt.Fprintln(c.writer, "  \\explain <query>       - Show execution plan for a query")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "SQL Syntax:")
	fmt.Fprintln(c.writer, "  SELECT columns FROM stream [WHERE condition]")
	fmt.Fprintln(c.writer, "    [WINDOW TUMBLING (SIZE <duration>)]")
	fmt.Fprintln(c.writer, "    [GROUP BY columns]")
	fmt.Fprintln(c.writer, "    [HAVING condition]")
	fmt.Fprintln(c.writer, "    [ORDER BY columns]")
	fmt.Fprintln(c.writer, "    [LIMIT n]")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Aggregate Functions:")
	fmt.Fprintln(c.writer, "  COUNT(*), COUNT(column), SUM(column), AVG(column)")
	fmt.Fprintln(c.writer, "  MIN(column), MAX(column), STDDEV(column)")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Window Types:")
	fmt.Fprintln(c.writer, "  TUMBLING (SIZE <duration>)")
	fmt.Fprintln(c.writer, "  SLIDING (SIZE <duration>, SLIDE <duration>)")
	fmt.Fprintln(c.writer, "  SESSION (GAP <duration>)")
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Example Queries:")
	fmt.Fprintln(c.writer, "  SELECT * FROM ride_requests LIMIT 10")
	fmt.Fprintln(c.writer, "  SELECT area, COUNT(*) as count FROM ride_requests")
	fmt.Fprintln(c.writer, "    WINDOW TUMBLING (SIZE 5 MINUTES)")
	fmt.Fprintln(c.writer, "    GROUP BY area")
	fmt.Fprintln(c.writer, "  SELECT user_id, AVG(price) FROM orders")
	fmt.Fprintln(c.writer, "    WHERE status = 'completed'")
	fmt.Fprintln(c.writer, "    GROUP BY user_id")
	fmt.Fprintln(c.writer, "    HAVING AVG(price) > 100")
	fmt.Fprintln(c.writer, "")
}

// executeQuery executes a SQL query
func (c *Console) executeQuery(sql string) {
	start := time.Now()

	// Execute query
	result, err := c.executor.Execute(sql)
	if err != nil {
		fmt.Fprintf(c.writer, "Error: %v\n", err)
		return
	}

	elapsed := time.Since(start)

	// Display results
	c.displayResult(result, elapsed)
}

// displayResult displays query results in a table format
func (c *Console) displayResult(result *QueryResult, elapsed time.Duration) {
	if result == nil || len(result.Rows) == 0 {
		fmt.Fprintln(c.writer, "No results")
		return
	}

	// Calculate column widths
	colWidths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		colWidths[i] = len(col)
	}

	for _, row := range result.Rows {
		for i, cell := range row {
			cellStr := fmt.Sprintf("%v", cell)
			if len(cellStr) > colWidths[i] {
				colWidths[i] = len(cellStr)
			}
		}
	}

	// Print header
	fmt.Fprintln(c.writer, "")
	c.printSeparator(colWidths)
	c.printRow(result.Columns, colWidths)
	c.printSeparator(colWidths)

	// Print rows
	for _, row := range result.Rows {
		rowStrs := make([]string, len(row))
		for i, cell := range row {
			rowStrs[i] = fmt.Sprintf("%v", cell)
		}
		c.printRow(rowStrs, colWidths)
	}

	c.printSeparator(colWidths)
	fmt.Fprintln(c.writer, "")
	fmt.Fprintf(c.writer, "%d rows returned in %v\n", result.Count, elapsed)
	fmt.Fprintln(c.writer, "")
}

// printSeparator prints a table separator
func (c *Console) printSeparator(colWidths []int) {
	fmt.Fprint(c.writer, "+")
	for _, width := range colWidths {
		fmt.Fprint(c.writer, strings.Repeat("-", width+2))
		fmt.Fprint(c.writer, "+")
	}
	fmt.Fprintln(c.writer, "")
}

// printRow prints a table row
func (c *Console) printRow(cells []string, colWidths []int) {
	fmt.Fprint(c.writer, "|")
	for i, cell := range cells {
		fmt.Fprintf(c.writer, " %-*s |", colWidths[i], cell)
	}
	fmt.Fprintln(c.writer, "")
}

// listStreamsAndTables lists all streams and tables
func (c *Console) listStreamsAndTables() {
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Streams:")
	fmt.Fprintln(c.writer, "--------")
	streams := c.catalog.ListStreams()
	if len(streams) == 0 {
		fmt.Fprintln(c.writer, "(none)")
	} else {
		for _, name := range streams {
			fmt.Fprintf(c.writer, "  - %s\n", name)
		}
	}

	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Tables:")
	fmt.Fprintln(c.writer, "-------")
	tables := c.catalog.ListTables()
	if len(tables) == 0 {
		fmt.Fprintln(c.writer, "(none)")
	} else {
		for _, name := range tables {
			fmt.Fprintf(c.writer, "  - %s\n", name)
		}
	}
	fmt.Fprintln(c.writer, "")
}

// describeStreamOrTable describes a stream or table schema
func (c *Console) describeStreamOrTable(name string) {
	schema, err := c.catalog.GetSchema(name)
	if err != nil {
		fmt.Fprintf(c.writer, "Error: %v\n", err)
		return
	}

	fmt.Fprintln(c.writer, "")
	fmt.Fprintf(c.writer, "Schema for: %s\n", name)
	fmt.Fprintln(c.writer, strings.Repeat("=", 50))
	fmt.Fprintln(c.writer, "")

	// Print columns
	colWidths := []int{20, 15, 10}
	c.printSeparator(colWidths)
	c.printRow([]string{"Column", "Type", "Nullable"}, colWidths)
	c.printSeparator(colWidths)

	for _, col := range schema.Columns {
		nullable := "NO"
		if col.Nullable {
			nullable = "YES"
		}
		c.printRow([]string{col.Name, dataTypeToString(col.Type), nullable}, colWidths)
	}

	c.printSeparator(colWidths)
	fmt.Fprintln(c.writer, "")
}

// listUDFs lists all registered UDFs
func (c *Console) listUDFs() {
	udfs := c.udfRegistry.ListUDFs()

	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Registered UDFs:")
	fmt.Fprintln(c.writer, "----------------")

	if len(udfs) == 0 {
		fmt.Fprintln(c.writer, "(none)")
	} else {
		for _, name := range udfs {
			udf, err := c.udfRegistry.GetUDFInfo(name)
			if err != nil {
				continue
			}

			inputTypes := make([]string, len(udf.InputTypes))
			for i, t := range udf.InputTypes {
				inputTypes[i] = dataTypeToString(t)
			}

			fmt.Fprintf(c.writer, "  - %s(%s) -> %s\n",
				name,
				strings.Join(inputTypes, ", "),
				dataTypeToString(udf.OutputType))
		}
	}
	fmt.Fprintln(c.writer, "")
}

// listContinuousQueries lists all running continuous queries
func (c *Console) listContinuousQueries() {
	queries := c.executor.ListContinuousQueries()

	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Running Continuous Queries:")
	fmt.Fprintln(c.writer, "---------------------------")

	if len(queries) == 0 {
		fmt.Fprintln(c.writer, "(none)")
	} else {
		for _, cq := range queries {
			fmt.Fprintf(c.writer, "  ID: %s\n", cq.ID)
			fmt.Fprintf(c.writer, "  Started: %s\n", cq.Started.Format(time.RFC3339))
			fmt.Fprintf(c.writer, "  Events Processed: %d\n", cq.EventsProcessed)
			fmt.Fprintf(c.writer, "  Last Result: %s\n", cq.LastResult.Format(time.RFC3339))
			fmt.Fprintln(c.writer, "")
		}
	}
	fmt.Fprintln(c.writer, "")
}

// stopContinuousQuery stops a continuous query
func (c *Console) stopContinuousQuery(queryID string) {
	err := c.executor.StopContinuousQuery(queryID)
	if err != nil {
		fmt.Fprintf(c.writer, "Error: %v\n", err)
		return
	}

	fmt.Fprintf(c.writer, "Stopped continuous query: %s\n", queryID)
}

// explainQuery shows the execution plan for a query
func (c *Console) explainQuery(sql string) {
	explanation, err := c.executor.ExplainQuery(sql)
	if err != nil {
		fmt.Fprintf(c.writer, "Error: %v\n", err)
		return
	}

	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, explanation)
}

// printHistory prints command history
func (c *Console) printHistory() {
	fmt.Fprintln(c.writer, "")
	fmt.Fprintln(c.writer, "Command History:")
	fmt.Fprintln(c.writer, "----------------")

	if len(c.history) == 0 {
		fmt.Fprintln(c.writer, "(empty)")
	} else {
		for i, cmd := range c.history {
			fmt.Fprintf(c.writer, "%3d: %s\n", i+1, cmd)
		}
	}
	fmt.Fprintln(c.writer, "")
}

// dataTypeToString converts a DataType to string representation
func dataTypeToString(dt DataType) string {
	switch dt {
	case DataTypeString:
		return "STRING"
	case DataTypeInt:
		return "INT"
	case DataTypeFloat:
		return "FLOAT"
	case DataTypeBool:
		return "BOOL"
	case DataTypeTimestamp:
		return "TIMESTAMP"
	case DataTypeBytes:
		return "BYTES"
	default:
		return "UNKNOWN"
	}
}

// SubscribeToQuery subscribes to results from a continuous query
func (c *Console) SubscribeToQuery(queryID string, handler func(*stream.Event)) error {
	cq, err := c.executor.GetContinuousQuery(queryID)
	if err != nil {
		return err
	}

	// Start goroutine to read from output channel
	go func() {
		for event := range cq.OutputChannel {
			handler(event)
		}
	}()

	return nil
}
