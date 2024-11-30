package dbutils

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
)

// MapSqlRows maps rows from a SQL query to a slice of map[string]interface{}
func MapSqlRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Result slice
	var results []map[string]interface{}

	// Iterate over rows
	for rows.Next() {
		// Create a slice of interface{} to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		// Create pointers for sql.Scan
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle NULL values
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return the result
	return results, nil
}

// MapToStruct maps a map[string]interface{} to a struct
func MapToStruct[T any](data map[string]interface{}) (dest *T, err error) { 
	// Validate that dest is a pointer to a struct
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		err = errors.New("dest must be a pointer to a struct")
		return
	}

	destVal = destVal.Elem()
	destType := destVal.Type()

	// Iterate over struct fields and set values from the map
	for i := 0; i < destVal.NumField(); i++ {
		field := destVal.Field(i)
		fieldType := destType.Field(i)

		// Get field name or JSON tag
		mapKey := fieldType.Name
		if tag := fieldType.Tag.Get("db"); tag != "" {
			mapKey = strings.Split(tag, ",")[0] // Handle "json" tags like `json:"field_name,omitempty"`
		}

		// Find the value in the map
		if value, exists := data[mapKey]; exists {
			if value != nil {
				val := reflect.ValueOf(value)

				// Ensure the types are compatible
				if field.Kind() == val.Kind() || (field.Kind() == reflect.Ptr && field.Type().Elem() == val.Type()) {
					if field.Kind() == reflect.Ptr {
						ptr := reflect.New(field.Type().Elem())
						ptr.Elem().Set(val)
						field.Set(ptr)
					} else {
						field.Set(val)
					}
				} else {
					err = errors.New("type mismatch for field: " + fieldType.Name)
				}
			}
		}
	}
	
	return 
}
