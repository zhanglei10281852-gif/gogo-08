package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	csvpkg "csvsql/internal/csv"
	"csvsql/internal/executor"
	"csvsql/internal/formatter"
	"csvsql/internal/lexer"
	"csvsql/internal/parser"
)

type csvFile struct {
	path  string
	names []string
}

func main() {
	fs := flag.NewFlagSet("csvsql", flag.ContinueOnError)
	format := fs.String("format", "table", "output format: table, csv, json")
	delimiter := fs.String("delimiter", ",", "CSV field delimiter (single char)")
	noHeader := fs.Bool("no-header", false, "CSV files have no header row")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `csvsql - run SQL queries on CSV files

Usage:
  csvsql [flags] "SELECT ..." file1.csv [[name=]file2.csv ...]

Flags:
  --format string     output format: table, csv, json (default "table")
  --delimiter string  CSV field delimiter (default ",")
  --no-header         CSV files have no header row (use col1, col2...)

Examples:
  csvsql "SELECT name, age FROM users WHERE age > 30" users.csv
  csvsql "SELECT u.name, o.amount FROM users u JOIN orders o ON u.id = o.user_id" users.csv orders.csv
  csvsql --format json "SELECT COUNT(*) AS n FROM data" data.csv
`)
	}

	if len(os.Args) < 2 {
		fs.Usage()
		os.Exit(1)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	args := fs.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: SQL statement required")
		os.Exit(1)
	}

	sql := args[0]
	files := args[1:]
	if len(files) == 0 {
		files = findCSVFiles()
	}
	if len(files) == 0 && !isConstantQuery(sql) {
		fmt.Fprintln(os.Stderr, "error: no CSV files specified (put CSV files in current dir or pass as args)")
		os.Exit(1)
	}

	outFmt, err := formatter.ParseFormat(*format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var delim rune
	if len(*delimiter) > 0 {
		runes := []rune(*delimiter)
		delim = runes[0]
	} else {
		delim = ','
	}

	var csvFiles []csvFile
	for _, f := range files {
		cf, err := parseFileArg(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		csvFiles = append(csvFiles, cf)
	}

	opts := csvpkg.DefaultOptions()
	opts.Delimiter = delim
	opts.HasHeader = !*noHeader

	var tables []*csvpkg.Table
	for _, cf := range csvFiles {
		var baseTable *csvpkg.Table
		for i, name := range cf.names {
			var tbl *csvpkg.Table
			if i == 0 {
				t, err := csvpkg.ReadFile(cf.path, name, &opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error reading %s: %v\n", cf.path, err)
					os.Exit(1)
				}
				baseTable = t
				tbl = t
			} else {
				tbl = &csvpkg.Table{
					Name:    name,
					Columns: baseTable.Columns,
					Rows:    baseTable.Rows,
				}
			}
			tables = append(tables, tbl)
		}
	}

	lex := lexer.New(sql)
	toks, err := lex.AllTokens()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lex error: %v\n", err)
		os.Exit(1)
	}

	par := parser.New(toks)
	stmt, err := par.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	exec := executor.New(tables)
	result, err := exec.Execute(stmt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execution error: %v\n", err)
		os.Exit(1)
	}

	if err := formatter.Write(os.Stdout, result, outFmt); err != nil {
		fmt.Fprintf(os.Stderr, "output error: %v\n", err)
		os.Exit(1)
	}
}

func parseFileArg(s string) (csvFile, error) {
	var names []string
	var path string
	if idx := strings.Index(s, "="); idx >= 0 {
		names = append(names, s[:idx])
		path = s[idx+1:]
	} else {
		path = s
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	baseName := strings.TrimSuffix(base, ext)
	found := false
	for _, n := range names {
		if n == baseName {
			found = true
			break
		}
	}
	if !found {
		names = append(names, baseName)
	}
	if _, err := os.Stat(path); err != nil {
		return csvFile{}, fmt.Errorf("cannot access %s: %w", path, err)
	}
	return csvFile{path: path, names: names}, nil
}

func findCSVFiles() []string {
	var files []string
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".csv") {
			files = append(files, name)
		}
	}
	return files
}

func isConstantQuery(sql string) bool {
	s := strings.ToLower(strings.TrimSpace(sql))
	return strings.HasPrefix(s, "select") && !strings.Contains(s, " from ")
}
