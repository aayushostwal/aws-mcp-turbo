package turbo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Prune removes absent data recursively, retaining false, zero and empty strings.
func Prune(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			if k != "ResultMetadata" {
				if p := Prune(v); p != nil {
					out[k] = p
				}
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, v := range x {
			if p := Prune(v); p != nil {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return v
	}
}

func validFormat(format string) error {
	switch format {
	case "", "markdown", "tsv", "json":
		return nil
	}
	return fmt.Errorf("format must be markdown, tsv, or json")
}

func Format(v any, format string, maxBytes int) (string, error) {
	if err := validFormat(format); err != nil {
		return "", err
	}
	v = Prune(v)
	if v == nil {
		return "No results.", nil
	}
	rows, isList := v.([]any)
	if !isList || format == "json" {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return bounded(string(b), maxBytes)
	}
	columns := map[string]bool{}
	objects := make([]map[string]any, len(rows))
	for i, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			obj = map[string]any{"Value": row}
		}
		objects[i] = obj
		for k := range obj {
			columns[k] = true
		}
	}
	keys := make([]string, 0, len(columns))
	for k := range columns {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	line := func(cells []string) {
		if format == "tsv" {
			out.WriteString(strings.Join(cells, "\t") + "\n")
		} else {
			out.WriteString("|" + strings.Join(cells, "|") + "|\n")
		}
	}
	header := make([]string, len(keys))
	for i, k := range keys {
		header[i] = escapeCell(k, format)
	}
	line(header)
	if format != "tsv" {
		separators := make([]string, len(keys))
		for i := range separators {
			separators[i] = "---"
		}
		line(separators)
	}
	for _, row := range objects {
		cells := make([]string, len(keys))
		for i, k := range keys {
			if val, ok := row[k]; ok {
				s, ok := val.(string)
				if !ok {
					b, _ := json.Marshal(val)
					s = string(b)
				}
				cells[i] = escapeCell(s, format)
			}
		}
		line(cells)
		if out.Len() > maxBytes {
			return "", outputLimit(maxBytes)
		}
	}
	return bounded(strings.TrimSuffix(out.String(), "\n"), maxBytes)
}

var cellEscaper = strings.NewReplacer("\\", "\\\\", "\r", "\\r", "\n", "\\n", "\t", "\\t")

func escapeCell(s, format string) string {
	s = cellEscaper.Replace(s)
	if format != "tsv" {
		s = strings.ReplaceAll(s, "|", "\\|")
	}
	return s
}

func outputLimit(n int) error {
	return fmt.Errorf("output exceeds %d bytes; narrow params/page size or projection (log cursor was not advanced)", n)
}
func bounded(s string, n int) (string, error) {
	if len(s) > n {
		return "", outputLimit(n)
	}
	return s, nil
}
