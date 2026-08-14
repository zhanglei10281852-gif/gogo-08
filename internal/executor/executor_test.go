package executor_test

import (
	"path/filepath"
	"strings"
	"testing"

	csvpkg "csvsql/internal/csv"
	"csvsql/internal/executor"
	"csvsql/internal/lexer"
	"csvsql/internal/parser"
	"csvsql/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdata(p string) string {
	return filepath.Join("..", "..", "testdata", p)
}

func runSQL(t *testing.T, sql string, csvArgs ...string) *executor.QueryResult {
	t.Helper()
	var tables []*csvpkg.Table
	opts := csvpkg.DefaultOptions()
	for _, arg := range csvArgs {
		var aliasName string
		var path string
		if idx := strings.Index(arg, "="); idx >= 0 {
			aliasName = arg[:idx]
			path = arg[idx+1:]
		} else {
			path = arg
		}
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		baseName := strings.TrimSuffix(base, ext)
		fullPath := testdata(base)

		var baseTbl *csvpkg.Table
		added := make(map[string]bool)
		addTable := func(name string) {
			if added[name] {
				return
			}
			added[name] = true
			var tbl *csvpkg.Table
			if baseTbl == nil {
				var err error
				tbl, err = csvpkg.ReadFile(fullPath, name, &opts)
				require.NoError(t, err)
				baseTbl = tbl
			} else {
				tbl = &csvpkg.Table{
					Name:    name,
					Columns: baseTbl.Columns,
					Rows:    baseTbl.Rows,
				}
			}
			tables = append(tables, tbl)
		}
		if aliasName != "" {
			addTable(aliasName)
		}
		addTable(baseName)
	}

	l := lexer.New(sql)
	toks, err := l.AllTokens()
	require.NoError(t, err)
	p := parser.New(toks)
	stmt, err := p.Parse()
	require.NoError(t, err)

	ex := executor.New(tables)
	res, err := ex.Execute(stmt)
	require.NoError(t, err)
	return res
}

func assertInt(t *testing.T, v types.Value, expected int64) {
	t.Helper()
	f, ok := v.AsNumber()
	require.True(t, ok, "expected number, got %s", v.Type)
	assert.Equal(t, float64(expected), f)
}

func assertFloat(t *testing.T, v types.Value, expected float64) {
	t.Helper()
	f, ok := v.AsNumber()
	require.True(t, ok, "expected number, got %s", v.Type)
	assert.InDelta(t, expected, f, 0.01)
}

func assertText(t *testing.T, v types.Value, expected string) {
	t.Helper()
	assert.Equal(t, expected, v.AsText())
}

func colIndex(t *testing.T, result *executor.QueryResult, name string) int {
	t.Helper()
	for i, c := range result.Columns {
		if c == name {
			return i
		}
	}
	t.Fatalf("column %q not found in %v", name, result.Columns)
	return -1
}

func TestSelectBasicColumns(t *testing.T) {
	res := runSQL(t, "SELECT id, name, age FROM employees", "employees.csv")
	require.Equal(t, 3, len(res.Columns))
	require.Equal(t, 12, len(res.Rows))
	assertText(t, res.Rows[0][colIndex(t, res, "name")], "张三")
	assertInt(t, res.Rows[0][colIndex(t, res, "age")], 28)
}

func TestSelectWhereComparison(t *testing.T) {
	res := runSQL(t, "SELECT name, age FROM employees WHERE age > 30", "employees.csv")
	assert.Equal(t, 7, len(res.Rows))
	names := map[string]bool{}
	for _, r := range res.Rows {
		names[r[0].AsText()] = true
	}
	assert.True(t, names["李四"])
	assert.True(t, names["赵六"])
	assert.True(t, names["孙七"])
}

func TestSelectWhereAndOr(t *testing.T) {
	res := runSQL(t, `SELECT name FROM employees WHERE city = '北京' AND age > 30`, "employees.csv")
	require.Equal(t, 1, len(res.Rows))
	assertText(t, res.Rows[0][0], "刘十一")

	res2 := runSQL(t, `SELECT name FROM employees WHERE city = '北京' OR city = '上海'`, "employees.csv")
	assert.Equal(t, 4, len(res2.Rows))
}

func TestSelectWhereNot(t *testing.T) {
	res := runSQL(t, `SELECT name, city FROM employees WHERE NOT city = '北京'`, "employees.csv")
	for _, r := range res.Rows {
		assert.NotEqual(t, "北京", r[1].AsText())
	}
}

func TestSelectWhereBetween(t *testing.T) {
	res := runSQL(t, `SELECT name, age FROM employees WHERE age BETWEEN 25 AND 30`, "employees.csv")
	assert.Equal(t, 4, len(res.Rows))
}

func TestSelectWhereIn(t *testing.T) {
	res := runSQL(t, `SELECT name, city FROM employees WHERE city IN ('北京', '上海', '深圳')`, "employees.csv")
	assert.Equal(t, 6, len(res.Rows))
	for _, r := range res.Rows {
		city := r[1].AsText()
		assert.True(t, city == "北京" || city == "上海" || city == "深圳")
	}
}

func TestSelectWhereLike(t *testing.T) {
	res := runSQL(t, `SELECT name FROM employees WHERE name LIKE '张%'`, "employees.csv")
	require.Equal(t, 1, len(res.Rows))
	assertText(t, res.Rows[0][0], "张三")

	res2 := runSQL(t, `SELECT name FROM employees WHERE name LIKE '_六'`, "employees.csv")
	require.Equal(t, 1, len(res2.Rows))
	assertText(t, res2.Rows[0][0], "赵六")
}

func TestSelectWhereIsNull(t *testing.T) {
	res := runSQL(t, `SELECT dept_name, manager_id FROM departments WHERE manager_id IS NULL`, "departments.csv")
	assert.Equal(t, 1, len(res.Rows))
	assertText(t, res.Rows[0][0], "人力资源部")
}

func TestSelectArithmetic(t *testing.T) {
	res := runSQL(t, `SELECT name, salary, salary * 1.1 AS new_salary FROM employees WHERE id = 1`, "employees.csv")
	require.Equal(t, 1, len(res.Rows))
	idx := colIndex(t, res, "new_salary")
	assertFloat(t, res.Rows[0][idx], 16500.55)
}

func TestGroupByCount(t *testing.T) {
	res := runSQL(t, `SELECT city, COUNT(*) AS n FROM employees GROUP BY city`, "employees.csv")
	cityCounts := map[string]int64{}
	for _, r := range res.Rows {
		f, _ := r[1].AsNumber()
		cityCounts[r[0].AsText()] = int64(f)
	}
	assert.Equal(t, int64(2), cityCounts["北京"])
	assert.Equal(t, int64(2), cityCounts["上海"])
	assert.Equal(t, int64(2), cityCounts["广州"])
	assert.Equal(t, int64(2), cityCounts["深圳"])
}

func TestGroupByAggregateFunctions(t *testing.T) {
	res := runSQL(t, `SELECT city, COUNT(*) AS cnt, SUM(salary) AS total, AVG(salary) AS avg, MIN(age) AS min_age, MAX(age) AS max_age FROM employees GROUP BY city HAVING COUNT(*) >= 2`, "employees.csv")
	for _, r := range res.Rows {
		city := r[0].AsText()
		t.Logf("%s: cnt=%v total=%v avg=%v min=%v max=%v",
			city, r[1].AsText(), r[2].AsText(), r[3].AsText(), r[4].AsText(), r[5].AsText())
	}
	assert.GreaterOrEqual(t, len(res.Rows), 4)
}

func TestOrderByAsc(t *testing.T) {
	res := runSQL(t, `SELECT name, age FROM employees ORDER BY age ASC`, "employees.csv")
	require.Equal(t, 12, len(res.Rows))
	for i := 1; i < len(res.Rows); i++ {
		prev, _ := res.Rows[i-1][1].AsNumber()
		cur, _ := res.Rows[i][1].AsNumber()
		assert.LessOrEqual(t, prev, cur)
	}
}

func TestOrderByDesc(t *testing.T) {
	res := runSQL(t, `SELECT name, salary FROM employees ORDER BY salary DESC`, "employees.csv")
	require.Equal(t, 12, len(res.Rows))
	for i := 1; i < len(res.Rows); i++ {
		prev, _ := res.Rows[i-1][1].AsNumber()
		cur, _ := res.Rows[i][1].AsNumber()
		assert.GreaterOrEqual(t, prev, cur)
	}
}

func TestLimitOffset(t *testing.T) {
	res := runSQL(t, `SELECT name FROM employees ORDER BY id LIMIT 3`, "employees.csv")
	assert.Equal(t, 3, len(res.Rows))
	assertText(t, res.Rows[0][0], "张三")
	assertText(t, res.Rows[1][0], "李四")
	assertText(t, res.Rows[2][0], "王五")

	res2 := runSQL(t, `SELECT name FROM employees ORDER BY id LIMIT 3 OFFSET 3`, "employees.csv")
	assert.Equal(t, 3, len(res2.Rows))
	assertText(t, res2.Rows[0][0], "赵六")
}

func TestInnerJoin(t *testing.T) {
	res := runSQL(t, `SELECT e.name, COUNT(o.order_id) AS order_cnt, SUM(o.amount) AS total
FROM employees e
INNER JOIN orders o ON e.id = o.user_id
GROUP BY e.id, e.name
ORDER BY COUNT(o.order_id) DESC`, "e=employees.csv", "o=orders.csv")
	assert.GreaterOrEqual(t, len(res.Rows), 1)
	// 张三(id=1) 有3个订单
	var zsTotal float64
	var zsCntFloat float64
	for _, r := range res.Rows {
		if r[0].AsText() == "张三" {
			zsCntFloat, _ = r[1].AsNumber()
			zsTotal, _ = r[2].AsNumber()
		}
	}
	assert.Equal(t, int64(3), int64(zsCntFloat))
	assert.InDelta(t, 5999+1899+459, zsTotal, 0.5)
}

func TestLeftJoin(t *testing.T) {
	res := runSQL(t, `SELECT e.name, COUNT(o.order_id) AS cnt
FROM employees e
LEFT JOIN orders o ON e.id = o.user_id
GROUP BY e.id, e.name`, "e=employees.csv", "o=orders.csv")
	assert.Equal(t, 12, len(res.Rows))
	// 员工没有订单的 cnt 应该是 0
	hasZero := false
	for _, r := range res.Rows {
		f, _ := r[1].AsNumber()
		if int64(f) == 0 {
			hasZero = true
		}
	}
	assert.True(t, hasZero, "should have employees with 0 orders in left join")
}

func TestTypeInferenceNumericCompare(t *testing.T) {
	// 验证 age 按数字排序，而不是按字符串
	res := runSQL(t, `SELECT name, age FROM employees WHERE age < 30 ORDER BY age`, "employees.csv")
	for _, r := range res.Rows {
		age, _ := r[1].AsNumber()
		assert.Less(t, age, 30.0)
	}
	// 验证 salary 数字比较
	res2 := runSQL(t, `SELECT name, salary FROM employees WHERE salary > 30000 ORDER BY salary`, "employees.csv")
	for _, r := range res2.Rows {
		sal, _ := r[1].AsNumber()
		assert.Greater(t, sal, 30000.0)
	}
}

func TestDirtyDataQuotedFields(t *testing.T) {
	res := runSQL(t, `SELECT id, description, tags FROM dirty_data`, "dirty_data.csv")
	assert.Equal(t, 8, len(res.Rows))

	// id=2: 带逗号
	assertText(t, res.Rows[1][1], "字段中包含,逗号")
	assertText(t, res.Rows[1][2], "tag1,tag2")

	// id=3: 带引号
	assertText(t, res.Rows[2][1], `字段中包含"引号"`)

	// id=4: 带换行
	assert.Contains(t, res.Rows[3][1].AsText(), "\n")
	assertText(t, res.Rows[3][2], "multi,line")

	// id=5: 中文
	assertText(t, res.Rows[4][1], "中文,标点。符号!")
	assertText(t, res.Rows[4][2], "中文")

	// id=6: 前后空格 - CSV库会保留原始内容
	// 注意 encoding/csv 默认不 trim
	// 我们的类型推断里会做 trim，但存的时候会保留
}

func TestDistinct(t *testing.T) {
	res := runSQL(t, `SELECT DISTINCT city FROM employees`, "employees.csv")
	assert.Equal(t, 8, len(res.Rows))
}

func TestCaseExpression(t *testing.T) {
	res := runSQL(t, `SELECT name, age,
CASE WHEN age < 25 THEN '青年'
     WHEN age < 35 THEN '中年'
     ELSE '资深' END AS level
FROM employees ORDER BY id`, "employees.csv")
	assertText(t, res.Rows[0][2], "中年") // 张三 28
	assertText(t, res.Rows[2][2], "青年") // 王五 22
	assertText(t, res.Rows[3][2], "资深") // 赵六 42
}

func TestStringFunctions(t *testing.T) {
	res := runSQL(t, `SELECT UPPER('hello') AS uname, LOWER('WORLD') AS lcity, LENGTH(name) AS len FROM employees WHERE id = 1`, "employees.csv")
	require.Equal(t, 1, len(res.Rows))
	assertText(t, res.Rows[0][0], "HELLO")
	assertText(t, res.Rows[0][1], "world")
	assertInt(t, res.Rows[0][2], 2)
}

func TestNumericFunctions(t *testing.T) {
	res := runSQL(t, `SELECT name, salary, ROUND(salary, 0) AS rs, ABS(-5) AS a FROM employees WHERE id = 1`, "employees.csv")
	assertFloat(t, res.Rows[0][colIndex(t, res, "rs")], 15001)
	assertInt(t, res.Rows[0][colIndex(t, res, "a")], 5)
}

func TestSelectAllColumns(t *testing.T) {
	res := runSQL(t, `SELECT * FROM employees`, "employees.csv")
	assert.Equal(t, 6, len(res.Columns))
	assert.Equal(t, 12, len(res.Rows))
}

func TestErrorColumnNotFound(t *testing.T) {
	l := lexer.New(`SELECT non_existent FROM employees`)
	toks, err := l.AllTokens()
	require.NoError(t, err)
	p := parser.New(toks)
	stmt, err := p.Parse()
	require.NoError(t, err)

	opts := csvpkg.DefaultOptions()
	tbl, err := csvpkg.ReadFile(testdata("employees.csv"), "employees", &opts)
	require.NoError(t, err)

	ex := executor.New([]*csvpkg.Table{tbl})
	_, err = ex.Execute(stmt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "column")
}

func TestErrorTableNotFound(t *testing.T) {
	l := lexer.New(`SELECT * FROM nonexistent_table`)
	toks, err := l.AllTokens()
	require.NoError(t, err)
	p := parser.New(toks)
	stmt, err := p.Parse()
	require.NoError(t, err)

	opts := csvpkg.DefaultOptions()
	tbl, err := csvpkg.ReadFile(testdata("employees.csv"), "employees", &opts)
	require.NoError(t, err)

	ex := executor.New([]*csvpkg.Table{tbl})
	_, err = ex.Execute(stmt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table")
}

func TestCoalesce(t *testing.T) {
	res := runSQL(t, `SELECT dept_name, COALESCE(manager_id, 0) AS mid FROM departments`, "departments.csv")
	for _, r := range res.Rows {
		v, ok := r[1].AsNumber()
		assert.True(t, ok)
		if r[0].AsText() == "人力资源部" {
			assert.Equal(t, float64(0), v)
		}
	}
}

func TestConcat(t *testing.T) {
	res := runSQL(t, `SELECT CONCAT(name, ' - ', city) AS info FROM employees WHERE id = 1`, "employees.csv")
	require.Equal(t, 1, len(res.Rows))
	assertText(t, res.Rows[0][0], "张三 - 北京")
}

func TestSubstr(t *testing.T) {
	res := runSQL(t, `SELECT name, SUBSTR(name, 1, 1) AS first_char FROM employees WHERE id = 1`, "employees.csv")
	require.Equal(t, 1, len(res.Rows))
	assertText(t, res.Rows[0][1], "张")
}

func TestHavingClause(t *testing.T) {
	res := runSQL(t, `SELECT city, AVG(salary) AS avg_s FROM employees GROUP BY city HAVING AVG(salary) > 20000`, "employees.csv")
	for _, r := range res.Rows {
		avg, _ := r[1].AsNumber()
		assert.Greater(t, avg, 20000.0)
	}
}

func TestMultipleOrderBy(t *testing.T) {
	res := runSQL(t, `SELECT city, age, name FROM employees ORDER BY city ASC, age DESC`, "employees.csv")
	for i := 1; i < len(res.Rows); i++ {
		prevCity := res.Rows[i-1][0].AsText()
		curCity := res.Rows[i][0].AsText()
		prevAge, _ := res.Rows[i-1][1].AsNumber()
		curAge, _ := res.Rows[i][1].AsNumber()
		if prevCity == curCity {
			assert.GreaterOrEqual(t, prevAge, curAge)
		} else {
			assert.LessOrEqual(t, prevCity, curCity)
		}
	}
}

func TestConstantQuery(t *testing.T) {
	l := lexer.New(`SELECT 1 + 2 AS result, UPPER('hello') AS h`)
	toks, err := l.AllTokens()
	require.NoError(t, err)
	p := parser.New(toks)
	stmt, err := p.Parse()
	require.NoError(t, err)
	ex := executor.New(nil)
	res, err := ex.Execute(stmt)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Rows))
	assertInt(t, res.Rows[0][0], 3)
	assertText(t, res.Rows[0][1], "HELLO")
}
