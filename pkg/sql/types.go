package sql

import (
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// DataType represents SQL data types
type DataType int

const (
	DataTypeString DataType = iota
	DataTypeInt
	DataTypeFloat
	DataTypeBool
	DataTypeTimestamp
	DataTypeBytes
)

// Column represents a column in a table or stream schema
type Column struct {
	Name     string
	Type     DataType
	Nullable bool
}

// Schema represents the schema of a table or stream
type Schema struct {
	Columns []Column
}

// Table represents a materialized view of a stream
type Table struct {
	Name       string
	Schema     Schema
	PrimaryKey []string
	State      map[string]interface{} // In-memory state for table
}

// StreamSource represents a registered stream source
type StreamSource struct {
	Name         string
	Schema       Schema
	EventChannel chan *stream.Event
	Watermark    time.Time
}

// QueryType represents the type of SQL query
type QueryType int

const (
	QueryTypeSelect QueryType = iota
	QueryTypeInsert
	QueryTypeUpdate
	QueryTypeDelete
	QueryTypeCreateStream
	QueryTypeCreateTable
	QueryTypeDropStream
	QueryTypeDropTable
)

// WindowSpec represents a window specification in SQL
type WindowSpec struct {
	Type     WindowType
	Size     time.Duration
	Slide    time.Duration // For sliding windows
	Gap      time.Duration // For session windows
	Offset   time.Duration
	AllowedLateness time.Duration
}

// WindowType represents the type of window
type WindowType int

const (
	WindowTypeTumbling WindowType = iota
	WindowTypeSliding
	WindowTypeSession
	WindowTypeHopping
)

// AggregateSpec represents an aggregation specification
type AggregateSpec struct {
	Function  string   // COUNT, SUM, AVG, MIN, MAX, etc.
	Column    string
	Alias     string
	Distinct  bool
}

// JoinSpec represents a join specification
type JoinSpec struct {
	Type      JoinType
	Left      string
	Right     string
	Condition string
	Window    *WindowSpec
}

// JoinType represents the type of join
type JoinType int

const (
	JoinTypeInner JoinType = iota
	JoinTypeLeft
	JoinTypeRight
	JoinTypeFull
)

// Query represents a parsed and validated SQL query
type Query struct {
	ID            string
	Type          QueryType
	SelectCols    []string
	FromStream    string
	JoinSpec      *JoinSpec
	WhereClause   string
	GroupByKeys   []string
	HavingClause  string
	WindowSpec    *WindowSpec
	OrderBy       []OrderBySpec
	Limit         int
	Aggregates    []AggregateSpec
	Continuous    bool // If true, this is a continuous query
}

// OrderBySpec represents an ORDER BY specification
type OrderBySpec struct {
	Column    string
	Ascending bool
}

// ContinuousQuery represents a continuously running SQL query
type ContinuousQuery struct {
	ID            string
	Query         *Query
	SourceStreams []string
	OutputChannel chan *stream.Event
	StopChannel   chan struct{}
	Started       time.Time
	EventsProcessed int64
	LastResult    time.Time
}

// QueryResult represents the result of a SQL query
type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
	Schema  Schema
	Count   int64
}

// UDF represents a User Defined Function
type UDF struct {
	Name       string
	InputTypes []DataType
	OutputType DataType
	Function   interface{} // Actual function implementation
}

// QueryPlan represents the execution plan for a query
type QueryPlan struct {
	Query     *Query
	Operators []PlanNode
	Cost      float64
	Optimized bool
}

// PlanNode represents a node in the query execution plan
type PlanNode interface {
	NodeType() string
	Children() []PlanNode
	Cost() float64
}

// ScanNode represents a table/stream scan
type ScanNode struct {
	SourceName string
	Schema     Schema
	Filter     string
	cost       float64
	children   []PlanNode
}

func (n *ScanNode) NodeType() string      { return "Scan" }
func (n *ScanNode) Children() []PlanNode  { return n.children }
func (n *ScanNode) Cost() float64         { return n.cost }

// FilterNode represents a filter operation
type FilterNode struct {
	Predicate string
	child     PlanNode
	cost      float64
}

func (n *FilterNode) NodeType() string     { return "Filter" }
func (n *FilterNode) Children() []PlanNode { return []PlanNode{n.child} }
func (n *FilterNode) Cost() float64        { return n.cost }

// ProjectNode represents a projection operation
type ProjectNode struct {
	Columns []string
	child   PlanNode
	cost    float64
}

func (n *ProjectNode) NodeType() string     { return "Project" }
func (n *ProjectNode) Children() []PlanNode { return []PlanNode{n.child} }
func (n *ProjectNode) Cost() float64        { return n.cost }

// AggregateNode represents an aggregation operation
type AggregateNode struct {
	Aggregates []AggregateSpec
	GroupBy    []string
	child      PlanNode
	cost       float64
}

func (n *AggregateNode) NodeType() string     { return "Aggregate" }
func (n *AggregateNode) Children() []PlanNode { return []PlanNode{n.child} }
func (n *AggregateNode) Cost() float64        { return n.cost }

// JoinNode represents a join operation
type JoinNode struct {
	JoinType  JoinType
	Condition string
	left      PlanNode
	right     PlanNode
	cost      float64
}

func (n *JoinNode) NodeType() string     { return "Join" }
func (n *JoinNode) Children() []PlanNode { return []PlanNode{n.left, n.right} }
func (n *JoinNode) Cost() float64        { return n.cost }

// WindowNode represents a windowing operation
type WindowNode struct {
	WindowSpec *WindowSpec
	child      PlanNode
	cost       float64
}

func (n *WindowNode) NodeType() string     { return "Window" }
func (n *WindowNode) Children() []PlanNode { return []PlanNode{n.child} }
func (n *WindowNode) Cost() float64        { return n.cost }
