package compare

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ResultTypeComparator defines comparison logic for a specific result type
type ResultTypeComparator interface {
	// GetResultType returns the ES result_type (e.g., "units", "tweets")
	GetResultType() string

	// GetIndexName returns ES6/ES9 index names for a language
	GetES6IndexName(lang string) string
	GetES9IndexName(lang string) string

	// GetCount returns total document count for the result type
	GetES6Count(ctx context.Context, lang string) (int64, error)
	GetES9Count(ctx context.Context, lang string) (int64, error)

	// Sample retrieves random document UIDs from both indices
	Sample(ctx context.Context, lang string, size int) ([]string, error)

	// FetchDocument retrieves a document by mdb_uid from ES6/ES9
	FetchES6Document(ctx context.Context, lang string, uid string) (map[string]interface{}, error)
	FetchES9Document(ctx context.Context, lang string, uid string) (map[string]interface{}, error)

	// Compare performs field-by-field comparison
	Compare(es6Doc, es9Doc map[string]interface{}) *ComparisonResult

	// GetCriticalFields returns fields that must match
	GetCriticalFields() []string

	// GetIgnoredFields returns fields that are expected to differ
	GetIgnoredFields() []string
}

// DiffType describes the type of difference
type DiffType string

const (
	DiffTypeMissing      DiffType = "missing"
	DiffTypeDifferent    DiffType = "different"
	DiffTypeTypeMismatch DiffType = "type_mismatch"
)

// Severity indicates how important a difference is
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// ComparisonResult holds comparison outcome
type ComparisonResult struct {
	MDB_UID      string
	Language     string
	ResultType   string
	PerfectMatch bool

	// Raw documents from ES (for raw JSON output)
	ES6Document map[string]interface{}
	ES9Document map[string]interface{}

	// Field-level differences
	MissingInES9 []string          // Fields present in ES6 but not ES9
	MissingInES6 []string          // Fields present in ES9 but not ES6
	Different    map[string]FieldDiff

	// Critical fields (must match)
	CriticalMatch  bool
	CriticalErrors []string

	// Statistics
	TotalFields     int
	MatchingFields  int
	DifferentFields int
}

// FieldDiff describes a difference in field values
type FieldDiff struct {
	Field    string
	ES6Value interface{}
	ES9Value interface{}
	Type     DiffType
	Severity Severity
	Note     string
}

// ComparisonSummary aggregates results across multiple documents
type ComparisonSummary struct {
	ResultType       string
	Language         string
	TotalCompared    int
	PerfectMatches   int
	WithDifferences  int
	MissingInES9     int // Documents with missing fields in ES9
	MissingInES6     int // Documents with missing fields in ES6
	CriticalErrors   int

	// Document-level fetch failures
	TotalAttempted        int      // Total documents attempted to compare
	DocsNotFoundInES9     []string // UIDs that couldn't be fetched from ES9
	DocsNotFoundInES6     []string // UIDs that couldn't be fetched from ES6

	// Field-level aggregation
	TotalFields        int
	MatchingFields     int
	DifferentFields    int

	// Common differences (field -> count)
	CommonDifferences map[string]int

	// Individual results
	Results []*ComparisonResult
}

// NewComparisonSummary creates a new summary
func NewComparisonSummary(resultType, language string) *ComparisonSummary {
	return &ComparisonSummary{
		ResultType:        resultType,
		Language:          language,
		CommonDifferences: make(map[string]int),
		Results:           make([]*ComparisonResult, 0),
		DocsNotFoundInES9: make([]string, 0),
		DocsNotFoundInES6: make([]string, 0),
	}
}

// AddResult adds a comparison result to the summary
func (s *ComparisonSummary) AddResult(result *ComparisonResult) {
	s.Results = append(s.Results, result)
	s.TotalCompared++

	if result.PerfectMatch {
		s.PerfectMatches++
	} else {
		s.WithDifferences++
	}

	if len(result.MissingInES9) > 0 {
		s.MissingInES9++
	}
	if len(result.MissingInES6) > 0 {
		s.MissingInES6++
	}
	if !result.CriticalMatch {
		s.CriticalErrors++
	}

	s.TotalFields += result.TotalFields
	s.MatchingFields += result.MatchingFields
	s.DifferentFields += result.DifferentFields

	// Track common differences
	for field := range result.Different {
		s.CommonDifferences[field]++
	}
}

// RecordNotFoundInES9 records a document that couldn't be fetched from ES9
func (s *ComparisonSummary) RecordNotFoundInES9(uid string) {
	s.TotalAttempted++
	s.DocsNotFoundInES9 = append(s.DocsNotFoundInES9, uid)
}

// RecordNotFoundInES6 records a document that couldn't be fetched from ES6
func (s *ComparisonSummary) RecordNotFoundInES6(uid string) {
	s.TotalAttempted++
	s.DocsNotFoundInES6 = append(s.DocsNotFoundInES6, uid)
}

// RecordAttempt records that a comparison was attempted (increments TotalAttempted)
func (s *ComparisonSummary) RecordAttempt() {
	s.TotalAttempted++
}

// CompareDocuments performs generic field-by-field comparison
func CompareDocuments(es6Doc, es9Doc map[string]interface{}, criticalFields, ignoredFields []string) *ComparisonResult {
	result := &ComparisonResult{
		ES6Document:    es6Doc,
		ES9Document:    es9Doc,
		Different:      make(map[string]FieldDiff),
		MissingInES9:   make([]string, 0),
		MissingInES6:   make([]string, 0),
		CriticalErrors: make([]string, 0),
		CriticalMatch:  true,
	}

	// Extract metadata
	if uid, ok := es6Doc["mdb_uid"].(string); ok {
		result.MDB_UID = uid
	} else if uid, ok := es9Doc["mdb_uid"].(string); ok {
		result.MDB_UID = uid
	}

	// Build ignored fields map for fast lookup
	ignoredMap := make(map[string]bool)
	for _, field := range ignoredFields {
		ignoredMap[field] = true
	}

	// Build critical fields map
	criticalMap := make(map[string]bool)
	for _, field := range criticalFields {
		criticalMap[field] = true
	}

	// Collect all fields from both documents
	allFields := make(map[string]bool)
	for field := range es6Doc {
		if !ignoredMap[field] {
			allFields[field] = true
		}
	}
	for field := range es9Doc {
		if !ignoredMap[field] {
			allFields[field] = true
		}
	}

	result.TotalFields = len(allFields)

	// Compare each field
	for field := range allFields {
		es6Val, es6Exists := es6Doc[field]
		es9Val, es9Exists := es9Doc[field]

		// Field missing in ES9
		if es6Exists && !es9Exists {
			result.MissingInES9 = append(result.MissingInES9, field)
			severity := SeverityWarning
			if criticalMap[field] {
				severity = SeverityCritical
				result.CriticalMatch = false
				result.CriticalErrors = append(result.CriticalErrors, fmt.Sprintf("Missing critical field: %s", field))
			}
			result.Different[field] = FieldDiff{
				Field:    field,
				ES6Value: es6Val,
				ES9Value: nil,
				Type:     DiffTypeMissing,
				Severity: severity,
				Note:     "Present in ES6 but missing in ES9",
			}
			result.DifferentFields++
			continue
		}

		// Field missing in ES6
		if !es6Exists && es9Exists {
			result.MissingInES6 = append(result.MissingInES6, field)
			result.Different[field] = FieldDiff{
				Field:    field,
				ES6Value: nil,
				ES9Value: es9Val,
				Type:     DiffTypeMissing,
				Severity: SeverityInfo,
				Note:     "Present in ES9 but missing in ES6 (new field)",
			}
			result.DifferentFields++
			continue
		}

		// Both exist - compare values
		if !deepEqual(es6Val, es9Val) {
			severity := SeverityWarning
			diffType := DiffTypeDifferent
			note := ""

			// Check if it's a type mismatch
			if reflect.TypeOf(es6Val) != reflect.TypeOf(es9Val) {
				diffType = DiffTypeTypeMismatch
			}

			// For critical fields, ANY difference is critical (data loss, gain, or change)
			if criticalMap[field] {
				severity = SeverityCritical
				result.CriticalMatch = false

				es6Slice, es6IsSlice := tryGetSliceLength(es6Val)
				es9Slice, es9IsSlice := tryGetSliceLength(es9Val)

				if es6IsSlice && es9IsSlice {
					if es9Slice < es6Slice {
						result.CriticalErrors = append(result.CriticalErrors,
							fmt.Sprintf("%s: ES9 missing %d items (%d in ES6 → %d in ES9)",
								field, es6Slice-es9Slice, es6Slice, es9Slice))
						note = fmt.Sprintf("ES9 missing %d items", es6Slice-es9Slice)
					} else if es9Slice > es6Slice {
						result.CriticalErrors = append(result.CriticalErrors,
							fmt.Sprintf("%s: ES9 has %d extra items (%d in ES6 → %d in ES9)",
								field, es9Slice-es6Slice, es6Slice, es9Slice))
						note = fmt.Sprintf("ES9 has %d extra items", es9Slice-es6Slice)
					} else {
						result.CriticalErrors = append(result.CriticalErrors,
							fmt.Sprintf("%s: Different items (same count: %d)", field, es6Slice))
						note = "Different items (same count)"
					}
				} else {
					result.CriticalErrors = append(result.CriticalErrors, fmt.Sprintf("%s: Values differ", field))
					note = "Values differ"
				}
			}

			result.Different[field] = FieldDiff{
				Field:    field,
				ES6Value: es6Val,
				ES9Value: es9Val,
				Type:     diffType,
				Severity: severity,
				Note:     note,
			}
			result.DifferentFields++
		} else {
			result.MatchingFields++
		}
	}

	// Determine if perfect match
	result.PerfectMatch = len(result.Different) == 0

	return result
}

// tryGetSliceLength attempts to get the length of a slice value
// Returns (length, true) if value is a slice, (0, false) otherwise
func tryGetSliceLength(v interface{}) (int, bool) {
	if v == nil {
		return 0, false
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		return rv.Len(), true
	}

	return 0, false
}

// deepEqual compares two values deeply, handling slices and maps
// For slices, it sorts them before comparing to ignore order differences
func deepEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Check if both are slices
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	if aVal.Kind() == reflect.Slice && bVal.Kind() == reflect.Slice {
		// Sort and compare slices
		return compareSlices(aVal, bVal)
	}

	// Use reflect.DeepEqual for other types
	return reflect.DeepEqual(a, b)
}

// compareSlices compares two slices after sorting them
func compareSlices(a, b reflect.Value) bool {
	if a.Len() != b.Len() {
		return false
	}

	// Convert to string slices for sorting
	aStrs := make([]string, a.Len())
	bStrs := make([]string, b.Len())

	for i := 0; i < a.Len(); i++ {
		aStrs[i] = fmt.Sprintf("%v", a.Index(i).Interface())
	}
	for i := 0; i < b.Len(); i++ {
		bStrs[i] = fmt.Sprintf("%v", b.Index(i).Interface())
	}

	// Sort both slices
	sort.Strings(aStrs)
	sort.Strings(bStrs)

	// Compare sorted slices
	for i := 0; i < len(aStrs); i++ {
		if aStrs[i] != bStrs[i] {
			return false
		}
	}

	return true
}

// FormatValue formats a value for display with sorted arrays
// Always expands arrays to show all items
func FormatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}

	switch val := v.(type) {
	case string:
		if len(val) > 200 {
			return fmt.Sprintf("%s... (%d chars total)", val[:200], len(val))
		}
		return val
	case []interface{}:
		// Convert to sorted string slice
		strs := make([]string, len(val))
		for i, item := range val {
			strs[i] = fmt.Sprintf("%v", item)
		}
		sort.Strings(strs)
		return fmt.Sprintf("[%s]", strings.Join(strs, " "))
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Sprintf("{%s}", strings.Join(keys, ", "))
	default:
		// Try to handle as reflect.Value for other slice types
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Slice {
			strs := make([]string, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				strs[i] = fmt.Sprintf("%v", rv.Index(i).Interface())
			}
			sort.Strings(strs)
			return fmt.Sprintf("[%s]", strings.Join(strs, " "))
		}
		return fmt.Sprintf("%v", val)
	}
}
