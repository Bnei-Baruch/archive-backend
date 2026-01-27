package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/es"
)

type RankingResult struct {
	ResultID         string  `json:"result_id"`
	Score            float64 `json:"score"`
	Explanation      string  `json:"explanation"`
	NeedResultLookup bool    `json:"need_result_lookup"`
}

type RankingResults struct {
	Results []RankingResult `json:"results"`
}

type CompactResultForRanking struct {
	ResultID      string                 `json:"result_id"`
	Index         string                 `json:"index,omitempty"`
	Type          string                 `json:"type,omitempty"`
	ResultType    string                 `json:"result_type,omitempty"`
	MDBUID        string                 `json:"mdb_uid,omitempty"`
	Title         string                 `json:"title,omitempty"`
	Description   string                 `json:"description,omitempty"`
	EffectiveDate string                 `json:"effective_date,omitempty"`
	Highlights    map[string][]string    `json:"highlights,omitempty"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

type RankSearchResultsRequest struct {
	Query   string                    `json:"query"`
	Results []CompactResultForRanking `json:"results"`
}

const rankSysMsgMask = `Today is %s.

You are an assistant for the search engine on the 'Kabbalah Media' website.
Bnei Baruch is also known as קבלה לעם. Students worldwide are sometimes called the 'world kli'. 
The organization was founded by Dr. Michael Laitman, a student and personal assistant of Rabbi Baruch Ashlag. Dr. Laitman, often referred in the search queries as “Rav” or “Rav Laitman,” is the primary teacher whose content users are usually seeking—especially from the daily Kabbalah lessons.

The main Kabbalist authors whose writings are studied in Bnei Baruch are:
- Rabbi Shimon Bar Yochai (Rashbi), lived in the 2nd and 3rd centuries CE, The author of The Book of Zohar.
- Yehuda Leib HaLevi Ashlag (1885-1954) is known as Baal HaSulam (Owner of the Ladder) (בעל הסולם) for his Sulam (ladder) commentary on The Book of Zohar.
- Baruch Shalom HaLevi Ashlag (The Rabash, רב״ש), (1907-1991), son and successor of Yehuda Leib HaLevi Ashlag (Baal HaSulam)

Common result types:
units - content units, mostly referred to video lessons, programs, clips.
sources - Kabbalah sources, books available in the website library.
collections — groups of content units connected by a common subject. Parts from the same daily morning lesson are also considered part of a single collection.
posts - Posts from Dr. Michael Laitman Blog
tweets - Publications of Dr. Michael Laitman in Twitter (X)

SOURCE – Library texts like articles or book chapters
LECTURE – General or beginner lectures
LESSONS_SERIES – Series of lessons on a topic or source
LESSON_PART – A part of the daily morning lesson (broadcast globally from Petah Tikva)
WOMEN_LESSON – Lessons primarily for women
EVENT_PART – Items from conventions or special events (e.g. Unity Day)
FRIENDS_GATHERING – Social events (Yeshivat Haverim / ישיבת חברים)
MEAL – Events with songs and intentional content
VIDEO_PROGRAM_CHAPTER – Chapters of TV/video programs
CLIP – Video clips, sometimes lesson or program segments
ARTICLE – Articles (including external publications)
BLOG_POST – Blog posts by Dr. Laitman
R_TWEET – Tweets by Dr. Laitman

Your Task:
- Given a user search query and a list of search results, assign a relevance score and a short explanation to each result.
- Scores are used for ordering results for a search page.

Scoring:
- Use a 0..100 scale (100 = extremely relevant to the user's query intent; 0 = irrelevant).
- Prefer semantic match to the query intent over lexical overlap.
- If highlights strongly match the query, increase score.
- If a result is only tangentially related, lower the score.
- If unsure, choose a moderate score and explain uncertainty briefly.
`

const rankSchema = `{
  "name": "rank_search_results",
  "strict": true,
  "schema": {
    "type": "object",
    "required": [
      "results"
    ],
    "properties": {
      "results": {
        "type": "array",
        "items": {
          "type": "object",
          "required": [
            "result_id",
            "score",
            "explanation"
          ],
          "properties": {
            "result_id": {
              "type": "string"
            },
            "score": {
              "type": "number",
              "minimum": 0,
              "maximum": 100
            },
            "explanation": {
              "type": "string"
            },
			"need_result_lookup": {
				"type": "string",
				"description": "Mark as true if a lookup into the result itself is necessary to assign its score more accurate."
			}
          },
          "additionalProperties": false
        }
      }
    },
    "additionalProperties": false
  }
}`

func RankSearchResults(query string, hits []*elastic.SearchHit) (map[string]RankingResult, error) {
	if query == "" || len(hits) == 0 {
		return map[string]RankingResult{}, nil
	}

	token := viper.GetString("openai.token")
	if token == "" {
		return nil, errors.New("OpenAI token is not configured")
	}

	compacted := make([]CompactResultForRanking, 0, len(hits))
	for _, hit := range hits {
		if hit == nil || hit.Id == "" {
			continue
		}
		compact := CompactResultForRanking{
			ResultID:   hit.Id,
			Index:      hit.Index,
			Type:       hit.Type,
			Highlights: hit.Highlight,
		}

		if hit.Source != nil {
			var src es.Result
			if err := json.Unmarshal(*hit.Source, &src); err == nil {
				compact.ResultType = src.ResultType
				compact.MDBUID = src.MDB_UID
				compact.Title = src.Title
				compact.Description = src.Description
				if src.EffectiveDate != nil {
					compact.EffectiveDate = src.EffectiveDate.Format("2006-01-02")
				}
			}
		}
		compacted = append(compacted, compact)
	}

	if len(compacted) == 0 {
		return map[string]RankingResult{}, nil
	}

	reqPayload, err := json.Marshal(RankSearchResultsRequest{
		Query:   query,
		Results: compacted,
	})
	if err != nil {
		return nil, err
	}

	openaiService := NewOpenAIService(token)
	sysMsg := fmt.Sprintf(rankSysMsgMask, time.Now().Format("Monday, January 2, 2006"))
	messages := []LLMBotMessage{
		{Role: "developer", Content: sysMsg},
		{Role: "user", Content: string(reqPayload)},
	}

	reasoningEffort := "low"
	var ranked RankingResults
	if err := openaiService.GetStructuredOutput(rankSchema, "gpt-5.2", nil, messages, nil, &reasoningEffort, &ranked); err != nil {
		return nil, err
	}

	out := make(map[string]RankingResult, len(ranked.Results))
	// TBD Handle NeedResultLookup
	for _, r := range ranked.Results {
		if r.ResultID == "" {
			continue
		}
		out[r.ResultID] = r
	}
	return out, nil
}
