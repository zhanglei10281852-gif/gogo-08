package executor_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDistinctOrderByKeepsSourceRows(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		columns []string
		rows    [][]string
	}{
		{
			name:    "adjacent and non-adjacent duplicates ordered by projected column ascending",
			sql:     `SELECT DISTINCT label FROM distinct_order ORDER BY label ASC`,
			columns: []string{"label"},
			rows:    [][]string{{"alpha"}, {"bravo"}, {"charlie"}, {"delta"}},
		},
		{
			name:    "projected column descending",
			sql:     `SELECT DISTINCT label FROM distinct_order ORDER BY label DESC`,
			columns: []string{"label"},
			rows:    [][]string{{"delta"}, {"charlie"}, {"bravo"}, {"alpha"}},
		},
		{
			name:    "unprojected column ascending",
			sql:     `SELECT DISTINCT label FROM distinct_order ORDER BY sort_key ASC`,
			columns: []string{"label"},
			rows:    [][]string{{"delta"}, {"alpha"}, {"charlie"}, {"bravo"}},
		},
		{
			name:    "unprojected column descending",
			sql:     `SELECT DISTINCT label FROM distinct_order ORDER BY sort_key DESC`,
			columns: []string{"label"},
			rows:    [][]string{{"bravo"}, {"charlie"}, {"alpha"}, {"delta"}},
		},
		{
			name:    "order before limit",
			sql:     `SELECT DISTINCT label FROM distinct_order ORDER BY sort_key ASC LIMIT 2`,
			columns: []string{"label"},
			rows:    [][]string{{"delta"}, {"alpha"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runSQL(t, tt.sql, "distinct_order.csv")
			actual := make([][]string, len(result.Rows))
			for i, row := range result.Rows {
				actual[i] = make([]string, len(row))
				for j, value := range row {
					actual[i][j] = value.AsText()
				}
			}
			require.Equal(t, tt.columns, result.Columns)
			require.Equal(t, tt.rows, actual)
		})
	}
}
