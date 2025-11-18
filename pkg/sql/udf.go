package sql

import (
	"fmt"
	"reflect"
	"strings"
)

// UDFRegistry manages user-defined functions
type UDFRegistry struct {
	catalog *Catalog
}

// NewUDFRegistry creates a new UDF registry
func NewUDFRegistry(catalog *Catalog) *UDFRegistry {
	registry := &UDFRegistry{
		catalog: catalog,
	}

	// Register built-in UDFs
	registry.registerBuiltinUDFs()

	return registry
}

// registerBuiltinUDFs registers built-in UDFs
func (r *UDFRegistry) registerBuiltinUDFs() {
	// String functions
	r.RegisterScalarUDF("upper", []DataType{DataTypeString}, DataTypeString,
		func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("upper requires 1 argument")
			}
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("upper requires string argument")
			}
			return strings.ToUpper(str), nil
		})

	r.RegisterScalarUDF("lower", []DataType{DataTypeString}, DataTypeString,
		func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("lower requires 1 argument")
			}
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("lower requires string argument")
			}
			return strings.ToLower(str), nil
		})

	r.RegisterScalarUDF("concat", []DataType{DataTypeString, DataTypeString}, DataTypeString,
		func(args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("concat requires at least 2 arguments")
			}
			result := ""
			for _, arg := range args {
				str, ok := arg.(string)
				if !ok {
					return nil, fmt.Errorf("concat requires string arguments")
				}
				result += str
			}
			return result, nil
		})

	r.RegisterScalarUDF("substring", []DataType{DataTypeString, DataTypeInt, DataTypeInt}, DataTypeString,
		func(args ...interface{}) (interface{}, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("substring requires 3 arguments")
			}
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("substring requires string as first argument")
			}
			start, ok := args[1].(int)
			if !ok {
				return nil, fmt.Errorf("substring requires int as second argument")
			}
			length, ok := args[2].(int)
			if !ok {
				return nil, fmt.Errorf("substring requires int as third argument")
			}

			if start < 0 || start >= len(str) {
				return "", nil
			}
			end := start + length
			if end > len(str) {
				end = len(str)
			}
			return str[start:end], nil
		})

	// Math functions
	r.RegisterScalarUDF("abs", []DataType{DataTypeFloat}, DataTypeFloat,
		func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("abs requires 1 argument")
			}

			// Handle different numeric types
			switch v := args[0].(type) {
			case int:
				if v < 0 {
					return -v, nil
				}
				return v, nil
			case int64:
				if v < 0 {
					return -v, nil
				}
				return v, nil
			case float64:
				if v < 0 {
					return -v, nil
				}
				return v, nil
			default:
				return nil, fmt.Errorf("abs requires numeric argument")
			}
		})

	r.RegisterScalarUDF("round", []DataType{DataTypeFloat}, DataTypeFloat,
		func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("round requires 1 argument")
			}
			num, ok := args[0].(float64)
			if !ok {
				return nil, fmt.Errorf("round requires float argument")
			}
			return float64(int(num + 0.5)), nil
		})

	// Conditional functions
	r.RegisterScalarUDF("coalesce", []DataType{}, DataTypeString,
		func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("coalesce requires at least 1 argument")
			}
			for _, arg := range args {
				if arg != nil {
					return arg, nil
				}
			}
			return nil, nil
		})

	// Type conversion functions
	r.RegisterScalarUDF("to_string", []DataType{}, DataTypeString,
		func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("to_string requires 1 argument")
			}
			return fmt.Sprintf("%v", args[0]), nil
		})
}

// RegisterScalarUDF registers a scalar (non-aggregate) UDF
func (r *UDFRegistry) RegisterScalarUDF(name string, inputTypes []DataType, outputType DataType, function interface{}) error {
	// Validate function signature
	if err := r.validateUDFFunction(function, len(inputTypes)); err != nil {
		return fmt.Errorf("invalid UDF function: %w", err)
	}

	return r.catalog.RegisterUDF(name, inputTypes, outputType, function)
}

// RegisterAggregateUDF registers an aggregate UDF
func (r *UDFRegistry) RegisterAggregateUDF(name string, inputType DataType, outputType DataType, function interface{}) error {
	// Validate function signature for aggregate
	// Aggregate functions should have signature: (accumulator, value) -> accumulator
	if err := r.validateAggregateFunction(function); err != nil {
		return fmt.Errorf("invalid aggregate UDF function: %w", err)
	}

	return r.catalog.RegisterUDF(name, []DataType{inputType}, outputType, function)
}

// RegisterTableUDF registers a table-valued function (returns multiple rows)
func (r *UDFRegistry) RegisterTableUDF(name string, inputTypes []DataType, outputSchema Schema, function interface{}) error {
	// Validate function signature for table UDF
	if err := r.validateTableUDFFunction(function); err != nil {
		return fmt.Errorf("invalid table UDF function: %w", err)
	}

	return r.catalog.RegisterUDF(name, inputTypes, DataTypeString, function) // Use DataTypeString as placeholder
}

// CallUDF invokes a UDF with the given arguments
func (r *UDFRegistry) CallUDF(name string, args ...interface{}) (interface{}, error) {
	udf, err := r.catalog.GetUDF(name)
	if err != nil {
		return nil, fmt.Errorf("UDF %s not found: %w", name, err)
	}

	// Get function value
	fn := reflect.ValueOf(udf.Function)

	// Validate argument count
	if fn.Type().NumIn() != 1 && fn.Type().NumIn() != len(args) {
		// Check if it's a variadic function
		if !fn.Type().IsVariadic() {
			return nil, fmt.Errorf("UDF %s expects %d arguments, got %d", name, fn.Type().NumIn(), len(args))
		}
	}

	// Convert arguments to reflect.Value
	reflectArgs := make([]reflect.Value, len(args))
	for i, arg := range args {
		reflectArgs[i] = reflect.ValueOf(arg)
	}

	// Call function
	results := fn.Call(reflectArgs)

	// Handle results
	if len(results) == 0 {
		return nil, fmt.Errorf("UDF %s returned no results", name)
	}

	// Check for error return
	if len(results) == 2 {
		if !results[1].IsNil() {
			err, ok := results[1].Interface().(error)
			if ok {
				return nil, err
			}
		}
	}

	return results[0].Interface(), nil
}

// validateUDFFunction validates a UDF function signature
func (r *UDFRegistry) validateUDFFunction(function interface{}, expectedArgs int) error {
	fn := reflect.TypeOf(function)

	// Must be a function
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("UDF must be a function")
	}

	// Check argument count (allow variadic)
	if !fn.IsVariadic() && fn.NumIn() != 1 && fn.NumIn() != expectedArgs {
		return fmt.Errorf("UDF function has wrong number of arguments: expected %d, got %d", expectedArgs, fn.NumIn())
	}

	// Must return at least one value
	if fn.NumOut() < 1 || fn.NumOut() > 2 {
		return fmt.Errorf("UDF function must return 1 or 2 values (result and optional error)")
	}

	// If 2 return values, second must be error
	if fn.NumOut() == 2 {
		errorInterface := reflect.TypeOf((*error)(nil)).Elem()
		if !fn.Out(1).Implements(errorInterface) {
			return fmt.Errorf("second return value must be error")
		}
	}

	return nil
}

// validateAggregateFunction validates an aggregate UDF function signature
func (r *UDFRegistry) validateAggregateFunction(function interface{}) error {
	fn := reflect.TypeOf(function)

	// Must be a function
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("aggregate UDF must be a function")
	}

	// Must take 2 arguments (accumulator, value)
	if fn.NumIn() != 2 {
		return fmt.Errorf("aggregate UDF function must take 2 arguments (accumulator, value)")
	}

	// Must return at least one value
	if fn.NumOut() < 1 || fn.NumOut() > 2 {
		return fmt.Errorf("aggregate UDF function must return 1 or 2 values (result and optional error)")
	}

	return nil
}

// validateTableUDFFunction validates a table-valued UDF function signature
func (r *UDFRegistry) validateTableUDFFunction(function interface{}) error {
	fn := reflect.TypeOf(function)

	// Must be a function
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("table UDF must be a function")
	}

	// Must return slice or array
	if fn.NumOut() < 1 {
		return fmt.Errorf("table UDF must return at least one value")
	}

	returnType := fn.Out(0)
	if returnType.Kind() != reflect.Slice && returnType.Kind() != reflect.Array {
		return fmt.Errorf("table UDF must return a slice or array")
	}

	return nil
}

// ListUDFs returns all registered UDFs
func (r *UDFRegistry) ListUDFs() []string {
	return r.catalog.ListUDFs()
}

// GetUDFInfo returns information about a UDF
func (r *UDFRegistry) GetUDFInfo(name string) (*UDF, error) {
	return r.catalog.GetUDF(name)
}

// UnregisterUDF removes a UDF from the registry
func (r *UDFRegistry) UnregisterUDF(name string) error {
	return r.catalog.UnregisterUDF(name)
}

// Example custom aggregate UDF
// This demonstrates how users can create their own aggregate functions

// MedianAggregate computes the median of values
type MedianAggregate struct {
	values []float64
}

// NewMedianAggregate creates a new median aggregate
func NewMedianAggregate() *MedianAggregate {
	return &MedianAggregate{
		values: []float64{},
	}
}

// Accumulate adds a value to the aggregate
func (m *MedianAggregate) Accumulate(value float64) {
	m.values = append(m.values, value)
}

// Compute computes the final median value
func (m *MedianAggregate) Compute() float64 {
	if len(m.values) == 0 {
		return 0
	}

	// Sort values
	sorted := make([]float64, len(m.values))
	copy(sorted, m.values)

	// Simple bubble sort for demonstration
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate median
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// Example window function UDF
// This demonstrates stateful UDFs that maintain state across calls

// MovingAverageUDF computes a moving average
type MovingAverageUDF struct {
	windowSize int
	values     []float64
}

// NewMovingAverageUDF creates a new moving average UDF
func NewMovingAverageUDF(windowSize int) *MovingAverageUDF {
	return &MovingAverageUDF{
		windowSize: windowSize,
		values:     []float64{},
	}
}

// Process adds a new value and returns the current moving average
func (m *MovingAverageUDF) Process(value float64) float64 {
	m.values = append(m.values, value)

	// Keep only the last windowSize values
	if len(m.values) > m.windowSize {
		m.values = m.values[1:]
	}

	// Calculate average
	sum := 0.0
	for _, v := range m.values {
		sum += v
	}

	return sum / float64(len(m.values))
}
