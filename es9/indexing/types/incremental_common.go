package types

import "github.com/volatiletech/sqlboiler/v4/queries/qm"

// dedupStrings returns the non-empty, de-duplicated values of in (order preserved).
func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// inScope builds a `<column> IN (...)` query mod for incremental re-fetch.
func inScope(column string, vals []string) qm.QueryMod {
	xs := make([]interface{}, len(vals))
	for i, v := range vals {
		xs[i] = v
	}
	return qm.WhereIn(column+" IN ?", xs...)
}

// uidInScope re-fetches by the `uid` column.
func uidInScope(uids []string) qm.QueryMod { return inScope("uid", uids) }
