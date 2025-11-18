package sql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parser parses SQL queries for stream processing
type Parser struct {
	catalog *Catalog
}

// NewParser creates a new SQL parser
func NewParser(catalog *Catalog) *Parser {
	return &Parser{
		catalog: catalog,
	}
}

// Parse parses a SQL query string and returns a Query object
func (p *Parser) Parse(sql string) (*Query, error) {
	sql = strings.TrimSpace(sql)

	// Determine query type
	sqlLower := strings.ToLower(sql)

	if strings.HasPrefix(sqlLower, "select") {
		return p.parseSelect(sql)
	} else if strings.HasPrefix(sqlLower, "create stream") {
		return p.parseCreateStream(sql)
	} else if strings.HasPrefix(sqlLower, "create table") {
		return p.parseCreateTable(sql)
	} else if strings.HasPrefix(sqlLower, "drop stream") {
		return p.parseDropStream(sql)
	} else if strings.HasPrefix(sqlLower, "drop table") {
		return p.parseDropTable(sql)
	} else if strings.HasPrefix(sqlLower, "insert") {
		return p.parseInsert(sql)
	}

	return nil, fmt.Errorf("unsupported query type")
}

// parseSelect parses a SELECT query
func (p *Parser) parseSelect(sql string) (*Query, error) {
	query := &Query{
		Type:       QueryTypeSelect,
		Continuous: false,
	}

	// Extract SELECT columns
	selectMatch := regexp.MustCompile(`(?i)SELECT\s+(.+?)\s+FROM`).FindStringSubmatch(sql)
	if len(selectMatch) < 2 {
		return nil, fmt.Errorf("invalid SELECT syntax")
	}

	// Parse SELECT columns and aggregates
	selectCols := strings.Split(selectMatch[1], ",")
	for _, col := range selectCols {
		col = strings.TrimSpace(col)

		// Check if it's an aggregate function
		if aggMatch := regexp.MustCompile(`(?i)(COUNT|SUM|AVG|MIN|MAX|STDDEV)\s*\(\s*(.+?)\s*\)(?:\s+AS\s+(\w+))?`).FindStringSubmatch(col); len(aggMatch) >= 3 {
			agg := AggregateSpec{
				Function: strings.ToUpper(aggMatch[1]),
				Column:   strings.TrimSpace(aggMatch[2]),
			}
			if len(aggMatch) >= 4 && aggMatch[3] != "" {
				agg.Alias = aggMatch[3]
			}
			// Check for DISTINCT
			if strings.Contains(strings.ToUpper(agg.Column), "DISTINCT") {
				agg.Distinct = true
				agg.Column = strings.TrimSpace(strings.Replace(strings.ToUpper(agg.Column), "DISTINCT", "", 1))
			}
			query.Aggregates = append(query.Aggregates, agg)
			if agg.Alias != "" {
				query.SelectCols = append(query.SelectCols, agg.Alias)
			} else {
				query.SelectCols = append(query.SelectCols, fmt.Sprintf("%s(%s)", agg.Function, agg.Column))
			}
		} else {
			// Regular column
			parts := strings.Split(col, " AS ")
			if len(parts) == 2 {
				query.SelectCols = append(query.SelectCols, strings.TrimSpace(parts[1]))
			} else {
				query.SelectCols = append(query.SelectCols, col)
			}
		}
	}

	// Extract FROM clause
	fromMatch := regexp.MustCompile(`(?i)FROM\s+(\w+)`).FindStringSubmatch(sql)
	if len(fromMatch) < 2 {
		return nil, fmt.Errorf("invalid FROM clause")
	}
	query.FromStream = fromMatch[1]

	// Extract JOIN clause if present
	if joinMatch := regexp.MustCompile(`(?i)JOIN\s+(\w+)\s+ON\s+(.+?)(?:\s+WHERE|\s+WINDOW|\s+GROUP|\s+HAVING|\s+ORDER|\s+LIMIT|$)`).FindStringSubmatch(sql); len(joinMatch) >= 3 {
		joinSpec := &JoinSpec{
			Type:      JoinTypeInner, // Default to INNER
			Right:     joinMatch[1],
			Condition: strings.TrimSpace(joinMatch[2]),
		}

		// Check for LEFT, RIGHT, FULL join types
		if strings.Contains(strings.ToUpper(sql), "LEFT JOIN") || strings.Contains(strings.ToUpper(sql), "LEFT OUTER JOIN") {
			joinSpec.Type = JoinTypeLeft
		} else if strings.Contains(strings.ToUpper(sql), "RIGHT JOIN") || strings.Contains(strings.ToUpper(sql), "RIGHT OUTER JOIN") {
			joinSpec.Type = JoinTypeRight
		} else if strings.Contains(strings.ToUpper(sql), "FULL JOIN") || strings.Contains(strings.ToUpper(sql), "FULL OUTER JOIN") {
			joinSpec.Type = JoinTypeFull
		}

		query.JoinSpec = joinSpec
	}

	// Extract WHERE clause
	if whereMatch := regexp.MustCompile(`(?i)WHERE\s+(.+?)(?:\s+WINDOW|\s+GROUP|\s+HAVING|\s+ORDER|\s+LIMIT|$)`).FindStringSubmatch(sql); len(whereMatch) >= 2 {
		query.WhereClause = strings.TrimSpace(whereMatch[1])
	}

	// Extract WINDOW clause
	if windowMatch := regexp.MustCompile(`(?i)WINDOW\s+(TUMBLING|SLIDING|SESSION|HOPPING)\s*\(\s*SIZE\s+(\d+)\s+(SECOND|SECONDS|MINUTE|MINUTES|HOUR|HOURS)(?:\s*,\s*SLIDE\s+(\d+)\s+(SECOND|SECONDS|MINUTE|MINUTES|HOUR|HOURS))?(?:\s*,\s*GAP\s+(\d+)\s+(SECOND|SECONDS|MINUTE|MINUTES|HOUR|HOURS))?\s*\)`).FindStringSubmatch(sql); len(windowMatch) >= 4 {
		windowSpec := &WindowSpec{}

		// Parse window type
		switch strings.ToUpper(windowMatch[1]) {
		case "TUMBLING":
			windowSpec.Type = WindowTypeTumbling
		case "SLIDING":
			windowSpec.Type = WindowTypeSliding
		case "SESSION":
			windowSpec.Type = WindowTypeSession
		case "HOPPING":
			windowSpec.Type = WindowTypeHopping
		}

		// Parse SIZE
		size, _ := strconv.Atoi(windowMatch[2])
		unit := strings.ToUpper(windowMatch[3])
		windowSpec.Size = parseDuration(size, unit)

		// Parse SLIDE (for sliding/hopping windows)
		if len(windowMatch) >= 6 && windowMatch[4] != "" {
			slide, _ := strconv.Atoi(windowMatch[4])
			slideUnit := strings.ToUpper(windowMatch[5])
			windowSpec.Slide = parseDuration(slide, slideUnit)
		}

		// Parse GAP (for session windows)
		if len(windowMatch) >= 8 && windowMatch[6] != "" {
			gap, _ := strconv.Atoi(windowMatch[6])
			gapUnit := strings.ToUpper(windowMatch[7])
			windowSpec.Gap = parseDuration(gap, gapUnit)
		}

		query.WindowSpec = windowSpec
	}

	// Extract GROUP BY clause
	if groupByMatch := regexp.MustCompile(`(?i)GROUP BY\s+(.+?)(?:\s+HAVING|\s+ORDER|\s+LIMIT|$)`).FindStringSubmatch(sql); len(groupByMatch) >= 2 {
		groupByCols := strings.Split(groupByMatch[1], ",")
		for _, col := range groupByCols {
			query.GroupByKeys = append(query.GroupByKeys, strings.TrimSpace(col))
		}
	}

	// Extract HAVING clause
	if havingMatch := regexp.MustCompile(`(?i)HAVING\s+(.+?)(?:\s+ORDER|\s+LIMIT|$)`).FindStringSubmatch(sql); len(havingMatch) >= 2 {
		query.HavingClause = strings.TrimSpace(havingMatch[1])
	}

	// Extract ORDER BY clause
	if orderByMatch := regexp.MustCompile(`(?i)ORDER BY\s+(.+?)(?:\s+LIMIT|$)`).FindStringSubmatch(sql); len(orderByMatch) >= 2 {
		orderByCols := strings.Split(orderByMatch[1], ",")
		for _, col := range orderByCols {
			col = strings.TrimSpace(col)
			orderSpec := OrderBySpec{
				Ascending: true,
			}
			if strings.HasSuffix(strings.ToUpper(col), " DESC") {
				orderSpec.Ascending = false
				col = strings.TrimSuffix(strings.ToUpper(col), " DESC")
			} else if strings.HasSuffix(strings.ToUpper(col), " ASC") {
				col = strings.TrimSuffix(strings.ToUpper(col), " ASC")
			}
			orderSpec.Column = strings.TrimSpace(col)
			query.OrderBy = append(query.OrderBy, orderSpec)
		}
	}

	// Extract LIMIT clause
	if limitMatch := regexp.MustCompile(`(?i)LIMIT\s+(\d+)`).FindStringSubmatch(sql); len(limitMatch) >= 2 {
		limit, _ := strconv.Atoi(limitMatch[1])
		query.Limit = limit
	}

	// Check if it's a continuous query (has WINDOW clause or no LIMIT)
	if query.WindowSpec != nil || query.Limit == 0 {
		query.Continuous = true
	}

	return query, nil
}

// parseCreateStream parses a CREATE STREAM statement
func (p *Parser) parseCreateStream(sql string) (*Query, error) {
	return &Query{
		Type: QueryTypeCreateStream,
	}, nil
}

// parseCreateTable parses a CREATE TABLE statement
func (p *Parser) parseCreateTable(sql string) (*Query, error) {
	return &Query{
		Type: QueryTypeCreateTable,
	}, nil
}

// parseDropStream parses a DROP STREAM statement
func (p *Parser) parseDropStream(sql string) (*Query, error) {
	return &Query{
		Type: QueryTypeDropStream,
	}, nil
}

// parseDropTable parses a DROP TABLE statement
func (p *Parser) parseDropTable(sql string) (*Query, error) {
	return &Query{
		Type: QueryTypeDropTable,
	}, nil
}

// parseInsert parses an INSERT statement
func (p *Parser) parseInsert(sql string) (*Query, error) {
	return &Query{
		Type: QueryTypeInsert,
	}, nil
}

// parseDuration converts a size and unit to a time.Duration
func parseDuration(size int, unit string) time.Duration {
	switch unit {
	case "SECOND", "SECONDS":
		return time.Duration(size) * time.Second
	case "MINUTE", "MINUTES":
		return time.Duration(size) * time.Minute
	case "HOUR", "HOURS":
		return time.Duration(size) * time.Hour
	default:
		return time.Duration(size) * time.Second
	}
}

// Validate validates a parsed query against the catalog
func (p *Parser) Validate(query *Query) error {
	// Validate stream exists
	if query.FromStream != "" && !p.catalog.StreamExists(query.FromStream) && !p.catalog.TableExists(query.FromStream) {
		return fmt.Errorf("stream or table %s not found", query.FromStream)
	}

	// Validate join target exists
	if query.JoinSpec != nil && query.JoinSpec.Right != "" {
		if !p.catalog.StreamExists(query.JoinSpec.Right) && !p.catalog.TableExists(query.JoinSpec.Right) {
			return fmt.Errorf("stream or table %s not found", query.JoinSpec.Right)
		}
	}

	// Validate aggregates are used with GROUP BY for non-window queries
	if len(query.Aggregates) > 0 && len(query.GroupByKeys) == 0 && query.WindowSpec == nil {
		// Check if all SELECT columns are aggregates
		if len(query.SelectCols) != len(query.Aggregates) {
			return fmt.Errorf("non-aggregate columns must be in GROUP BY clause")
		}
	}

	return nil
}
