package search

import (
	elastic "gopkg.in/olivere/elastic.v6"
)

// fromOlivereResponses converts a slice of olivere SearchResult pointers to our shared type.
func fromOlivereResponses(responses []*elastic.SearchResult) []*SearchResult {
	result := make([]*SearchResult, len(responses))
	for i, r := range responses {
		result[i] = fromOlivereResult(r)
	}
	return result
}

// fromOlivereResult converts an olivere *SearchResult to our shared *SearchResult.
func fromOlivereResult(r *elastic.SearchResult) *SearchResult {
	if r == nil {
		return nil
	}
	return &SearchResult{
		Hits:    fromOlivereHits(r.Hits),
		Suggest: fromOlivereSuggest(r.Suggest),
	}
}

// fromOlivereHits converts an olivere *SearchHits to our shared *SearchHits.
func fromOlivereHits(hits *elastic.SearchHits) *SearchHits {
	if hits == nil {
		return nil
	}
	result := &SearchHits{
		TotalHits: hits.TotalHits,
		MaxScore:  hits.MaxScore,
		Hits:      make([]*SearchHit, 0, len(hits.Hits)),
	}
	for _, h := range hits.Hits {
		result.Hits = append(result.Hits, fromOlivereHit(h))
	}
	return result
}

// fromOlivereHit converts an olivere *SearchHit to our shared *SearchHit.
func fromOlivereHit(h *elastic.SearchHit) *SearchHit {
	if h == nil {
		return nil
	}
	hit := &SearchHit{
		Index:       h.Index,
		Type:        h.Type,
		ID:          h.Id,
		Uid:         h.Uid,
		Score:       h.Score,
		Source:      h.Source,
		Highlight:   fromOlivereHighlight(h.Highlight),
		Explanation: fromOlivereExplanation(h.Explanation),
	}
	if h.InnerHits != nil {
		hit.InnerHits = make(map[string]*SearchHitInnerHits, len(h.InnerHits))
		for k, v := range h.InnerHits {
			hit.InnerHits[k] = &SearchHitInnerHits{Hits: fromOlivereHits(v.Hits)}
		}
	}
	return hit
}

// fromOlivereHighlight converts an olivere SearchHitHighlight to our shared type.
func fromOlivereHighlight(h elastic.SearchHitHighlight) SearchHitHighlight {
	if h == nil {
		return nil
	}
	result := make(SearchHitHighlight, len(h))
	for k, v := range h {
		result[k] = v
	}
	return result
}

// fromOlivereExplanation converts an olivere *SearchExplanation to our shared type.
func fromOlivereExplanation(exp *elastic.SearchExplanation) *SearchExplanation {
	if exp == nil {
		return nil
	}
	result := &SearchExplanation{
		Value:       exp.Value,
		Description: exp.Description,
	}
	for i := range exp.Details {
		result.Details = append(result.Details, *fromOlivereExplanation(&exp.Details[i]))
	}
	return result
}

// fromOlivereSuggest converts an olivere SearchSuggest to our shared SearchSuggest.
func fromOlivereSuggest(suggest elastic.SearchSuggest) SearchSuggest {
	if suggest == nil {
		return nil
	}
	result := make(SearchSuggest, len(suggest))
	for k, suggestions := range suggest {
		converted := make([]SearchSuggestion, len(suggestions))
		for i, s := range suggestions {
			opts := make([]SearchSuggestionOption, len(s.Options))
			for j, opt := range s.Options {
				opts[j] = SearchSuggestionOption{
					Text:   opt.Text,
					Score:  opt.Score,
					Freq:   opt.Freq,
					Source: opt.Source,
				}
			}
			converted[i] = SearchSuggestion{
				Text:    s.Text,
				Offset:  s.Offset,
				Length:  s.Length,
				Options: opts,
			}
		}
		result[k] = converted
	}
	return result
}
