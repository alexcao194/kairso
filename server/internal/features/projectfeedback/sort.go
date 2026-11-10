package projectfeedback

import (
	"fmt"
	"strings"
)

// feedbackSortColumns is the allow-list of fields callers may sort feedback
// by, mapped to the (trusted, hardcoded) SQL expression used to sort on
// them — mirrors task/sort.go's taskSortColumns. A requested field name is
// validated against this map's keys before use; only the matched,
// allow-listed expression string ever gets interpolated into a
// dynamically-built ORDER BY clause.
var feedbackSortColumns = map[string]string{
	"code":        "f.sequence_no",
	"priority":    "f.priority",
	"feature":     "lower(f.feature_name)",
	"submitter":   "lower(f.submitter_name)",
	"company":     "lower(f.submitter_company)",
	"status":      "array_position(ARRAY['pending', 'acknowledged', 'resolved', 'canceled']::text[], f.status)",
	"expectedFix": "f.expected_fix_date",
	"submitted":   "f.created_at",
}

// feedbackSortFieldNames lists valid sort fields in a stable order, for
// error messages and swagger docs.
var feedbackSortFieldNames = []string{
	"code", "priority", "feature", "submitter", "company", "status", "expectedFix", "submitted",
}

// nullableFeedbackSortColumns are fields whose SQL expression can be NULL —
// sorted with NULLS LAST so feedback missing that field sinks to the bottom
// regardless of direction.
var nullableFeedbackSortColumns = map[string]bool{
	"expectedFix": true,
}

// FeedbackSortRule is one level of a multi-field feedback sort.
type FeedbackSortRule struct {
	Field string
	Order string // "asc" or "desc"
}

// defaultFeedbackSortRules is used whenever the caller doesn't specify a
// sort: most recently submitted first — matches the board's previous fixed
// ORDER BY created_at DESC.
func defaultFeedbackSortRules() []FeedbackSortRule {
	return []FeedbackSortRule{
		{Field: "submitted", Order: "desc"},
	}
}

// parseFeedbackSortParam parses the "sort" query param — a comma-separated
// list of "field" or "field:order" entries, e.g. "priority:desc,submitted:asc"
// (omitting ":order" defaults that entry to ascending). An empty/missing
// value returns defaultFeedbackSortRules(). Every field must be one of
// feedbackSortColumns' keys and each field may only appear once. Mirrors
// task/sort.go's parseTaskSortParam.
func parseFeedbackSortParam(raw string) ([]FeedbackSortRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultFeedbackSortRules(), nil
	}

	parts := strings.Split(raw, ",")
	rules := make([]FeedbackSortRule, 0, len(parts))
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
		if _, ok := feedbackSortColumns[field]; !ok {
			return nil, fmt.Errorf("Trường sắp xếp %q phải là một trong: %s", field, strings.Join(feedbackSortFieldNames, ", "))
		}
		if order != "asc" && order != "desc" {
			return nil, fmt.Errorf("Thứ tự sắp xếp của %q phải là asc hoặc desc (hiện là %q)", field, order)
		}
		if seen[field] {
			return nil, fmt.Errorf("Trường sắp xếp %q bị khai báo nhiều lần", field)
		}
		seen[field] = true
		rules = append(rules, FeedbackSortRule{Field: field, Order: order})
	}
	if len(rules) == 0 {
		return defaultFeedbackSortRules(), nil
	}
	return rules, nil
}

// buildFeedbackOrderByClause turns validated sort rules into a safe ORDER BY
// clause, ending in a stable (created_at, id) tie-breaker. Every field in
// rules must already be a key of feedbackSortColumns — parseFeedbackSortParam
// guarantees this; this function does not itself validate against
// unvalidated input.
func buildFeedbackOrderByClause(rules []FeedbackSortRule) string {
	if len(rules) == 0 {
		rules = defaultFeedbackSortRules()
	}
	var b strings.Builder
	b.WriteString("ORDER BY ")
	for _, rule := range rules {
		expr := feedbackSortColumns[rule.Field]
		dir := "ASC"
		if rule.Order == "desc" {
			dir = "DESC"
		}
		b.WriteString(expr)
		b.WriteString(" ")
		b.WriteString(dir)
		if nullableFeedbackSortColumns[rule.Field] {
			b.WriteString(" NULLS LAST")
		}
		b.WriteString(", ")
	}
	b.WriteString("f.created_at DESC, f.id")
	return b.String()
}

// sqlArgs accumulates positional query args and returns the "$N" placeholder
// for each one added, so hand-built SQL text always references the correct
// parameter index regardless of how many conditions precede it. Small
// enough to duplicate from task/sort.go's identical type rather than export
// an unexported type across packages.
type sqlArgs struct {
	values []interface{}
}

func (a *sqlArgs) bind(v interface{}) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}
