package search

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// JSON HTML renderer — sorted keys + per-line diff highlighting
// ---------------------------------------------------------------------------

// buildDiffPathMap returns a map of JSON path → highlight class for one side.
// side is "base" or "exp".
func buildDiffPathMap(diffs []JsonDiff, side string) map[string]string {
	m := make(map[string]string)
	for _, d := range diffs {
		switch d.Kind {
		case "different":
			m[d.Path] = "changed"
		case "only_in_base":
			if side == "base" {
				m[d.Path] = "removed"
			}
		case "only_in_exp":
			if side == "exp" {
				m[d.Path] = "added"
			}
		}
	}
	return m
}

type jsonPaneRenderer struct {
	sb      strings.Builder
	diffMap map[string]string
}

// emit writes one JSON line wrapped in a styled <span>.
func (r *jsonPaneRenderer) emit(path, text string) {
	kind := r.diffMap[path]
	class := "jl"
	if kind != "" {
		class += " jd-" + kind
	}
	r.sb.WriteString(`<span class="` + class + `">` + html.EscapeString(text) + `</span>`)
}

func (r *jsonPaneRenderer) renderObject(obj map[string]interface{}, path, indent, trail string) {
	keys := jsonSortedKeys(obj)
	if len(keys) == 0 {
		r.emit(path, indent+"{}"+trail)
		return
	}
	r.emit(path, indent+"{")
	for i, k := range keys {
		cp := jsonChildPath(path, k)
		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}
		r.renderKV(k, obj[k], cp, indent+"  ", comma)
	}
	r.emit("", indent+"}"+trail)
}

func (r *jsonPaneRenderer) renderKV(key string, val interface{}, path, indent, trail string) {
	kp := indent + jsonQuoteKey(key) + ": "
	switch t := val.(type) {
	case map[string]interface{}:
		keys := jsonSortedKeys(t)
		if len(keys) == 0 {
			r.emit(path, kp+"{}"+trail)
			return
		}
		r.emit(path, kp+"{")
		for i, k := range keys {
			cp := jsonChildPath(path, k)
			comma := ","
			if i == len(keys)-1 {
				comma = ""
			}
			r.renderKV(k, t[k], cp, indent+"  ", comma)
		}
		r.emit("", indent+"}"+trail)
	case []interface{}:
		if len(t) == 0 {
			r.emit(path, kp+"[]"+trail)
			return
		}
		r.emit(path, kp+"[")
		for i, item := range t {
			cp := fmt.Sprintf("%s[%d]", path, i)
			comma := ","
			if i == len(t)-1 {
				comma = ""
			}
			r.renderItem(item, cp, indent+"  ", comma)
		}
		r.emit("", indent+"]"+trail)
	default:
		r.emit(path, kp+jsonScalarStr(val)+trail)
	}
}

func (r *jsonPaneRenderer) renderItem(item interface{}, path, indent, trail string) {
	switch t := item.(type) {
	case map[string]interface{}:
		keys := jsonSortedKeys(t)
		if len(keys) == 0 {
			r.emit(path, indent+"{}"+trail)
			return
		}
		r.emit(path, indent+"{")
		for i, k := range keys {
			cp := jsonChildPath(path, k)
			comma := ","
			if i == len(keys)-1 {
				comma = ""
			}
			r.renderKV(k, t[k], cp, indent+"  ", comma)
		}
		r.emit("", indent+"}"+trail)
	case []interface{}:
		if len(t) == 0 {
			r.emit(path, indent+"[]"+trail)
			return
		}
		r.emit(path, indent+"[")
		for i, sub := range t {
			cp := fmt.Sprintf("%s[%d]", path, i)
			comma := ","
			if i == len(t)-1 {
				comma = ""
			}
			r.renderItem(sub, cp, indent+"  ", comma)
		}
		r.emit("", indent+"]"+trail)
	default:
		r.emit(path, indent+jsonScalarStr(item)+trail)
	}
}

func jsonSortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func jsonChildPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func jsonScalarStr(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func jsonQuoteKey(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// renderJSONPaneHTML renders rawJSON as HTML with per-line diff highlighting.
func renderJSONPaneHTML(rawJSON string, diffs []JsonDiff, side string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return html.EscapeString(rawJSON)
	}
	r := &jsonPaneRenderer{diffMap: buildDiffPathMap(diffs, side)}
	r.renderObject(m, "", "", "")
	return r.sb.String()
}

// ---------------------------------------------------------------------------
// HTML report
// ---------------------------------------------------------------------------

// SearchCompareHTMLReport renders the summary to a self-contained HTML string.
func SearchCompareHTMLReport(summary SearchCompareSummary, baseURL, expURL string) string {
	var sb strings.Builder

	identicalPct := 0.0
	identicalTopPct := 0.0
	differentPct := 0.0
	errorPct := 0.0
	if summary.Total > 0 {
		identicalPct = float64(summary.Identical) * 100 / float64(summary.Total)
		identicalTopPct = float64(summary.IdenticalTop) * 100 / float64(summary.Total)
		differentPct = float64(summary.Different) * 100 / float64(summary.Total)
		errorPct = float64(summary.Errors) * 100 / float64(summary.Total)
	}

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Search Compare Report</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:monospace;font-size:13px;background:#f5f5f5;color:#222}
h1{font-size:18px;padding:16px;background:#333;color:#fff}
.meta{padding:10px 16px;background:#444;color:#ccc;font-size:12px}
.summary{display:flex;gap:16px;padding:12px 16px;background:#fff;border-bottom:1px solid #ddd;flex-wrap:wrap}
.stat{padding:8px 16px;border-radius:4px;font-weight:bold;font-size:14px}
.stat.ok{background:#d4edda;color:#155724}
.stat.top{background:#e8d5ff;color:#4b0082}
.stat.diff{background:#f8d7da;color:#721c24}
.stat.err{background:#fff3cd;color:#856404}
.stat.total{background:#d1ecf1;color:#0c5460}
table{width:100%;border-collapse:collapse}
th{position:sticky;top:0;background:#e9ecef;padding:7px 10px;text-align:left;border-bottom:2px solid #adb5bd;font-size:12px;text-transform:uppercase;letter-spacing:.5px}
td{padding:6px 10px;border-bottom:1px solid #dee2e6;vertical-align:top}
tr.identical td{background:#f8fff8}
tr.identical-top td{background:#f5f0ff}
tr.different td{background:#fff8f8}
tr.error-row td{background:#fffbf0}
.badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:11px;font-weight:bold}
.badge.ok{background:#28a745;color:#fff}
.badge.top{background:#6f42c1;color:#fff}
.badge.diff{background:#dc3545;color:#fff}
.badge.err{background:#ffc107;color:#333}
.toggle-btn{cursor:pointer;border:1px solid #aaa;background:#fff;padding:2px 8px;border-radius:3px;font-size:11px;font-family:monospace}
.toggle-btn:hover{background:#eee}
.detail{display:none;margin-top:6px}
.detail.open{display:block}
.diff-list{background:#fff;border:1px solid #f5c6cb;border-radius:3px;padding:6px 10px;margin-top:4px;font-size:11px;max-height:200px;overflow-y:auto}
.diff-item{padding:2px 0;border-bottom:1px solid #fce4e4}
.diff-item:last-child{border-bottom:none}
.diff-path{color:#666;min-width:220px;display:inline-block}
.diff-base{color:#c0392b}
.diff-exp{color:#27ae60}
.json-panes{display:flex;gap:8px;margin-top:6px}
.json-pane{flex:1;border:1px solid #ddd;border-radius:3px;overflow:hidden;display:flex;flex-direction:column;max-height:500px}
.json-pane-hdr{font-size:11px;color:#555;padding:3px 6px;background:#f0f0f0;border-bottom:1px solid #ddd;font-family:sans-serif;flex-shrink:0}
.json-content{flex:1;overflow:auto;padding:4px 6px;font-size:10px;margin:0;background:#fff}
.jl{display:block;white-space:pre}
.jd-changed{background:#fff3cd}
.jd-removed{background:#f8d7da}
.jd-added{background:#d4edda}
.err-msg{color:#856404;font-size:11px;margin-top:4px;background:#fff3cd;border-radius:3px;padding:4px 8px}
.lang-tag{display:inline-block;width:28px;text-align:center;padding:1px 4px;border-radius:3px;background:#6c757d;color:#fff;font-size:10px;font-weight:bold;margin-right:4px}
</style>
</head>
<body>
`)

	sb.WriteString("<h1>Search Compare Report</h1>\n")
	sb.WriteString(fmt.Sprintf(`<div class="meta">Generated: %s &nbsp;|&nbsp; Base: <b>%s</b> &nbsp;|&nbsp; Exp: <b>%s</b></div>`,
		html.EscapeString(time.Now().Format("2006-01-02 15:04:05")),
		html.EscapeString(baseURL),
		html.EscapeString(expURL),
	))
	sb.WriteString("\n")

	sb.WriteString(`<div class="summary">`)
	sb.WriteString(fmt.Sprintf(`<div class="stat total">Total: %d</div>`, summary.Total))
	sb.WriteString(fmt.Sprintf(`<div class="stat ok">✓ Identical: %d (%.1f%%)</div>`, summary.Identical, identicalPct))
	if summary.IdenticalTop > 0 {
		sb.WriteString(fmt.Sprintf(`<div class="stat top">~ Identical top: %d (%.1f%%)</div>`, summary.IdenticalTop, identicalTopPct))
	}
	sb.WriteString(fmt.Sprintf(`<div class="stat diff">✗ Different: %d (%.1f%%)</div>`, summary.Different, differentPct))
	if summary.Errors > 0 {
		sb.WriteString(fmt.Sprintf(`<div class="stat err">⚠ Errors: %d (%.1f%%)</div>`, summary.Errors, errorPct))
	}
	sb.WriteString("</div>\n")

	sb.WriteString("<table>\n<thead><tr>")
	sb.WriteString("<th>#</th><th>Lang</th><th>Query</th><th>Status</th><th>Details</th>")
	sb.WriteString("</tr></thead>\n<tbody>\n")

	for i, r := range summary.Results {
		rowClass := "identical"
		badge := `<span class="badge ok">✓ identical</span>`
		if r.BaseError != "" || r.ExpError != "" {
			rowClass = "error-row"
			badge = `<span class="badge err">⚠ error</span>`
		} else if r.IdenticalTop {
			rowClass = "identical-top"
			badge = fmt.Sprintf(`<span class="badge top">~ identical top (%d tail diff(s))</span>`, len(r.Diffs))
		} else if !r.Identical {
			rowClass = "different"
			badge = fmt.Sprintf(`<span class="badge diff">✗ %d diff(s)</span>`, len(r.Diffs))
		}

		sb.WriteString(fmt.Sprintf(`<tr class="%s">`, rowClass))
		sb.WriteString(fmt.Sprintf(`<td>%d</td>`, i+1))
		sb.WriteString(fmt.Sprintf(`<td><span class="lang-tag">%s</span></td>`, html.EscapeString(r.Language)))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, html.EscapeString(r.Query)))
		sb.WriteString(fmt.Sprintf(`<td>%s</td>`, badge))

		sb.WriteString("<td>")
		if r.BaseError != "" || r.ExpError != "" {
			if r.BaseError != "" {
				sb.WriteString(fmt.Sprintf(`<div class="err-msg">Base error: %s</div>`, html.EscapeString(r.BaseError)))
			}
			if r.ExpError != "" {
				sb.WriteString(fmt.Sprintf(`<div class="err-msg">Exp error: %s</div>`, html.EscapeString(r.ExpError)))
			}
		} else if !r.Identical {
			detailID := fmt.Sprintf("d%d", i)
			sb.WriteString(fmt.Sprintf(`<button class="toggle-btn" onclick="toggle('%s')">show diff</button>`, detailID))
			sb.WriteString(fmt.Sprintf(`<div class="detail" id="%s">`, detailID))

			// Diff summary list
			sb.WriteString(`<div class="diff-list">`)
			for _, d := range r.Diffs {
				sb.WriteString(`<div class="diff-item">`)
				sb.WriteString(fmt.Sprintf(`<span class="diff-path">%s</span> `, html.EscapeString(d.Path)))
				switch d.Kind {
				case "only_in_base":
					sb.WriteString(fmt.Sprintf(`<span class="diff-base">only in base: %s</span>`, html.EscapeString(fmt.Sprintf("%v", d.Base))))
				case "only_in_exp":
					sb.WriteString(fmt.Sprintf(`<span class="diff-exp">only in exp: %s</span>`, html.EscapeString(fmt.Sprintf("%v", d.Exp))))
				default:
					sb.WriteString(fmt.Sprintf(`<span class="diff-base">base: %s</span>`, html.EscapeString(fmt.Sprintf("%v", d.Base))))
					sb.WriteString(` &nbsp;≠&nbsp; `)
					sb.WriteString(fmt.Sprintf(`<span class="diff-exp">exp: %s</span>`, html.EscapeString(fmt.Sprintf("%v", d.Exp))))
				}
				sb.WriteString("</div>")
			}
			sb.WriteString("</div>") // diff-list

			// Side-by-side JSON panes with synchronized scroll
			baseID := detailID + "-base"
			expID := detailID + "-exp"
			sb.WriteString(`<div class="json-panes">`)

			sb.WriteString(`<div class="json-pane">`)
			sb.WriteString(`<div class="json-pane-hdr">Base</div>`)
			sb.WriteString(fmt.Sprintf(`<pre class="json-content" id="%s">`, baseID))
			sb.WriteString(renderJSONPaneHTML(r.BaseRaw, r.Diffs, "base"))
			sb.WriteString("</pre></div>")

			sb.WriteString(`<div class="json-pane">`)
			sb.WriteString(`<div class="json-pane-hdr">Exp</div>`)
			sb.WriteString(fmt.Sprintf(`<pre class="json-content" id="%s">`, expID))
			sb.WriteString(renderJSONPaneHTML(r.ExpRaw, r.Diffs, "exp"))
			sb.WriteString("</pre></div>")

			sb.WriteString("</div>") // json-panes
			sb.WriteString("</div>") // detail
		}
		sb.WriteString("</td>")
		sb.WriteString("</tr>\n")
	}

	sb.WriteString("</tbody></table>\n")

	sb.WriteString(`<script>
function toggle(id) {
  var el = document.getElementById(id);
  var btn = el.previousElementSibling;
  if (el.classList.contains('open')) {
    el.classList.remove('open');
    btn.textContent = 'show diff';
  } else {
    el.classList.add('open');
    btn.textContent = 'hide diff';
    if (!el.dataset.sync) {
      el.dataset.sync = '1';
      initSync(id + '-base', id + '-exp');
    }
  }
}
function initSync(aId, bId) {
  var a = document.getElementById(aId);
  var b = document.getElementById(bId);
  if (!a || !b) return;
  var lock = false;
  a.addEventListener('scroll', function() {
    if (lock) return; lock = true; b.scrollTop = a.scrollTop; lock = false;
  });
  b.addEventListener('scroll', function() {
    if (lock) return; lock = true; a.scrollTop = b.scrollTop; lock = false;
  });
}
</script>
</body>
</html>
`)

	return sb.String()
}
