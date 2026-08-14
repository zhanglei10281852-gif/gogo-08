# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA，再应用仓库内的可信测试补丁；不要在当前修复结果源码上期待重新出现修复前失败。

## 问题现象

DISTINCT 查询配合 ORDER BY 时顺序错了，请帮我修复。

查询会投影重复值并用未出现在 SELECT 列表里的来源列排序。去重本身得到的值集合正确，但保留下来的值会使用另一条原始记录的排序键，导致升降序都不符合 SQL 结果；LIMIT 又会截取错误的行。没有 DISTINCT 或直接按投影列排序时通常正常。

期望每个 DISTINCT 输出值在后续排序和分页阶段仍与产生该值的来源记录保持一致，升序、降序和 LIMIT 均返回确定且正确的结果。修复后请保证 go test ./... 全绿。

## 含 Bug 版本

- 仓库：zhanglei10281852-gif/gogo-08
- 仓库地址：https://github.com/zhanglei10281852-gif/gogo-08.git
- parent SHA：45d357b96215844424f47cca4c5052e30f3f940b

## 复现步骤

```bash
git clone -- https://github.com/zhanglei10281852-gif/gogo-08.git bug-repro
cd bug-repro
git checkout --detach 45d357b96215844424f47cca4c5052e30f3f940b
git apply ../BENZHI_VALIDATION/trusted-test.patch
go test ./internal/executor -run "^TestDistinctOrderByKeepsSourceRows$" -count=1 -v
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/executor -run "^TestDistinctOrderByKeepsSourceRows$" -count=1 -v
=== RUN   TestDistinctOrderByKeepsSourceRows
=== RUN   TestDistinctOrderByKeepsSourceRows/adjacent_and_non-adjacent_duplicates_ordered_by_projected_column_ascending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"alpha"}, []string{"bravo"}, []string{"charlie"}, []string{"delta"}}
        	            	actual  : [][]string{[]string{"bravo"}, []string{"delta"}, []string{"charlie"}, []string{"alpha"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,6 +2,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "bravo"
        	            	+  (string) (len=5) "delta"
        	            	  },
        	            	@@ -11,3 +11,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/adjacent_and_non-adjacent_duplicates_ordered_by_projected_column_ascending
=== RUN   TestDistinctOrderByKeepsSourceRows/projected_column_descending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"delta"}, []string{"charlie"}, []string{"bravo"}, []string{"alpha"}}
        	            	actual  : [][]string{[]string{"charlie"}, []string{"alpha"}, []string{"delta"}, []string{"bravo"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -1,2 +1,8 @@
        	            	 ([][]string) (len=4) {
        	            	+ ([]string) (len=1) {
        	            	+  (string) (len=7) "charlie"
        	            	+ },
        	            	+ ([]string) (len=1) {
        	            	+  (string) (len=5) "alpha"
        	            	+ },
        	            	  ([]string) (len=1) {
        	            	@@ -5,9 +11,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=7) "charlie"
        	            	- },
        	            	- ([]string) (len=1) {
        	            	   (string) (len=5) "bravo"
        	            	- },
        	            	- ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/projected_column_descending
=== RUN   TestDistinctOrderByKeepsSourceRows/unprojected_column_ascending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"delta"}, []string{"alpha"}, []string{"charlie"}, []string{"bravo"}}
        	            	actual  : [][]string{[]string{"alpha"}, []string{"bravo"}, []string{"charlie"}, []string{"delta"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,6 +2,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  },
        	            	@@ -11,3 +11,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "bravo"
        	            	+  (string) (len=5) "delta"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/unprojected_column_ascending
=== RUN   TestDistinctOrderByKeepsSourceRows/unprojected_column_descending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"bravo"}, []string{"charlie"}, []string{"alpha"}, []string{"delta"}}
        	            	actual  : [][]string{[]string{"delta"}, []string{"charlie"}, []string{"bravo"}, []string{"alpha"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,3 +2,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "bravo"
        	            	+  (string) (len=5) "delta"
        	            	  },
        	            	@@ -8,6 +8,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/unprojected_column_descending
=== RUN   TestDistinctOrderByKeepsSourceRows/order_before_limit
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"delta"}, []string{"alpha"}}
        	            	actual  : [][]string{[]string{"alpha"}, []string{"bravo"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,6 +2,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/order_before_limit
--- FAIL: TestDistinctOrderByKeepsSourceRows (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/adjacent_and_non-adjacent_duplicates_ordered_by_projected_column_ascending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/projected_column_descending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/unprojected_column_ascending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/unprojected_column_descending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/order_before_limit (0.00s)
FAIL
FAIL	csvsql/internal/executor	0.005s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/executor -run "^TestDistinctOrderByKeepsSourceRows$" -count=1 -v
=== RUN   TestDistinctOrderByKeepsSourceRows
=== RUN   TestDistinctOrderByKeepsSourceRows/adjacent_and_non-adjacent_duplicates_ordered_by_projected_column_ascending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"alpha"}, []string{"bravo"}, []string{"charlie"}, []string{"delta"}}
        	            	actual  : [][]string{[]string{"bravo"}, []string{"delta"}, []string{"charlie"}, []string{"alpha"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,6 +2,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "bravo"
        	            	+  (string) (len=5) "delta"
        	            	  },
        	            	@@ -11,3 +11,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/adjacent_and_non-adjacent_duplicates_ordered_by_projected_column_ascending
=== RUN   TestDistinctOrderByKeepsSourceRows/projected_column_descending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"delta"}, []string{"charlie"}, []string{"bravo"}, []string{"alpha"}}
        	            	actual  : [][]string{[]string{"charlie"}, []string{"alpha"}, []string{"delta"}, []string{"bravo"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -1,2 +1,8 @@
        	            	 ([][]string) (len=4) {
        	            	+ ([]string) (len=1) {
        	            	+  (string) (len=7) "charlie"
        	            	+ },
        	            	+ ([]string) (len=1) {
        	            	+  (string) (len=5) "alpha"
        	            	+ },
        	            	  ([]string) (len=1) {
        	            	@@ -5,9 +11,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=7) "charlie"
        	            	- },
        	            	- ([]string) (len=1) {
        	            	   (string) (len=5) "bravo"
        	            	- },
        	            	- ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/projected_column_descending
=== RUN   TestDistinctOrderByKeepsSourceRows/unprojected_column_ascending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"delta"}, []string{"alpha"}, []string{"charlie"}, []string{"bravo"}}
        	            	actual  : [][]string{[]string{"alpha"}, []string{"bravo"}, []string{"charlie"}, []string{"delta"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,6 +2,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  },
        	            	@@ -11,3 +11,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "bravo"
        	            	+  (string) (len=5) "delta"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/unprojected_column_ascending
=== RUN   TestDistinctOrderByKeepsSourceRows/unprojected_column_descending
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"bravo"}, []string{"charlie"}, []string{"alpha"}, []string{"delta"}}
        	            	actual  : [][]string{[]string{"delta"}, []string{"charlie"}, []string{"bravo"}, []string{"alpha"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,3 +2,3 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "bravo"
        	            	+  (string) (len=5) "delta"
        	            	  },
        	            	@@ -8,6 +8,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/unprojected_column_descending
=== RUN   TestDistinctOrderByKeepsSourceRows/order_before_limit
    distinct_order_test.go:59: 
        	Error Trace:	/app/internal/executor/distinct_order_test.go:59
        	Error:      	Not equal: 
        	            	expected: [][]string{[]string{"delta"}, []string{"alpha"}}
        	            	actual  : [][]string{[]string{"alpha"}, []string{"bravo"}}
        	            	
        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -2,6 +2,6 @@
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "delta"
        	            	+  (string) (len=5) "alpha"
        	            	  },
        	            	  ([]string) (len=1) {
        	            	-  (string) (len=5) "alpha"
        	            	+  (string) (len=5) "bravo"
        	            	  }
        	Test:       	TestDistinctOrderByKeepsSourceRows/order_before_limit
--- FAIL: TestDistinctOrderByKeepsSourceRows (0.05s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/adjacent_and_non-adjacent_duplicates_ordered_by_projected_column_ascending (0.03s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/projected_column_descending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/unprojected_column_ascending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/unprojected_column_descending (0.00s)
    --- FAIL: TestDistinctOrderByKeepsSourceRows/order_before_limit (0.00s)
FAIL
FAIL	csvsql/internal/executor	0.227s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

定向测试通过：go test ./internal/executor -run '^TestDistinctOrderByKeepsSourceRows$' -count=1 -v
全量 go test ./... -count=1、go build ./...、go vet ./... 通过
linux/amd64 与 linux/arm64 均通过；验证 DISTINCT、ASC/DESC 与 LIMIT 的公开结果
