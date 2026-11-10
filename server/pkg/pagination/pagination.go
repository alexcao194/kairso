package pagination

import (
	"net/http"
	"strconv"
)

// Params represents pagination parameters.
type Params struct {
	// Page number.
	Page int `json:"page"`
	// Page size.
	Limit int `json:"limit"`
}

// Offset calculates the SQL OFFSET based on Page and Limit.
func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// LimitWithMore returns Limit + 1 to check if there are more items.
func (p Params) LimitWithMore() int {
	return p.Limit + 1
}

// StatsParams is Params plus an optional month filter (YYYY-MM), for list
// endpoints whose per-item stat aggregates (e.g. a project's taskCount /
// totalEstimation) can be scoped to a month — the project/module lists that
// feed the Statistics screens' pie charts, month-filtered the same way as
// the workspace-wide task-statistics endpoint.
type StatsParams struct {
	Params
	// Filter task stats (taskCount/totalEstimation) to this month, YYYY-MM.
	Month string `json:"month"`
}

// ParseStats extracts pagination parameters plus the optional "month" query
// param from the HTTP request.
func ParseStats(r *http.Request, defaultLimit, maxLimit int) StatsParams {
	return StatsParams{
		Params: Parse(r, defaultLimit, maxLimit),
		Month:  r.URL.Query().Get("month"),
	}
}

// Parse extracts pagination parameters from the HTTP request query string.
func Parse(r *http.Request, defaultLimit, maxLimit int) Params {
	query := r.URL.Query()

	page := 1
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = p
	}

	limit := defaultLimit
	if l, err := strconv.Atoi(query.Get("limit")); err == nil && l > 0 {
		if maxLimit > 0 && l > maxLimit {
			limit = maxLimit
		} else {
			limit = l
		}
	}

	return Params{
		Page:  page,
		Limit: limit,
	}
}

// Process returns the items to return and whether there are more pages.
// Items slice MUST be passed as a pointer so we can trim it if it exceeds Limit.
func Process[T any](items *[]T, limit int) bool {
	hasMore := false
	if len(*items) > limit {
		hasMore = true
		*items = (*items)[:limit]
	}
	return hasMore
}

// Response represents a paginated response.
type Response[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"hasMore"`
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
}

// NewResponse creates a new pagination response.
func NewResponse[T any](items []T, hasMore bool, page, limit int) Response[T] {
	return Response[T]{
		Items:   items,
		HasMore: hasMore,
		Page:    page,
		Limit:   limit,
	}
}
