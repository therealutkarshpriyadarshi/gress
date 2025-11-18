package sql

import (
	"fmt"
)

// Optimizer optimizes query execution plans
type Optimizer struct {
	catalog *Catalog
}

// NewOptimizer creates a new query optimizer
func NewOptimizer(catalog *Catalog) *Optimizer {
	return &Optimizer{
		catalog: catalog,
	}
}

// Optimize applies optimization rules to a query plan
func (o *Optimizer) Optimize(plan *QueryPlan) (*QueryPlan, error) {
	if plan.Optimized {
		return plan, nil
	}

	optimizedPlan := &QueryPlan{
		Query:     plan.Query,
		Operators: plan.Operators,
		Cost:      plan.Cost,
		Optimized: false,
	}

	// Apply optimization rules in order
	var err error

	// 1. Predicate pushdown - push filters as close to source as possible
	optimizedPlan, err = o.applyPredicatePushdown(optimizedPlan)
	if err != nil {
		return nil, fmt.Errorf("predicate pushdown failed: %w", err)
	}

	// 2. Projection pushdown - minimize columns early
	optimizedPlan, err = o.applyProjectionPushdown(optimizedPlan)
	if err != nil {
		return nil, fmt.Errorf("projection pushdown failed: %w", err)
	}

	// 3. Join reordering - optimize join order
	optimizedPlan, err = o.applyJoinReordering(optimizedPlan)
	if err != nil {
		return nil, fmt.Errorf("join reordering failed: %w", err)
	}

	// 4. Window optimization - merge adjacent windows
	optimizedPlan, err = o.applyWindowOptimization(optimizedPlan)
	if err != nil {
		return nil, fmt.Errorf("window optimization failed: %w", err)
	}

	// 5. Aggregate optimization - combine aggregates
	optimizedPlan, err = o.applyAggregateOptimization(optimizedPlan)
	if err != nil {
		return nil, fmt.Errorf("aggregate optimization failed: %w", err)
	}

	// Recalculate cost
	optimizedPlan.Cost = o.calculatePlanCost(optimizedPlan)
	optimizedPlan.Optimized = true

	return optimizedPlan, nil
}

// applyPredicatePushdown pushes filter predicates as close to the source as possible
func (o *Optimizer) applyPredicatePushdown(plan *QueryPlan) (*QueryPlan, error) {
	// This is a simplified implementation
	// In a real optimizer, we would:
	// 1. Identify filter nodes
	// 2. Analyze predicates to see if they can be pushed down
	// 3. Move filters below joins if they only reference one side
	// 4. Move filters into scan nodes if they're on indexed columns

	// For now, we just return the plan as-is
	// This would be a significant optimization in production
	return plan, nil
}

// applyProjectionPushdown pushes projections down to minimize data movement
func (o *Optimizer) applyProjectionPushdown(plan *QueryPlan) (*QueryPlan, error) {
	// This optimization:
	// 1. Identifies which columns are actually needed
	// 2. Pushes projections down to scan nodes
	// 3. Eliminates unnecessary column reads

	// Simplified implementation - return plan as-is
	return plan, nil
}

// applyJoinReordering reorders joins for optimal execution
func (o *Optimizer) applyJoinReordering(plan *QueryPlan) (*QueryPlan, error) {
	// This optimization:
	// 1. Estimates cardinality of each join input
	// 2. Reorders joins to minimize intermediate results
	// 3. Typically uses dynamic programming or greedy algorithm

	// For streaming, we also consider:
	// - Data arrival rates
	// - State size implications
	// - Watermark dependencies

	// Simplified implementation - return plan as-is
	return plan, nil
}

// applyWindowOptimization optimizes window operations
func (o *Optimizer) applyWindowOptimization(plan *QueryPlan) (*QueryPlan, error) {
	// This optimization:
	// 1. Merges adjacent windows with same specification
	// 2. Shares window state across multiple aggregations
	// 3. Optimizes trigger firing

	// Simplified implementation - return plan as-is
	return plan, nil
}

// applyAggregateOptimization optimizes aggregate operations
func (o *Optimizer) applyAggregateOptimization(plan *QueryPlan) (*QueryPlan, error) {
	// This optimization:
	// 1. Combines multiple aggregates into single pass
	// 2. Uses incremental aggregation where possible
	// 3. Optimizes state management

	// Simplified implementation - return plan as-is
	return plan, nil
}

// calculatePlanCost calculates the estimated cost of a plan
func (o *Optimizer) calculatePlanCost(plan *QueryPlan) float64 {
	totalCost := 0.0

	for _, node := range plan.Operators {
		totalCost += o.calculateNodeCost(node)
	}

	return totalCost
}

// calculateNodeCost calculates the cost of a single plan node
func (o *Optimizer) calculateNodeCost(node PlanNode) float64 {
	baseCost := node.Cost()

	// Add costs of children
	for _, child := range node.Children() {
		baseCost += o.calculateNodeCost(child)
	}

	return baseCost
}

// ExplainPlan returns a human-readable explanation of the query plan
func (o *Optimizer) ExplainPlan(plan *QueryPlan) string {
	explanation := fmt.Sprintf("Query Plan (Cost: %.2f, Optimized: %v)\n", plan.Cost, plan.Optimized)
	explanation += "==============================================\n"

	for i, node := range plan.Operators {
		explanation += fmt.Sprintf("%d. %s\n", i+1, o.explainNode(node, 0))
	}

	return explanation
}

// explainNode recursively explains a plan node
func (o *Optimizer) explainNode(node PlanNode, depth int) string {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	explanation := fmt.Sprintf("%s%s (Cost: %.2f)\n", indent, node.NodeType(), node.Cost())

	// Add node-specific details
	switch n := node.(type) {
	case *ScanNode:
		explanation += fmt.Sprintf("%s  Source: %s\n", indent, n.SourceName)
	case *FilterNode:
		explanation += fmt.Sprintf("%s  Predicate: %s\n", indent, n.Predicate)
	case *ProjectNode:
		explanation += fmt.Sprintf("%s  Columns: %v\n", indent, n.Columns)
	case *AggregateNode:
		explanation += fmt.Sprintf("%s  GroupBy: %v\n", indent, n.GroupBy)
		explanation += fmt.Sprintf("%s  Aggregates: %v\n", indent, n.Aggregates)
	case *JoinNode:
		explanation += fmt.Sprintf("%s  Type: %v\n", indent, n.JoinType)
		explanation += fmt.Sprintf("%s  Condition: %s\n", indent, n.Condition)
	case *WindowNode:
		explanation += fmt.Sprintf("%s  WindowType: %v\n", indent, n.WindowSpec.Type)
		explanation += fmt.Sprintf("%s  Size: %v\n", indent, n.WindowSpec.Size)
	}

	// Explain children
	for _, child := range node.Children() {
		explanation += o.explainNode(child, depth+1)
	}

	return explanation
}

// OptimizationStats tracks optimization statistics
type OptimizationStats struct {
	OriginalCost   float64
	OptimizedCost  float64
	Improvement    float64
	RulesApplied   []string
	ExecutionTime  float64
}

// GetOptimizationStats returns statistics about the optimization
func (o *Optimizer) GetOptimizationStats(originalPlan, optimizedPlan *QueryPlan) OptimizationStats {
	improvement := 0.0
	if originalPlan.Cost > 0 {
		improvement = ((originalPlan.Cost - optimizedPlan.Cost) / originalPlan.Cost) * 100.0
	}

	return OptimizationStats{
		OriginalCost:  originalPlan.Cost,
		OptimizedCost: optimizedPlan.Cost,
		Improvement:   improvement,
		RulesApplied: []string{
			"PredicatePushdown",
			"ProjectionPushdown",
			"JoinReordering",
			"WindowOptimization",
			"AggregateOptimization",
		},
	}
}

// ValidatePlan validates that a plan is semantically correct
func (o *Optimizer) ValidatePlan(plan *QueryPlan) error {
	// Check that all referenced streams/tables exist
	for _, node := range plan.Operators {
		if err := o.validateNode(node); err != nil {
			return err
		}
	}

	return nil
}

// validateNode validates a single plan node
func (o *Optimizer) validateNode(node PlanNode) error {
	switch n := node.(type) {
	case *ScanNode:
		// Validate source exists
		if !o.catalog.StreamExists(n.SourceName) && !o.catalog.TableExists(n.SourceName) {
			return fmt.Errorf("source %s not found", n.SourceName)
		}
	case *JoinNode:
		// Validate children
		for _, child := range n.Children() {
			if err := o.validateNode(child); err != nil {
				return err
			}
		}
	}

	// Validate children
	for _, child := range node.Children() {
		if err := o.validateNode(child); err != nil {
			return err
		}
	}

	return nil
}
