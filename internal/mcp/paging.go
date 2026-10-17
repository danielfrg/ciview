package mcp

import "fmt"

type Paging struct {
	Limit  *int `json:"limit,omitempty" jsonschema:"Maximum results, 1 to 100 (default 20)"`
	Offset int  `json:"offset,omitempty" jsonschema:"Zero-based result offset"`
}

func (p Paging) bounds(total int) (int, int, *int, error) {
	limit := 20
	if p.Limit != nil {
		limit = *p.Limit
	}
	if limit < 1 || limit > 100 || p.Offset < 0 {
		return 0, 0, nil, fmt.Errorf("limit must be 1 to 100 and offset must be nonnegative")
	}
	start := min(p.Offset, total)
	end := start + min(limit, total-start)
	var next *int
	if end < total {
		next = &end
	}
	return start, end, next, nil
}
