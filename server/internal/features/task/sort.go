package task

import (
	"fmt"
	"strings"
)

// taskSortColumns is the allow-list of fields callers may sort tasks by,
// mapped to the (trusted, hardcoded) SQL expression used to sort on them.
// A requested field name is validated against this map's keys before use —
// only the matched, allow-listed expression string ever gets interpolated
// into a dynamically-built ORDER BY clause; the raw requested string never
// reaches the SQL text itself.
var taskSortColumns = map[string]string{
	"title":   "lower(t.title)",
	"project": "lower(p.name)",
	// Module's own timeline position, not its (often arbitrarily-chosen)
	// name — grouping by module is meant to read like a roadmap, earliest
	// module first, so it has to follow modules.start_date.
	"module":      "m.start_date",
	"status":      "array_position(ARRAY['backlog', 'todo', 'in_progress', 'review', 'done', 'canceled']::task_status[], t.status)",
	"priority":    "t.priority",
	"estimation":  "t.estimation",
	"returnCount": "t.return_count",
	"startDate":   "t.start_date",
	"endDate":     "t.end_date",
}

// taskSortFieldNames lists valid sort fields in a stable order, for error
// messages and swagger docs.
var taskSortFieldNames = []string{
	"title", "project", "module", "status", "priority", "estimation", "returnCount", "startDate", "endDate",
}

// nullableSortColumns are fields whose SQL expression can be NULL — sorted
// with NULLS LAST so tasks missing that field sink to the bottom regardless
// of direction (matches the previous fixed-CASE-WHEN query's behavior).
var nullableSortColumns = map[string]bool{
	"title":      true,
	"project":    true,
	"module":     true,
	"estimation": true,
	"startDate":  true,
	"endDate":    true,
}

// TaskSortRule is one level of a multi-field task sort.
type TaskSortRule struct {
	Field string
	Order string // "asc" or "desc"
}

// defaultTaskSortRules is used whenever the caller doesn't specify a sort:
// status ascending, then deadline (end date) ascending.
func defaultTaskSortRules() []TaskSortRule {
	return []TaskSortRule{
		{Field: "status", Order: "asc"},
		{Field: "endDate", Order: "asc"},
	}
}

// parseTaskSortParam parses the "sort" query param — a comma-separated list
// of "field" or "field:order" entries, e.g. "status:asc,endDate:desc"
// (omitting ":order" defaults that entry to ascending). An empty/missing
// value returns defaultTaskSortRules(). Every field must be one of
// taskSortColumns' keys and each field may only appear once.
func parseTaskSortParam(raw string) ([]TaskSortRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultTaskSortRules(), nil
	}

	parts := strings.Split(raw, ",")
	rules := make([]TaskSortRule, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		field, order, _ := strings.Cut(part, ":")
		field = strings.TrimSpace(field)
		order = strings.ToLower(strings.TrimSpace(order))
		if order == "" {
			order = "asc"
		}
		if _, ok := taskSortColumns[field]; !ok {
			return nil, fmt.Errorf("Trường sắp xếp %q phải là một trong: %s", field, strings.Join(taskSortFieldNames, ", "))
		}
		if order != "asc" && order != "desc" {
			return nil, fmt.Errorf("Thứ tự sắp xếp của %q phải là asc hoặc desc (hiện là %q)", field, order)
		}
		if seen[field] {
			return nil, fmt.Errorf("Trường sắp xếp %q bị khai báo nhiều lần", field)
		}
		seen[field] = true
		rules = append(rules, TaskSortRule{Field: field, Order: order})
	}
	if len(rules) == 0 {
		return defaultTaskSortRules(), nil
	}
	return rules, nil
}

// buildTaskOrderByClause turns validated sort rules into a safe ORDER BY
// clause, ending in a stable (created_at, id) tie-breaker. Every field in
// rules must already be a key of taskSortColumns — parseTaskSortParam
// guarantees this; this function does not itself validate against
// unvalidated input.
func buildTaskOrderByClause(rules []TaskSortRule) string {
	if len(rules) == 0 {
		rules = defaultTaskSortRules()
	}
	var b strings.Builder
	b.WriteString("ORDER BY ")
	for _, rule := range rules {
		expr := taskSortColumns[rule.Field]
		dir := "ASC"
		if rule.Order == "desc" {
			dir = "DESC"
		}
		b.WriteString(expr)
		b.WriteString(" ")
		b.WriteString(dir)
		if nullableSortColumns[rule.Field] {
			b.WriteString(" NULLS LAST")
		}
		b.WriteString(", ")
	}
	b.WriteString("t.created_at DESC, t.id")
	return b.String()
}

// sqlArgs accumulates positional query args and returns the "$N"
// placeholder for each one added, so hand-built SQL text always references
// the correct parameter index regardless of how many conditions precede it.
type sqlArgs struct {
	values []interface{}
}

func (a *sqlArgs) bind(v interface{}) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}
