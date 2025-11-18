package sql

import (
	"fmt"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/join"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"github.com/therealutkarshpriyadarshi/gress/pkg/window"
)

// Planner converts SQL queries to stream execution plans
type Planner struct {
	catalog *Catalog
}

// NewPlanner creates a new query planner
func NewPlanner(catalog *Catalog) *Planner {
	return &Planner{
		catalog: catalog,
	}
}

// Plan creates an execution plan for a query
func (p *Planner) Plan(query *Query) (*QueryPlan, error) {
	plan := &QueryPlan{
		Query:     query,
		Operators: []PlanNode{},
		Cost:      0,
		Optimized: false,
	}

	// Build plan nodes based on query
	var rootNode PlanNode
	var err error

	// 1. Start with scan node
	scanNode := &ScanNode{
		SourceName: query.FromStream,
		cost:       1.0,
	}
	rootNode = scanNode

	// Get source schema
	schema, err := p.catalog.GetSchema(query.FromStream)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for %s: %w", query.FromStream, err)
	}
	scanNode.Schema = schema

	// 2. Add filter node for WHERE clause
	if query.WhereClause != "" {
		filterNode := &FilterNode{
			Predicate: query.WhereClause,
			child:     rootNode,
			cost:      rootNode.Cost() * 0.5, // Assume 50% selectivity
		}
		rootNode = filterNode
	}

	// 3. Add join node if present
	if query.JoinSpec != nil {
		rightScanNode := &ScanNode{
			SourceName: query.JoinSpec.Right,
			cost:       1.0,
		}

		rightSchema, err := p.catalog.GetSchema(query.JoinSpec.Right)
		if err != nil {
			return nil, fmt.Errorf("failed to get schema for %s: %w", query.JoinSpec.Right, err)
		}
		rightScanNode.Schema = rightSchema

		joinNode := &JoinNode{
			JoinType:  query.JoinSpec.Type,
			Condition: query.JoinSpec.Condition,
			left:      rootNode,
			right:     rightScanNode,
			cost:      rootNode.Cost() * rightScanNode.Cost() * 2.0,
		}
		rootNode = joinNode
	}

	// 4. Add window node if present
	if query.WindowSpec != nil {
		windowNode := &WindowNode{
			WindowSpec: query.WindowSpec,
			child:      rootNode,
			cost:       rootNode.Cost() * 1.5,
		}
		rootNode = windowNode
	}

	// 5. Add aggregate node for GROUP BY
	if len(query.GroupByKeys) > 0 || len(query.Aggregates) > 0 {
		aggNode := &AggregateNode{
			Aggregates: query.Aggregates,
			GroupBy:    query.GroupByKeys,
			child:      rootNode,
			cost:       rootNode.Cost() * 2.0,
		}
		rootNode = aggNode
	}

	// 6. Add project node for SELECT
	if len(query.SelectCols) > 0 {
		projectNode := &ProjectNode{
			Columns: query.SelectCols,
			child:   rootNode,
			cost:    rootNode.Cost() * 1.1,
		}
		rootNode = projectNode
	}

	plan.Operators = []PlanNode{rootNode}
	plan.Cost = rootNode.Cost()

	return plan, nil
}

// BuildStreamPipeline converts a query plan to actual stream operators
func (p *Planner) BuildStreamPipeline(plan *QueryPlan, engine *stream.Engine) error {
	query := plan.Query

	// Get source stream
	sourceStream, err := p.catalog.GetStream(query.FromStream)
	if err != nil {
		return fmt.Errorf("failed to get source stream: %w", err)
	}

	// Create source operator - directly read from the stream's event channel
	sourceOp := stream.NewOperator("source", func(event *stream.Event, ctx stream.Context) ([]*stream.Event, error) {
		return []*stream.Event{event}, nil
	})

	// Add WHERE filter if present
	if query.WhereClause != "" {
		filterOp := p.buildFilterOperator(query.WhereClause)
		sourceOp = stream.ChainOperators(sourceOp, filterOp)
	}

	// Add JOIN if present
	if query.JoinSpec != nil {
		joinOp, err := p.buildJoinOperator(query.JoinSpec, engine)
		if err != nil {
			return fmt.Errorf("failed to build join operator: %w", err)
		}
		sourceOp = stream.ChainOperators(sourceOp, joinOp)
	}

	// Add WINDOW if present
	if query.WindowSpec != nil {
		windowOp := p.buildWindowOperator(query.WindowSpec, query.GroupByKeys, query.Aggregates)
		sourceOp = stream.ChainOperators(sourceOp, windowOp)
	} else if len(query.Aggregates) > 0 {
		// Add aggregation without window
		aggOp := p.buildAggregateOperator(query.GroupByKeys, query.Aggregates)
		sourceOp = stream.ChainOperators(sourceOp, aggOp)
	}

	// Register the pipeline with the engine
	// The engine will process events from sourceStream.EventChannel
	// This is where we'd connect the stream to the engine
	// For now, we mark this as a placeholder for integration

	return nil
}

// buildFilterOperator creates a filter operator from a WHERE clause
func (p *Planner) buildFilterOperator(whereClause string) stream.Operator {
	return stream.NewOperator("filter", func(event *stream.Event, ctx stream.Context) ([]*stream.Event, error) {
		// Evaluate the WHERE clause
		// This is a simplified implementation - a real implementation would parse and evaluate the expression
		if p.evaluatePredicate(whereClause, event) {
			return []*stream.Event{event}, nil
		}
		return nil, nil
	})
}

// buildJoinOperator creates a join operator
func (p *Planner) buildJoinOperator(joinSpec *JoinSpec, engine *stream.Engine) (stream.Operator, error) {
	// Map SQL join types to stream join types
	var joinType join.JoinType
	switch joinSpec.Type {
	case JoinTypeInner:
		joinType = join.InnerJoin
	case JoinTypeLeft:
		joinType = join.LeftOuterJoin
	case JoinTypeRight:
		joinType = join.RightOuterJoin
	case JoinTypeFull:
		joinType = join.FullOuterJoin
	default:
		joinType = join.InnerJoin
	}

	// Create join configuration
	config := join.JoinConfig{
		Type:     joinType,
		Strategy: join.HashJoin, // Default to hash join
		Condition: join.JoinCondition{
			LeftKey:  "", // Would be extracted from joinSpec.Condition
			RightKey: "",
		},
		TimeWindow:      5 * time.Minute,
		AllowedLateness: 30 * time.Second,
	}

	// Create join operator
	rightStream, err := p.catalog.GetStream(joinSpec.Right)
	if err != nil {
		return nil, fmt.Errorf("failed to get right stream: %w", err)
	}

	joinOp := join.NewJoinOperator(config, nil, rightStream.EventChannel)

	// Wrap join operator in stream.Operator interface
	return stream.NewOperator("join", func(event *stream.Event, ctx stream.Context) ([]*stream.Event, error) {
		// Process through join operator
		result, err := joinOp.Process(event, ctx)
		if err != nil {
			return nil, err
		}
		return result, nil
	}), nil
}

// buildWindowOperator creates a window operator
func (p *Planner) buildWindowOperator(windowSpec *WindowSpec, groupByKeys []string, aggregates []AggregateSpec) stream.Operator {
	// Create window assigner
	var assigner window.Assigner
	switch windowSpec.Type {
	case WindowTypeTumbling:
		assigner = window.NewTumblingWindow(windowSpec.Size)
	case WindowTypeSliding:
		assigner = window.NewSlidingWindow(windowSpec.Size, windowSpec.Slide)
	case WindowTypeSession:
		assigner = window.NewSessionWindow(windowSpec.Gap)
	default:
		assigner = window.NewTumblingWindow(windowSpec.Size)
	}

	// Create trigger
	trigger := window.NewEventTimeTrigger()

	// Create aggregate function
	aggregateFunc := p.buildAggregateFunction(aggregates)

	// Create window operator
	windowOp := window.NewWindowOperator(assigner, trigger, aggregateFunc)

	// Wrap in stream.Operator interface
	return stream.NewOperator("window", func(event *stream.Event, ctx stream.Context) ([]*stream.Event, error) {
		return windowOp.Process(event, ctx)
	})
}

// buildAggregateOperator creates an aggregate operator without windowing
func (p *Planner) buildAggregateOperator(groupByKeys []string, aggregates []AggregateSpec) stream.Operator {
	aggregateFunc := p.buildAggregateFunction(aggregates)

	return stream.NewOperator("aggregate", func(event *stream.Event, ctx stream.Context) ([]*stream.Event, error) {
		// Get or create aggregation state for this key
		key := event.Key
		if len(groupByKeys) > 0 {
			// Build composite key from group by columns
			key = p.buildGroupKey(event, groupByKeys)
		}

		// Apply aggregate function
		result := aggregateFunc(event, nil)

		// Return aggregated event
		return []*stream.Event{{
			Key:       key,
			Value:     result,
			EventTime: event.EventTime,
		}}, nil
	})
}

// buildAggregateFunction creates an aggregate function from aggregate specs
func (p *Planner) buildAggregateFunction(aggregates []AggregateSpec) stream.AggregateFunc {
	return func(event *stream.Event, currentState interface{}) interface{} {
		result := make(map[string]interface{})

		for _, agg := range aggregates {
			switch agg.Function {
			case "COUNT":
				// Count aggregation
				if currentState == nil {
					result[agg.Alias] = 1
				} else {
					state := currentState.(map[string]interface{})
					count := state[agg.Alias].(int)
					result[agg.Alias] = count + 1
				}
			case "SUM":
				// Sum aggregation
				value := p.extractColumnValue(event, agg.Column)
				if currentState == nil {
					result[agg.Alias] = value
				} else {
					state := currentState.(map[string]interface{})
					sum := state[agg.Alias]
					result[agg.Alias] = addValues(sum, value)
				}
			case "AVG":
				// Average aggregation (needs count and sum)
				value := p.extractColumnValue(event, agg.Column)
				if currentState == nil {
					result[agg.Alias] = value
					result[agg.Alias+"_count"] = 1
				} else {
					state := currentState.(map[string]interface{})
					sum := state[agg.Alias]
					count := state[agg.Alias+"_count"].(int)
					result[agg.Alias] = addValues(sum, value)
					result[agg.Alias+"_count"] = count + 1
				}
			case "MIN":
				// Min aggregation
				value := p.extractColumnValue(event, agg.Column)
				if currentState == nil {
					result[agg.Alias] = value
				} else {
					state := currentState.(map[string]interface{})
					min := state[agg.Alias]
					result[agg.Alias] = minValue(min, value)
				}
			case "MAX":
				// Max aggregation
				value := p.extractColumnValue(event, agg.Column)
				if currentState == nil {
					result[agg.Alias] = value
				} else {
					state := currentState.(map[string]interface{})
					max := state[agg.Alias]
					result[agg.Alias] = maxValue(max, value)
				}
			}
		}

		return result
	}
}

// Helper functions

func (p *Planner) evaluatePredicate(predicate string, event *stream.Event) bool {
	// Simplified predicate evaluation
	// In a real implementation, this would parse and evaluate the predicate expression
	return true
}

func (p *Planner) buildGroupKey(event *stream.Event, groupByKeys []string) string {
	// Build composite key from group by columns
	key := ""
	for _, col := range groupByKeys {
		value := p.extractColumnValue(event, col)
		key += fmt.Sprintf("%v_", value)
	}
	return key
}

func (p *Planner) extractColumnValue(event *stream.Event, column string) interface{} {
	// Extract column value from event
	// This is simplified - would need to handle nested fields, etc.
	if column == "*" {
		return event.Value
	}

	// Try to extract from event value if it's a map
	if valueMap, ok := event.Value.(map[string]interface{}); ok {
		if val, exists := valueMap[column]; exists {
			return val
		}
	}

	return nil
}

func addValues(a, b interface{}) interface{} {
	// Add two values (handles int, float)
	switch v := a.(type) {
	case int:
		return v + b.(int)
	case int64:
		return v + b.(int64)
	case float64:
		return v + b.(float64)
	default:
		return a
	}
}

func minValue(a, b interface{}) interface{} {
	// Return minimum of two values
	switch v := a.(type) {
	case int:
		bv := b.(int)
		if v < bv {
			return v
		}
		return bv
	case int64:
		bv := b.(int64)
		if v < bv {
			return v
		}
		return bv
	case float64:
		bv := b.(float64)
		if v < bv {
			return v
		}
		return bv
	default:
		return a
	}
}

func maxValue(a, b interface{}) interface{} {
	// Return maximum of two values
	switch v := a.(type) {
	case int:
		bv := b.(int)
		if v > bv {
			return v
		}
		return bv
	case int64:
		bv := b.(int64)
		if v > bv {
			return v
		}
		return bv
	case float64:
		bv := b.(float64)
		if v > bv {
			return v
		}
		return bv
	default:
		return a
	}
}
