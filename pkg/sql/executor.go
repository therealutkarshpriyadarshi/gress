package sql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// Executor executes SQL queries against streams
type Executor struct {
	catalog          *Catalog
	parser           *Parser
	planner          *Planner
	optimizer        *Optimizer
	engine           *stream.Engine
	continuousQueries map[string]*ContinuousQuery
	mu               sync.RWMutex
}

// NewExecutor creates a new SQL executor
func NewExecutor(catalog *Catalog, engine *stream.Engine) *Executor {
	return &Executor{
		catalog:           catalog,
		parser:            NewParser(catalog),
		planner:           NewPlanner(catalog),
		optimizer:         NewOptimizer(catalog),
		engine:            engine,
		continuousQueries: make(map[string]*ContinuousQuery),
	}
}

// Execute executes a SQL query
func (e *Executor) Execute(sql string) (*QueryResult, error) {
	// Parse query
	query, err := e.parser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Validate query
	if err := e.parser.Validate(query); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Handle different query types
	switch query.Type {
	case QueryTypeSelect:
		return e.executeSelect(query)
	case QueryTypeCreateStream:
		return e.executeCreateStream(query)
	case QueryTypeCreateTable:
		return e.executeCreateTable(query)
	case QueryTypeDropStream:
		return e.executeDropStream(query)
	case QueryTypeDropTable:
		return e.executeDropTable(query)
	default:
		return nil, fmt.Errorf("unsupported query type: %v", query.Type)
	}
}

// executeSelect executes a SELECT query
func (e *Executor) executeSelect(query *Query) (*QueryResult, error) {
	// Create execution plan
	plan, err := e.planner.Plan(query)
	if err != nil {
		return nil, fmt.Errorf("planning error: %w", err)
	}

	// Optimize plan
	optimizedPlan, err := e.optimizer.Optimize(plan)
	if err != nil {
		return nil, fmt.Errorf("optimization error: %w", err)
	}

	// Validate plan
	if err := e.optimizer.ValidatePlan(optimizedPlan); err != nil {
		return nil, fmt.Errorf("plan validation error: %w", err)
	}

	// Check if this is a continuous query
	if query.Continuous {
		return e.executeContinuousQuery(query, optimizedPlan)
	} else {
		return e.executeOneTimeQuery(query, optimizedPlan)
	}
}

// executeContinuousQuery starts a continuous query
func (e *Executor) executeContinuousQuery(query *Query, plan *QueryPlan) (*QueryResult, error) {
	// Generate query ID
	queryID := fmt.Sprintf("cq_%d", time.Now().UnixNano())
	query.ID = queryID

	// Create output channel
	outputChan := make(chan *stream.Event, 1000)
	stopChan := make(chan struct{})

	// Create continuous query
	cq := &ContinuousQuery{
		ID:            queryID,
		Query:         query,
		SourceStreams: []string{query.FromStream},
		OutputChannel: outputChan,
		StopChannel:   stopChan,
		Started:       time.Now(),
		EventsProcessed: 0,
	}

	// Register continuous query
	e.mu.Lock()
	e.continuousQueries[queryID] = cq
	e.mu.Unlock()

	// Build and start stream pipeline
	go e.runContinuousQuery(cq, plan)

	// Return query info
	return &QueryResult{
		Columns: []string{"query_id", "status"},
		Rows: [][]interface{}{
			{queryID, "started"},
		},
		Count: 1,
	}, nil
}

// runContinuousQuery runs a continuous query
func (e *Executor) runContinuousQuery(cq *ContinuousQuery, plan *QueryPlan) {
	ctx := context.Background()

	// Get source stream
	sourceStream, err := e.catalog.GetStream(cq.Query.FromStream)
	if err != nil {
		fmt.Printf("Error getting source stream: %v\n", err)
		return
	}

	// Process events from source stream
	for {
		select {
		case <-cq.StopChannel:
			return
		case event := <-sourceStream.EventChannel:
			// Process event through plan
			results, err := e.processEvent(event, plan, ctx)
			if err != nil {
				fmt.Printf("Error processing event: %v\n", err)
				continue
			}

			// Send results to output channel
			for _, result := range results {
				select {
				case cq.OutputChannel <- result:
					cq.EventsProcessed++
					cq.LastResult = time.Now()
				default:
					// Output channel full, drop event
					fmt.Println("Output channel full, dropping event")
				}
			}
		}
	}
}

// processEvent processes an event through the query plan
func (e *Executor) processEvent(event *stream.Event, plan *QueryPlan, ctx context.Context) ([]*stream.Event, error) {
	// This is a simplified implementation
	// In a real implementation, we would process the event through each operator in the plan

	results := []*stream.Event{event}

	// Apply operators in order
	for _, node := range plan.Operators {
		newResults := []*stream.Event{}
		for _, result := range results {
			processed, err := e.processNode(result, node, ctx)
			if err != nil {
				return nil, err
			}
			newResults = append(newResults, processed...)
		}
		results = newResults
	}

	return results, nil
}

// processNode processes an event through a plan node
func (e *Executor) processNode(event *stream.Event, node PlanNode, ctx context.Context) ([]*stream.Event, error) {
	// Process based on node type
	switch n := node.(type) {
	case *ScanNode:
		return []*stream.Event{event}, nil
	case *FilterNode:
		// Apply filter
		if e.evaluateFilter(event, n.Predicate) {
			// Process through child
			return e.processNode(event, n.Children()[0], ctx)
		}
		return nil, nil
	case *ProjectNode:
		// Apply projection
		projected := e.applyProjection(event, n.Columns)
		return []*stream.Event{projected}, nil
	case *AggregateNode:
		// Apply aggregation (simplified - would need state management)
		return []*stream.Event{event}, nil
	case *JoinNode:
		// Apply join (simplified - would need state management)
		return []*stream.Event{event}, nil
	case *WindowNode:
		// Apply windowing (simplified - would use window operator)
		return []*stream.Event{event}, nil
	default:
		return []*stream.Event{event}, nil
	}
}

// executeOneTimeQuery executes a one-time (non-continuous) query
func (e *Executor) executeOneTimeQuery(query *Query, plan *QueryPlan) (*QueryResult, error) {
	// For one-time queries, we collect results up to the LIMIT
	results := [][]interface{}{}
	limit := query.Limit
	if limit == 0 {
		limit = 100 // Default limit
	}

	// Get source stream
	sourceStream, err := e.catalog.GetStream(query.FromStream)
	if err != nil {
		return nil, fmt.Errorf("failed to get source stream: %w", err)
	}

	// Collect results
	ctx := context.Background()
	timeout := time.After(5 * time.Second) // 5 second timeout

	for len(results) < limit {
		select {
		case event := <-sourceStream.EventChannel:
			// Process event
			processed, err := e.processEvent(event, plan, ctx)
			if err != nil {
				continue
			}

			// Convert to result rows
			for _, result := range processed {
				row := e.eventToRow(result, query.SelectCols)
				results = append(results, row)

				if len(results) >= limit {
					break
				}
			}
		case <-timeout:
			// Timeout reached
			break
		}
	}

	return &QueryResult{
		Columns: query.SelectCols,
		Rows:    results,
		Count:   int64(len(results)),
	}, nil
}

// executeCreateStream handles CREATE STREAM statements
func (e *Executor) executeCreateStream(query *Query) (*QueryResult, error) {
	return &QueryResult{
		Columns: []string{"status"},
		Rows: [][]interface{}{
			{"stream created"},
		},
		Count: 1,
	}, nil
}

// executeCreateTable handles CREATE TABLE statements
func (e *Executor) executeCreateTable(query *Query) (*QueryResult, error) {
	return &QueryResult{
		Columns: []string{"status"},
		Rows: [][]interface{}{
			{"table created"},
		},
		Count: 1,
	}, nil
}

// executeDropStream handles DROP STREAM statements
func (e *Executor) executeDropStream(query *Query) (*QueryResult, error) {
	return &QueryResult{
		Columns: []string{"status"},
		Rows: [][]interface{}{
			{"stream dropped"},
		},
		Count: 1,
	}, nil
}

// executeDropTable handles DROP TABLE statements
func (e *Executor) executeDropTable(query *Query) (*QueryResult, error) {
	return &QueryResult{
		Columns: []string{"status"},
		Rows: [][]interface{}{
			{"table dropped"},
		},
		Count: 1,
	}, nil
}

// Helper functions

func (e *Executor) evaluateFilter(event *stream.Event, predicate string) bool {
	// Simplified filter evaluation
	// In a real implementation, this would parse and evaluate the predicate
	return true
}

func (e *Executor) applyProjection(event *stream.Event, columns []string) *stream.Event {
	// Simplified projection
	// In a real implementation, this would extract only the specified columns
	return event
}

func (e *Executor) eventToRow(event *stream.Event, columns []string) []interface{} {
	// Convert event to result row
	row := make([]interface{}, len(columns))

	if valueMap, ok := event.Value.(map[string]interface{}); ok {
		for i, col := range columns {
			if val, exists := valueMap[col]; exists {
				row[i] = val
			} else {
				row[i] = nil
			}
		}
	}

	return row
}

// StopContinuousQuery stops a running continuous query
func (e *Executor) StopContinuousQuery(queryID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cq, exists := e.continuousQueries[queryID]
	if !exists {
		return fmt.Errorf("continuous query %s not found", queryID)
	}

	close(cq.StopChannel)
	delete(e.continuousQueries, queryID)

	return nil
}

// ListContinuousQueries returns all running continuous queries
func (e *Executor) ListContinuousQueries() []*ContinuousQuery {
	e.mu.RLock()
	defer e.mu.RUnlock()

	queries := make([]*ContinuousQuery, 0, len(e.continuousQueries))
	for _, cq := range e.continuousQueries {
		queries = append(queries, cq)
	}

	return queries
}

// GetContinuousQuery returns a specific continuous query
func (e *Executor) GetContinuousQuery(queryID string) (*ContinuousQuery, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cq, exists := e.continuousQueries[queryID]
	if !exists {
		return nil, fmt.Errorf("continuous query %s not found", queryID)
	}

	return cq, nil
}

// ExplainQuery returns the execution plan for a query
func (e *Executor) ExplainQuery(sql string) (string, error) {
	// Parse query
	query, err := e.parser.Parse(sql)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	// Create execution plan
	plan, err := e.planner.Plan(query)
	if err != nil {
		return "", fmt.Errorf("planning error: %w", err)
	}

	// Optimize plan
	optimizedPlan, err := e.optimizer.Optimize(plan)
	if err != nil {
		return "", fmt.Errorf("optimization error: %w", err)
	}

	// Return explanation
	return e.optimizer.ExplainPlan(optimizedPlan), nil
}
