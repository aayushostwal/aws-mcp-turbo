package turbo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
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
	case "", "compact", "markdown", "tsv", "json":
		return nil
	}
	return fmt.Errorf("format must be compact, markdown, tsv, or json")
}

// Rendered is formatted output plus how many records it holds.
type Rendered struct {
	Text  string
	Shown int
	Total int
}

func (r Rendered) Truncated() bool { return r.Shown < r.Total }

// Format renders v and fails if the result does not fit.
func Format(v any, format string, maxBytes int) (string, error) {
	r, err := Render(v, format, maxBytes)
	if err != nil {
		return "", err
	}
	if r.Truncated() {
		return "", outputLimit(maxBytes)
	}
	return r.Text, nil
}

// Render formats v within budget bytes. Lists are cut to the rows that fit.
func Render(v any, format string, budget int) (Rendered, error) {
	return renderOrdered(v, format, budget, nil)
}

// renderOrdered is Render with columns in the given order, then the rest.
func renderOrdered(v any, format string, budget int, order []string) (Rendered, error) {
	if err := validFormat(format); err != nil {
		return Rendered{}, err
	}
	v = Prune(v)
	if v == nil {
		return Rendered{Text: "No results."}, nil
	}
	rows, isList := v.([]any)
	if obj, ok := v.(map[string]any); ok && (format == "" || format == "compact") {
		return renderObject(obj, budget)
	}
	if !isList || format == "json" {
		b, err := json.Marshal(v)
		if err != nil {
			return Rendered{}, err
		}
		if len(b) > budget {
			return Rendered{}, outputLimit(budget)
		}
		n := 1
		if isList {
			n = len(rows)
		}
		return Rendered{Text: string(b), Shown: n, Total: n}, nil
	}
	if format == "" || format == "compact" {
		return renderCompact(rows, budget, order)
	}
	return renderTable(rows, format, budget, order)
}

func objectsOf(rows []any, order []string) ([]map[string]any, []string) {
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
	position := map[string]int{}
	for i, k := range order {
		position[k] = i + 1
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, pj := position[keys[i]], position[keys[j]]
		if (pi != 0) != (pj != 0) {
			return pi != 0
		}
		if pi != pj {
			return pi < pj
		}
		ri, rj := columnRank(keys[i]), columnRank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	return objects, keys
}

// columnRank puts identifying columns first.
func columnRank(k string) int {
	switch k {
	case "Id", "Name":
		return 0
	case "Arn":
		return 1
	case "Count":
		return 3
	}
	return 2
}

func cellText(val any) string {
	switch x := val.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	b, _ := json.Marshal(val)
	return string(b)
}

func renderTable(rows []any, format string, budget int, order []string) (Rendered, error) {
	objects, keys := objectsOf(rows, order)
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
	shown := 0
	for _, row := range objects {
		before := out.Len()
		cells := make([]string, len(keys))
		for i, k := range keys {
			if val, ok := row[k]; ok {
				cells[i] = escapeCell(cellText(val), format)
			}
		}
		line(cells)
		if out.Len() > budget {
			s := out.String()[:before]
			if shown == 0 {
				return Rendered{}, outputLimit(budget)
			}
			return Rendered{Text: strings.TrimSuffix(s, "\n"), Shown: shown, Total: len(objects)}, nil
		}
		shown++
	}
	return Rendered{Text: strings.TrimSuffix(out.String(), "\n"), Shown: shown, Total: shown}, nil
}

var cellEscaper = strings.NewReplacer("\\", "\\\\", "\r", "\\r", "\n", "\\n", "\t", "\\t")

func escapeCell(s, format string) string {
	s = cellEscaper.Replace(s)
	if format != "tsv" {
		s = strings.ReplaceAll(s, "|", "\\|")
	}
	return s
}

var isoTime = regexp.MustCompile(`^(\d{4}-\d\d-\d\d)T(\d\d:\d\d):\d\d(?:\.\d+)?(?:Z|[+-]00:?00)$`)

// shortTime turns 2026-10-17T09:41:12.000Z into "2026-10-17 09:41Z".
func shortTime(s string) string {
	if m := isoTime.FindStringSubmatch(s); m != nil {
		return m[1] + " " + m[2] + "Z"
	}
	return s
}

// commonPrefix returns the longest shared prefix that ends at ':' or '/'.
func commonPrefix(values []string) string {
	if len(values) < 3 {
		return ""
	}
	p := values[0]
	for _, v := range values[1:] {
		for !strings.HasPrefix(v, p) {
			p = p[:len(p)-1]
			if p == "" {
				return ""
			}
		}
	}
	cut := strings.LastIndexAny(p, ":/")
	if cut < 7 {
		return ""
	}
	p = p[:cut+1]
	for _, v := range values {
		if len(v) == len(p) {
			return ""
		}
	}
	return p
}

// renderCompact is the default format. Values every row shares are printed once,
// shared ARN/path prefixes are stated once, timestamps lose seconds, and rows
// are tab separated with no table decoration.
func renderCompact(rows []any, budget int, order []string) (Rendered, error) {
	objects, keys := objectsOf(rows, order)
	n := len(objects)
	text := make([][]string, n) // [row][col]
	for r, obj := range objects {
		text[r] = make([]string, len(keys))
		for c, k := range keys {
			s := cellText(obj[k])
			text[r][c] = shortTime(s)
		}
	}
	var constants, prefixes []string
	var cols []int
	for c, k := range keys {
		values := make([]string, n)
		same, allStrings := true, true
		for r := range objects {
			values[r] = text[r][c]
			if values[r] != values[0] {
				same = false
			}
			if _, ok := objects[r][k].(string); !ok {
				allStrings = false
			}
		}
		if n >= 3 && same && values[0] != "" && !strings.ContainsAny(values[0], "{[") {
			constants = append(constants, k+"="+escapeCell(values[0], "tsv"))
			continue
		}
		if allStrings {
			if p := commonPrefix(values); p != "" {
				prefixes = append(prefixes, k+"="+p)
				for r := range text {
					text[r][c] = text[r][c][len(p):]
				}
			}
		}
		cols = append(cols, c)
	}
	var head strings.Builder
	if len(constants) > 0 {
		head.WriteString("all: " + strings.Join(constants, " ") + "\n")
	}
	if len(prefixes) > 0 {
		head.WriteString("prefix: " + strings.Join(prefixes, " ") + "\n")
	}
	if len(cols) == 0 {
		head.WriteString(fmt.Sprintf("rows=%d", n))
		return Rendered{Text: head.String(), Shown: n, Total: n}, nil
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = escapeCell(keys[c], "tsv")
	}
	var out strings.Builder
	out.WriteString(head.String())
	out.WriteString(strings.Join(names, "\t"))
	shown := 0
	for r := 0; r < n; r++ {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = escapeCell(text[r][c], "tsv")
		}
		row := "\n" + strings.Join(cells, "\t")
		if out.Len()+len(row) > budget {
			if shown == 0 {
				return Rendered{}, outputLimit(budget)
			}
			return Rendered{Text: out.String(), Shown: shown, Total: n}, nil
		}
		out.WriteString(row)
		shown++
	}
	return Rendered{Text: out.String(), Shown: shown, Total: n}, nil
}

func outputLimit(n int) error {
	return fmt.Errorf("output exceeds %d bytes; narrow params/page size, use fields, or raise --max-output-bytes (log cursor was not advanced)", n)
}

func bounded(s string, n int) (string, error) {
	if len(s) > n {
		return "", outputLimit(n)
	}
	return s, nil
}

// renderObject prints one record as key=value lines; nested values stay JSON.
func renderObject(obj map[string]any, budget int) (Rendered, error) {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if ri, rj := columnRank(keys[i]), columnRank(keys[j]); ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	var out strings.Builder
	for i, k := range keys {
		if i > 0 {
			out.WriteByte('\n')
		}
		s := shortTime(cellText(obj[k]))
		out.WriteString(k + "=" + strings.NewReplacer("\n", "\\n", "\r", "\\r").Replace(s))
	}
	if out.Len() > budget {
		return Rendered{}, outputLimit(budget)
	}
	return Rendered{Text: out.String(), Shown: 1, Total: 1}, nil
}
