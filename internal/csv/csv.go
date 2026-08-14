package csvpkg

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"csvsql/internal/types"
)

type Column struct {
	Name string
	Type types.ValueType
}

type Table struct {
	Name    string
	Columns []Column
	Rows    [][]types.Value
}

type ReadOptions struct {
	Delimiter rune
	HasHeader bool
	Encoding  string
}

func DefaultOptions() ReadOptions {
	return ReadOptions{
		Delimiter: ',',
		HasHeader: true,
		Encoding:  "utf-8",
	}
}

func ReadFile(path string, tableName string, opts *ReadOptions) (*Table, error) {
	if opts == nil {
		o := DefaultOptions()
		opts = &o
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %s: %w", path, err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	r := csv.NewReader(br)
	r.Comma = opts.Delimiter
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	allRecords, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error parsing CSV %s: %w", path, err)
	}

	if len(allRecords) == 0 {
		return &Table{Name: tableName, Columns: nil, Rows: nil}, nil
	}

	var headers []string
	var dataStart int
	if opts.HasHeader {
		headers = allRecords[0]
		for i := range headers {
			headers[i] = strings.TrimSpace(headers[i])
		}
		dataStart = 1
	} else {
		ncols := len(allRecords[0])
		headers = make([]string, ncols)
		for i := 0; i < ncols; i++ {
			headers[i] = fmt.Sprintf("col%d", i+1)
		}
		dataStart = 0
	}

	ncols := len(headers)
	records := allRecords[dataStart:]

	rawValues := make([][]string, len(records))
	for i, rec := range records {
		row := make([]string, ncols)
		for j := 0; j < ncols; j++ {
			if j < len(rec) {
				row[j] = rec[j]
			} else {
				row[j] = ""
			}
		}
		rawValues[i] = row
	}

	colTypes := inferColumnTypes(rawValues, ncols)
	columns := make([]Column, ncols)
	for i, h := range headers {
		columns[i] = Column{Name: h, Type: colTypes[i]}
	}

	rows := make([][]types.Value, len(rawValues))
	for i, rawRow := range rawValues {
		row := make([]types.Value, ncols)
		for j := 0; j < ncols; j++ {
			row[j] = parseAsType(rawRow[j], colTypes[j])
		}
		rows[i] = row
	}

	return &Table{
		Name:    tableName,
		Columns: columns,
		Rows:    rows,
	}, nil
}

func inferColumnTypes(records [][]string, ncols int) []types.ValueType {
	colTypes := make([]types.ValueType, ncols)
	for j := 0; j < ncols; j++ {
		colTypes[j] = types.TypeNull
	}
	for _, rec := range records {
		for j := 0; j < ncols; j++ {
			s := strings.TrimSpace(rec[j])
			if s == "" {
				continue
			}
			v := types.InferValueType(s)
			if v.Type == types.TypeNull {
				continue
			}
			colTypes[j] = types.PromoteTypes(colTypes[j], v.Type)
		}
	}
	for j := 0; j < ncols; j++ {
		if colTypes[j] == types.TypeNull {
			colTypes[j] = types.TypeText
		}
	}
	return colTypes
}

func parseAsType(s string, t types.ValueType) types.Value {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "NULL") {
		return types.NullValue()
	}
	switch t {
	case types.TypeInt:
		if i, err := parseInt(s); err == nil {
			return types.IntValue(i)
		}
		if f, ok := types.TextValue(s).AsNumber(); ok {
			return types.IntValue(int64(f))
		}
		return types.NullValue()
	case types.TypeFloat:
		if f, ok := types.TextValue(s).AsNumber(); ok {
			return types.FloatValue(f)
		}
		return types.NullValue()
	case types.TypeBool:
		if b, err := parseBool(s); err == nil {
			return types.BoolValue(b)
		}
		return types.NullValue()
	case types.TypeDate:
		if t2, ok := parseDate(s); ok {
			return types.DateValue(t2)
		}
		return types.NullValue()
	case types.TypeDateTime:
		if t2, ok := parseDateTime(s); ok {
			return types.DateTimeValue(t2)
		}
		return types.NullValue()
	default:
		return types.TextValue(s)
	}
}

func parseInt(s string) (int64, error) {
	if strings.Contains(s, ".") {
		return 0, fmt.Errorf("not int")
	}
	return parseIntSimple(s)
}

func parseIntSimple(s string) (int64, error) {
	var (
		n     int64
		neg   bool
		start int
	)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty")
	}
	if s[0] == '-' {
		neg = true
		start = 1
	} else if s[0] == '+' {
		start = 1
	}
	for i := start; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("bad digit: %c", c)
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "t", "1", "yes", "y":
		return true, nil
	case "false", "f", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("not bool")
	}
}

func parseDate(s string) (time.Time, bool) {
	layouts := []string{"2006-01-02", "2006/01/02", "01/02/2006", "02-01-2006"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseDateTime(s string) (time.Time, bool) {
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006/01/02 15:04:05"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return parseDate(s)
}

func (t *Table) ColumnIndex(name string) (int, error) {
	lower := strings.ToLower(name)
	for i, c := range t.Columns {
		if strings.ToLower(c.Name) == lower {
			return i, nil
		}
	}
	return -1, fmt.Errorf("column %q not found in table %q", name, t.Name)
}

func (t *Table) ColumnByTableAndName(tableName, colName string) (int, error) {
	if tableName == "" || strings.EqualFold(tableName, t.Name) {
		return t.ColumnIndex(colName)
	}
	return -1, fmt.Errorf("table %q not found", tableName)
}

func Write(w io.Writer, columns []string, rows [][]types.Value, delimiter rune) error {
	cw := csv.NewWriter(w)
	cw.Comma = delimiter
	header := make([]string, len(columns))
	copy(header, columns)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(row))
		for i, v := range row {
			rec[i] = v.AsText()
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
