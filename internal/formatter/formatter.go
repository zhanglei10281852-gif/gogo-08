package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	csvpkg "csvsql/internal/csv"
	"csvsql/internal/executor"
	"csvsql/internal/types"
)

type Format string

const (
	FormatTable Format = "table"
	FormatCSV   Format = "csv"
	FormatJSON  Format = "json"
)

func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "table", "":
		return FormatTable, nil
	case "csv":
		return FormatCSV, nil
	case "json":
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unknown format: %s (supported: table, csv, json)", s)
	}
}

func Write(w io.Writer, result *executor.QueryResult, format Format) error {
	switch format {
	case FormatTable:
		return writeTable(w, result)
	case FormatCSV:
		return writeCSV(w, result)
	case FormatJSON:
		return writeJSON(w, result)
	default:
		return writeTable(w, result)
	}
}

func writeTable(w io.Writer, result *executor.QueryResult) error {
	if len(result.Columns) == 0 {
		_, err := fmt.Fprintln(w, "(no columns)")
		return err
	}
	ncols := len(result.Columns)
	widths := make([]int, ncols)
	for i, c := range result.Columns {
		widths[i] = displayWidth(c)
	}
	for _, row := range result.Rows {
		for i, v := range row {
			w := displayWidth(v.AsText())
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("+")
	for i := 0; i < ncols; i++ {
		sb.WriteString(strings.Repeat("-", widths[i]+2))
		if i < ncols-1 {
			sb.WriteString("+")
		} else {
			sb.WriteString("+")
		}
	}
	sb.WriteString("\n")

	sb.WriteString("|")
	for i, c := range result.Columns {
		sb.WriteString(" ")
		sb.WriteString(padRight(c, widths[i]))
		sb.WriteString(" |")
	}
	sb.WriteString("\n")

	sb.WriteString("+")
	for i := 0; i < ncols; i++ {
		sb.WriteString(strings.Repeat("-", widths[i]+2))
		if i < ncols-1 {
			sb.WriteString("+")
		} else {
			sb.WriteString("+")
		}
	}
	sb.WriteString("\n")

	for _, row := range result.Rows {
		sb.WriteString("|")
		for i, v := range row {
			sb.WriteString(" ")
			s := v.AsText()
			if v.Type == types.TypeInt || v.Type == types.TypeFloat {
				sb.WriteString(padLeft(s, widths[i]))
			} else {
				sb.WriteString(padRight(s, widths[i]))
			}
			sb.WriteString(" |")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("+")
	for i := 0; i < ncols; i++ {
		sb.WriteString(strings.Repeat("-", widths[i]+2))
		if i < ncols-1 {
			sb.WriteString("+")
		} else {
			sb.WriteString("+")
		}
	}
	sb.WriteString("\n")

	if len(result.Rows) == 1 {
		sb.WriteString("(1 row)\n")
	} else {
		fmt.Fprintf(&sb, "(%d rows)\n", len(result.Rows))
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}

func displayWidth(s string) int {
	runes := []rune(s)
	w := 0
	for _, r := range runes {
		if r > 127 {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func padRight(s string, w int) string {
	dw := displayWidth(s)
	if dw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-dw)
}

func padLeft(s string, w int) string {
	dw := displayWidth(s)
	if dw >= w {
		return s
	}
	return strings.Repeat(" ", w-dw) + s
}

func writeCSV(w io.Writer, result *executor.QueryResult) error {
	return csvpkg.Write(w, result.Columns, result.Rows, ',')
}

func writeJSON(w io.Writer, result *executor.QueryResult) error {
	var arr []map[string]interface{}
	for _, row := range result.Rows {
		m := make(map[string]interface{})
		for i, c := range result.Columns {
			v := row[i]
			switch v.Type {
			case types.TypeNull:
				m[c] = nil
			case types.TypeInt:
				m[c] = v.V.(int64)
			case types.TypeFloat:
				m[c] = v.V.(float64)
			case types.TypeBool:
				m[c] = v.V.(bool)
			default:
				m[c] = v.AsText()
			}
		}
		arr = append(arr, m)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(arr)
}
