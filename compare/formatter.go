package compare

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// OutputFormat defines the output format type
type OutputFormat string

const (
	FormatSummary OutputFormat = "summary"
	FormatDiff    OutputFormat = "diff"
	FormatJSON    OutputFormat = "json"
	FormatRaw     OutputFormat = "raw"
)

// FormatSummary prints a summary of all comparisons
func (s *ComparisonSummary) FormatSummary() string {
	var sb strings.Builder

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("ES6 vs ES9 COMPARISON: %s\n", s.ResultType))
	sb.WriteString(fmt.Sprintf("Language: %s | Sample Size: %d documents\n", s.Language, s.TotalCompared))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Summary stats
	perfectPercent := 0.0
	diffPercent := 0.0
	if s.TotalCompared > 0 {
		perfectPercent = float64(s.PerfectMatches) / float64(s.TotalCompared) * 100
		diffPercent = float64(s.WithDifferences) / float64(s.TotalCompared) * 100
	}

	sb.WriteString("SUMMARY:\n")
	sb.WriteString(fmt.Sprintf("  Total Compared: %d\n", s.TotalCompared))
	sb.WriteString(fmt.Sprintf("  Perfect Match:  %d  (%.1f%%)\n", s.PerfectMatches, perfectPercent))
	sb.WriteString(fmt.Sprintf("  Differences:    %d  (%.1f%%)\n", s.WithDifferences, diffPercent))
	sb.WriteString(fmt.Sprintf("  Missing in ES9: %d  (%.1f%%)\n", s.MissingInES9, float64(s.MissingInES9)/float64(s.TotalCompared)*100))
	sb.WriteString(fmt.Sprintf("  Missing in ES6: %d  (%.1f%%)\n\n", s.MissingInES6, float64(s.MissingInES6)/float64(s.TotalCompared)*100))

	// Critical fields
	sb.WriteString("CRITICAL FIELDS:\n")
	if s.CriticalErrors > 0 {
		sb.WriteString(fmt.Sprintf("  ✗ %d documents have critical field mismatches\n\n", s.CriticalErrors))
	} else {
		sb.WriteString("  ✓ All critical fields match\n\n")
	}

	// Field-level stats
	matchingPercent := 0.0
	if s.TotalFields > 0 {
		matchingPercent = float64(s.MatchingFields) / float64(s.TotalFields) * 100
	}

	sb.WriteString("FIELD-LEVEL STATS:\n")
	sb.WriteString(fmt.Sprintf("  total_fields:      %d\n", s.TotalFields))
	sb.WriteString(fmt.Sprintf("  matching_fields:   %d (%.1f%%)\n", s.MatchingFields, matchingPercent))
	sb.WriteString(fmt.Sprintf("  different_fields:  %d (%.1f%%)\n\n", s.DifferentFields, 100-matchingPercent))

	// Common differences with sample values
	if len(s.CommonDifferences) > 0 {
		sb.WriteString("COMMON DIFFERENCES:\n")

		// Sort by frequency
		type fieldCount struct {
			field string
			count int
		}
		counts := make([]fieldCount, 0, len(s.CommonDifferences))
		for field, count := range s.CommonDifferences {
			counts = append(counts, fieldCount{field, count})
		}
		sort.Slice(counts, func(i, j int) bool {
			return counts[i].count > counts[j].count
		})

		// Print top 10 with sample values
		maxDisplay := 10
		if len(counts) < maxDisplay {
			maxDisplay = len(counts)
		}
		for i := 0; i < maxDisplay; i++ {
			field := counts[i].field
			count := counts[i].count
			percent := float64(count) / float64(s.TotalCompared) * 100

			// Find a sample document with this field difference
			var sampleDiff *FieldDiff
			for _, result := range s.Results {
				if diff, ok := result.Different[field]; ok {
					sampleDiff = &diff
					break
				}
			}

			sb.WriteString(fmt.Sprintf("  - %s: %d docs (%.1f%%)\n", field, count, percent))
			if sampleDiff != nil {
				sb.WriteString(fmt.Sprintf("    Type: %s | Severity: %s\n", sampleDiff.Type, sampleDiff.Severity))
				if sampleDiff.Type == DiffTypeMissing {
					if sampleDiff.ES6Value != nil {
						sb.WriteString(fmt.Sprintf("    ES6: %s\n", FormatValue(sampleDiff.ES6Value)))
						sb.WriteString("    ES9: <missing>\n")
					} else {
						sb.WriteString("    ES6: <missing>\n")
						sb.WriteString(fmt.Sprintf("    ES9: %s\n", FormatValue(sampleDiff.ES9Value)))
					}
				} else {
					sb.WriteString(fmt.Sprintf("    ES6: %s\n", FormatValue(sampleDiff.ES6Value)))
					sb.WriteString(fmt.Sprintf("    ES9: %s\n", FormatValue(sampleDiff.ES9Value)))
				}
				if sampleDiff.Note != "" {
					sb.WriteString(fmt.Sprintf("    Note: %s\n", sampleDiff.Note))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Sample details (first 5 with differences)
	if s.WithDifferences > 0 {
		sb.WriteString("SAMPLE DIFFERENCES:\n")
		shown := 0
		for i, result := range s.Results {
			if !result.PerfectMatch && shown < 5 {
				shown++
				diffFields := make([]string, 0, len(result.Different))
				for field := range result.Different {
					diffFields = append(diffFields, field)
				}
				sort.Strings(diffFields)
				sb.WriteString(fmt.Sprintf("  [%d/%d] %s: %s\n",
					shown, s.WithDifferences, result.MDB_UID,
					strings.Join(diffFields[:min(3, len(diffFields))], ", ")))
			}
			if shown >= 5 {
				break
			}
			_ = i
		}
	}

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	return sb.String()
}

// FormatDiff prints detailed field-by-field differences
func (s *ComparisonSummary) FormatDiff() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("DETAILED COMPARISON: %s (%s)\n", s.ResultType, s.Language))
	sb.WriteString(fmt.Sprintf("Total: %d documents\n\n", s.TotalCompared))

	for i, result := range s.Results {
		if result.PerfectMatch {
			sb.WriteString(fmt.Sprintf("[%d/%d] %s: ✓ PERFECT MATCH\n\n", i+1, s.TotalCompared, result.MDB_UID))
			continue
		}

		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString(fmt.Sprintf("[%d/%d] Document: %s\n", i+1, s.TotalCompared, result.MDB_UID))
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		// Critical errors first
		if !result.CriticalMatch {
			sb.WriteString("⚠ CRITICAL FIELD MISMATCHES:\n")
			for _, err := range result.CriticalErrors {
				sb.WriteString(fmt.Sprintf("  ✗ %s\n", err))
			}
			sb.WriteString("\n")
		}

		// Sort fields for consistent output
		fields := make([]string, 0, len(result.Different))
		for field := range result.Different {
			fields = append(fields, field)
		}
		sort.Strings(fields)

		// Print differences
		for _, field := range fields {
			diff := result.Different[field]
			severity := "⚠"
			if diff.Severity == SeverityCritical {
				severity = "✗"
			} else if diff.Severity == SeverityInfo {
				severity = "ℹ"
			}

			sb.WriteString(fmt.Sprintf("%s %s: %s\n", severity, field, diff.Type))

			if diff.Type == DiffTypeMissing {
				if diff.ES6Value != nil {
					sb.WriteString(fmt.Sprintf("  ES6: %s\n", FormatValue(diff.ES6Value)))
					sb.WriteString("  ES9: <missing>\n")
				} else {
					sb.WriteString("  ES6: <missing>\n")
					sb.WriteString(fmt.Sprintf("  ES9: %s\n", FormatValue(diff.ES9Value)))
				}
			} else {
				sb.WriteString(fmt.Sprintf("  ES6: %s\n", FormatValue(diff.ES6Value)))
				sb.WriteString(fmt.Sprintf("  ES9: %s\n", FormatValue(diff.ES9Value)))
			}

			if diff.Note != "" {
				sb.WriteString(fmt.Sprintf("  Note: %s\n", diff.Note))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// FormatJSON outputs results as JSON
func (s *ComparisonSummary) FormatJSON() string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Errorf("Failed to marshal comparison summary: %v", err)
		return "{}"
	}
	return string(data)
}

// FormatRaw outputs raw ES6 and ES9 JSON documents side-by-side
func (s *ComparisonSummary) FormatRaw() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("RAW DOCUMENT COMPARISON: %s (%s)\n", s.ResultType, s.Language))
	sb.WriteString(fmt.Sprintf("Total: %d documents\n\n", s.TotalCompared))

	for i, result := range s.Results {
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString(fmt.Sprintf("[%d/%d] Document: %s\n", i+1, s.TotalCompared, result.MDB_UID))
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		if result.PerfectMatch {
			sb.WriteString("✓ PERFECT MATCH\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("✗ %d DIFFERENCES\n\n", len(result.Different)))
		}

		// ES6 Document
		sb.WriteString("ES6 DOCUMENT:\n")
		sb.WriteString("─────────────────────────────────────────────────────────────\n")
		if result.ES6Document != nil {
			es6JSON, err := json.MarshalIndent(result.ES6Document, "", "  ")
			if err != nil {
				sb.WriteString(fmt.Sprintf("ERROR: %v\n", err))
			} else {
				sb.WriteString(string(es6JSON))
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("<nil>\n")
		}
		sb.WriteString("\n")

		// ES9 Document
		sb.WriteString("ES9 DOCUMENT:\n")
		sb.WriteString("─────────────────────────────────────────────────────────────\n")
		if result.ES9Document != nil {
			es9JSON, err := json.MarshalIndent(result.ES9Document, "", "  ")
			if err != nil {
				sb.WriteString(fmt.Sprintf("ERROR: %v\n", err))
			} else {
				sb.WriteString(string(es9JSON))
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("<nil>\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
