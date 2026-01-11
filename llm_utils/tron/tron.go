package tron

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Schema describes ordered fields per table/type.
type Schema map[string][]string

// TRONTable holds columnar, CSV-friendly data (strings repeat; no pooling).
type TRONTable struct {
	Name       string          `json:"name"`
	Fields     []string        `json:"fields"`
	Rows       [][]interface{} `json:"rows"`
	fieldIndex map[string]int  `json:"-"`
	dedupe     map[string]int  `json:"-"`
}

// TRONDocument is the final LOEF-like payload: multiple tables keyed by name.
type TRONDocument struct {
	Root   string                `json:"root"`
	Tables map[string]*TRONTable `json:"tables"`
}

func BuildTRONDocuments(rootName string, payload []interface{}) (*TRONDocument, error) {
	return BuildTRONDocument(rootName, map[string]interface{}{rootName: payload})
}

// BuildTRONDocument takes a JSON payload (already unmarshaled) and flattens it into tables.
// Nested objects become separate tables; parents reference children via "<field>_id" (1-based).
func BuildTRONDocument(rootName string, payload map[string]interface{}) (*TRONDocument, error) {
	rootVal, ok := payload[rootName]
	if !ok {
		return nil, fmt.Errorf("root key %q not found", rootName)
	}
	slice, ok := toSlice(rootVal)
	if !ok {
		return nil, fmt.Errorf("root %q must be an array", rootName)
	}

	doc := &TRONDocument{
		Root:   rootName,
		Tables: make(map[string]*TRONTable),
	}

	var encodeRow func(table string, row map[string]interface{}) (int, error)

	getTable := func(name string) *TRONTable {
		if tbl, ok := doc.Tables[name]; ok {
			return tbl
		}
		tbl := &TRONTable{
			Name:       name,
			Fields:     []string{},
			Rows:       [][]interface{}{},
			fieldIndex: make(map[string]int),
			dedupe:     make(map[string]int),
		}
		doc.Tables[name] = tbl
		return tbl
	}

	addField := func(tbl *TRONTable, field string) {
		if _, ok := tbl.fieldIndex[field]; ok {
			return
		}
		tbl.fieldIndex[field] = len(tbl.Fields)
		tbl.Fields = append(tbl.Fields, field)
		// Extend existing rows with nil for the new column.
		for i := range tbl.Rows {
			tbl.Rows[i] = append(tbl.Rows[i], nil)
		}
	}

	encodeRow = func(table string, row map[string]interface{}) (int, error) {
		tbl := getTable(table)

		// Ensure fields exist and track nested rows.
		type nestedRef struct {
			field string
			id    interface{}
		}
		var nests []nestedRef

		// deterministic field ordering: scalars first (sorted), then nested (sorted)
		var scalarKeys, nestedKeys []string
		for k, v := range row {
			if _, ok := v.(map[string]interface{}); ok {
				nestedKeys = append(nestedKeys, k)
				continue
			}
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				if _, isMap := arr[0].(map[string]interface{}); isMap {
					nestedKeys = append(nestedKeys, k)
					continue
				}
			}
			scalarKeys = append(scalarKeys, k)
		}
		// deterministic order with a slight preference: "name" first if present, then alpha.
		sort.Strings(scalarKeys)
		sort.Strings(nestedKeys)

		for _, k := range scalarKeys {
			addField(tbl, k)
		}
		for _, k := range nestedKeys {
			v := row[k]
			switch child := v.(type) {
			case map[string]interface{}:
				childTable := table + "." + k
				id, err := encodeRow(childTable, child)
				if err != nil {
					return 0, err
				}
				addField(tbl, k+"_id")
				nests = append(nests, nestedRef{field: k + "_id", id: id})
			case []interface{}:
				if len(child) > 0 {
					if _, ok := child[0].(map[string]interface{}); ok {
						childTable := table + "." + k
						ids := make([]int, 0, len(child))
						for _, elem := range child {
							mm, _ := elem.(map[string]interface{})
							id, err := encodeRow(childTable, mm)
							if err != nil {
								return 0, err
							}
							ids = append(ids, id)
						}
						addField(tbl, k+"_id")
						nests = append(nests, nestedRef{field: k + "_id", id: idsToInterface(ids)})
						continue
					}
				}
				addField(tbl, k)
			}
		}

		rowVals := make([]interface{}, len(tbl.Fields))
		for k, v := range row {
			if _, ok := v.(map[string]interface{}); ok {
				continue
			}
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				if _, isMap := arr[0].(map[string]interface{}); isMap {
					continue
				}
			}
			idx := tbl.fieldIndex[k]
			rowVals[idx] = v
		}
		for _, n := range nests {
			idx := tbl.fieldIndex[n.field]
			rowVals[idx] = n.id
		}

		// Dedupe identical rows; ids are 1-based.
		keyBytes, _ := json.Marshal(rowVals)
		key := string(keyBytes)
		if id, ok := tbl.dedupe[key]; ok {
			return id, nil
		}
		tbl.Rows = append(tbl.Rows, rowVals)
		id := len(tbl.Rows)
		tbl.dedupe[key] = id
		return id, nil
	}

	for _, item := range slice {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("root array items must be objects")
		}
		if _, err := encodeRow(rootName, m); err != nil {
			return nil, err
		}
	}

	return doc, nil
}

// RenderTRONDocument renders the TRONDocument into a human-friendly CSV-like text (root first, then other tables).
func RenderTRONDocument(doc *TRONDocument) string {
	var b bytes.Buffer

	names := make([]string, 0, len(doc.Tables))
	for n := range doc.Tables {
		names = append(names, n)
	}
	sort.Strings(names)

	// root first
	writeTable(&b, doc.Tables[doc.Root])
	for _, n := range names {
		if n == doc.Root {
			continue
		}
		b.WriteString("---")
		b.WriteString(stripPrefix(n, doc.Root+"."))
		b.WriteString("\n")
		writeTable(&b, doc.Tables[n])
	}
	return b.String()
}

func writeTable(b *bytes.Buffer, tbl *TRONTable) {
	if tbl == nil {
		return
	}
	b.WriteString(strings.Join(tbl.Fields, ","))
	b.WriteString("\n")
	for _, row := range tbl.Rows {
		cells := make([]string, len(tbl.Fields))
		for i, v := range row {
			cells[i] = fmt.Sprintf("%v", v)
		}
		b.WriteString(strings.Join(cells, ","))
		b.WriteString("\n")
	}
}

func stripPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func idsToInterface(ids []int) interface{} {
	out := make([]interface{}, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func toSlice(v interface{}) ([]interface{}, bool) {
	if arr, ok := v.([]interface{}); ok {
		return arr, true
	}
	return nil, false
}
