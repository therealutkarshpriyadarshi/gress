package sql

import (
	"fmt"
	"sync"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// Catalog manages registered streams, tables, and UDFs
type Catalog struct {
	mu      sync.RWMutex
	streams map[string]*StreamSource
	tables  map[string]*Table
	udfs    map[string]*UDF
}

// NewCatalog creates a new catalog
func NewCatalog() *Catalog {
	return &Catalog{
		streams: make(map[string]*StreamSource),
		tables:  make(map[string]*Table),
		udfs:    make(map[string]*UDF),
	}
}

// RegisterStream registers a new stream source
func (c *Catalog) RegisterStream(name string, schema Schema, eventChannel chan *stream.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.streams[name]; exists {
		return fmt.Errorf("stream %s already exists", name)
	}

	c.streams[name] = &StreamSource{
		Name:         name,
		Schema:       schema,
		EventChannel: eventChannel,
	}

	return nil
}

// GetStream retrieves a stream by name
func (c *Catalog) GetStream(name string) (*StreamSource, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stream, exists := c.streams[name]
	if !exists {
		return nil, fmt.Errorf("stream %s not found", name)
	}

	return stream, nil
}

// UnregisterStream removes a stream from the catalog
func (c *Catalog) UnregisterStream(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.streams[name]; !exists {
		return fmt.Errorf("stream %s not found", name)
	}

	delete(c.streams, name)
	return nil
}

// ListStreams returns all registered stream names
func (c *Catalog) ListStreams() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.streams))
	for name := range c.streams {
		names = append(names, name)
	}
	return names
}

// RegisterTable registers a new table
func (c *Catalog) RegisterTable(name string, schema Schema, primaryKey []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tables[name]; exists {
		return fmt.Errorf("table %s already exists", name)
	}

	c.tables[name] = &Table{
		Name:       name,
		Schema:     schema,
		PrimaryKey: primaryKey,
		State:      make(map[string]interface{}),
	}

	return nil
}

// GetTable retrieves a table by name
func (c *Catalog) GetTable(name string) (*Table, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	table, exists := c.tables[name]
	if !exists {
		return nil, fmt.Errorf("table %s not found", name)
	}

	return table, nil
}

// UnregisterTable removes a table from the catalog
func (c *Catalog) UnregisterTable(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tables[name]; !exists {
		return fmt.Errorf("table %s not found", name)
	}

	delete(c.tables, name)
	return nil
}

// ListTables returns all registered table names
func (c *Catalog) ListTables() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.tables))
	for name := range c.tables {
		names = append(names, name)
	}
	return names
}

// RegisterUDF registers a new User Defined Function
func (c *Catalog) RegisterUDF(name string, inputTypes []DataType, outputType DataType, function interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.udfs[name]; exists {
		return fmt.Errorf("UDF %s already exists", name)
	}

	c.udfs[name] = &UDF{
		Name:       name,
		InputTypes: inputTypes,
		OutputType: outputType,
		Function:   function,
	}

	return nil
}

// GetUDF retrieves a UDF by name
func (c *Catalog) GetUDF(name string) (*UDF, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	udf, exists := c.udfs[name]
	if !exists {
		return nil, fmt.Errorf("UDF %s not found", name)
	}

	return udf, nil
}

// UnregisterUDF removes a UDF from the catalog
func (c *Catalog) UnregisterUDF(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.udfs[name]; !exists {
		return fmt.Errorf("UDF %s not found", name)
	}

	delete(c.udfs, name)
	return nil
}

// ListUDFs returns all registered UDF names
func (c *Catalog) ListUDFs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.udfs))
	for name := range c.udfs {
		names = append(names, name)
	}
	return names
}

// GetSchema retrieves the schema for a stream or table
func (c *Catalog) GetSchema(name string) (Schema, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check streams first
	if stream, exists := c.streams[name]; exists {
		return stream.Schema, nil
	}

	// Check tables
	if table, exists := c.tables[name]; exists {
		return table.Schema, nil
	}

	return Schema{}, fmt.Errorf("stream or table %s not found", name)
}

// StreamExists checks if a stream exists
func (c *Catalog) StreamExists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.streams[name]
	return exists
}

// TableExists checks if a table exists
func (c *Catalog) TableExists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.tables[name]
	return exists
}

// UDFExists checks if a UDF exists
func (c *Catalog) UDFExists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.udfs[name]
	return exists
}
