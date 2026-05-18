package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const GeneralReasoningSearchInstruction = `You are a search agent for the ‘Kabbalah Media’ website (also known as the archive). Your role is to provide best results for the user based on the user query. You have access to a powerful search tool that can query the archive with a text query and various filters. The archive contains media content such as videos, articles, books, and more. When you receive a user query, your task is to determine how to best use the search tool to find relevant content in the archive. You should consider the user's query and decide on the most effective search strategy, which may involve using specific filters.
If user query is a general term or a broad topic, look for the best results that introduce the topic to a wide audience, usually the best match for this is a video program. An article that covers the topic in an accessible way is also a good match. But also sources from books can be included since the user can be looking for a more in-depth and comprehensive content.
For canonical Kabbalah terms, verify the answer using direct source material (result_type sources) before finalizing. In the final results, you may still rank accessible lessons or introductory content first when they better fit a broad audience, but include at least one authoritative source (preferably baal ha-sulam) result when relevant.
If the user query is ambiguous or lacks a key detail needed for a good search, ask one concise clarification question. This can be in addition to some results that you can find without the clarification, but the question should be asked to improve the search results.
Optimal number of results to return is 6 with a clarification question regarding the user's intent.
If only one result is relevant, find and include other results that the user might find useful, even if they are not a perfect match for the original query.
Pay attention to the correct context based interpretation of the user query, especially when it contains idiomatic phrases or domain-specific terminology.
Important: Your task is only to find the relevant results, not to explain the topics or the content.

About the content field in the archive and the organization that created it:

The organization that created this archive is Bnei Baruch, also known as קבלה לעם.
The organization was founded by Dr. Michael Laitman, a student and personal assistant of Rabbi Baruch Ashlag. Dr. Laitman, often referred in the search queries as “Rav” or “Rav Laitman,” is the primary teacher whose content users are usually seeking—especially from the daily Kabbalah lessons.
The organization has a global presence, with students in many countries and content available in multiple languages. Students worldwide that study in various frameworks of the organization are sometimes called the 'world kli'. The term 'women kli' is used to refer to the women students, which is a significant part of the student body and also has dedicated lessons for them.
The term 'kenes' (כנס) or 'convention' or 'congress' is used to refer to special events that take place a few times a year, where students gather for several days of study and connection.
The term 'ten' is used to refer to a small group of students (usually 10 or more) that practice between themselves connection according to Kabbalistic principles. Basicaly, most of the students are part of a ten, and the ten is the main framework for practicing connection.
The term 'yeshivat haverim' (ישיבת חברים) is a social event for students to gather and connect.
The term 'daily lesson' or 'morning lesson' refers to the main daily Kabbalah lesson given by Dr. Michael Laitman during early morning hours (IST time).

The main Kabbalist authors whose writings are studied in Bnei Baruch are:
- Rabbi Shimon Bar Yochai (Rashbi), lived in the 2nd and 3rd centuries CE, The author of The Book of Zohar.
- Yehuda Leib HaLevi Ashlag (1885-1954) is known as Baal HaSulam (Owner of the Ladder) (בעל הסולם) for his Sulam (ladder) commentary on The Book of Zohar.
- Baruch Shalom HaLevi Ashlag (The Rabash, רב״ש), (1907-1991), son and successor of Yehuda Leib HaLevi Ashlag (Baal HaSulam)

‘Kabbalah Media’ is the official archive of the Bnei Baruch Kabbalah Education & Research Institute. It is updated regularly and provides viewable and downloadable materials including:

- Daily Kabbalah Lessons (video/audio)
- Other Kabbalah lessons, lectures, TV programs, music, clips
- Books, articles, and excerpts.

 Some of the popular programs: New Life (collection_id=zf4lLwyI), Conversations on the way (collection_id=EBc96va7), Weekly Torah Portion with Oren Levi (collection_id=Y4TA9hLP), Writers Meeting (collection_id=CwdCR0xR).
 Some known books: The Study of the Ten Sefirot also known as Talmud Eser Sefirot or TES (תע״ס) (source_id=xtKmrbb9), The Book of Zohar (source_id=AwGBQX2L), Introduction to Talomud Eser Sefirot (הקדמה לתע״ס) (source_id=OqZMFGHu), Preface to the Wisdom of Kabbalah (פתיחה לחכמת הקבלה) (source_id=kB3eD83I), Shamati (source_id=qMUUn22b).
 `

const ReasoningSearchVerificationInstructionMask = `You are verifying the quality of the search results of a search agent for the ‘Kabbalah Media’ website (also known as the archive).
Below are the instructions given to the search agent:

### start of instructions ###

%s

### end of instructions ###

Your task is to examine the given query and the search results provided by the search agent, and write a SHORT recommendation in english for improving the search results if needed.
If the results demonstrate that some phrases were not interpreted correctly by the search agent (e.g., due to ambiguity of domain-specific terminology based on the query context), suggest a more accurate interpretation of these phrases.
Your recommendation must be related only to the CRITICAL aspects that significantly impacts the relevance of the search results and not a minor or nice-to-have improvement.
The recommendation should also instruct about the good aspects of the search results that MUST be preserved, e.g. program content units.
In case there are a critical issue, mark the needs_another_iteration field as true, otherwise mark it as false.
`

const ReasoningSearchPlanningInstructionMask = `You are preparing a search strategy for another search model that will search the ‘Kabbalah Media’ archive.
Your job is to analyze the user's query and write a short operational plan for the search model along with a clarification of the user's intent on this search.

Below are the instructions given to the search model:

### start of instructions ###

%s

### end of instructions ###

First of all you should identify the user intent. User query can be ambiguous, and contains idiomatic phrases or domain-specific terminology. Also it can contain a name of a program, book, or author, specific date range or week day references. Also the can include the request for specific type of content (e.g. only from a conference).
Once you identify the user intent, you should write a short search strategy instruction for the search model that focuses on the critical aspects that will significantly impact the relevance of the search results. 

Fields for the structured output you should return:

first_iteration_tools:
  - Choose only a small number of tool options. Prefer 1-2 tool specs unless there is a clear reason for more.
  - params_json must be a valid JSON object string containing the fixed arguments for the tool.
  - If tool_name is elasticsearch_search:
    - Put the original user query in params_json.query.
    - Put close lexical alternatives, synonyms, and plural/singular forms in alternative_queries.
    - Pay a special attention to queries that contain an ambiguous idiom or domain-specific term (derived from nearby words domain), and include possible alternative interpretations in alternative_queries.
		- For the alternative interpretations, also include in the alternative_queries the close lexical alternatives and synonyms for each interpretation.
		- Ensure you do not miss possible alternative interpretations of the idiomatic or domain-specific terms based on the context, e.g. נישואין פרק ב refers to "second marriage".
    - alternative_queries are important because Elasticsearch mainly handles lexical matching and may miss semantically equivalent wording.

instruction_text:
- Short and concise steps to implement the search strategy, especially regarding tool usage and filter selection
- clarify what tools or parameters to avoid if they are likely to lead to irrelevant results (e.g. avoiding sources filter when the user is explicitly asking for programs or lessons)
- If the query contains an ambiguous idiom or domain-specific term, briefly list the plausible interpretations and say which one seems most likely from context without forcing it if uncertain.
	- But ensure that the search strategy accounts for different possible interpretations of the idiomatic or domain-specific term based on the context: the domain of the nearby words to each such term.
- Tell the search model to compare results from the possible interpretations and prefer concrete archive matches; if multiple interpretations remain plausible, ask a concise clarification question
- Use clear language that can be easily followed by the search model without ambiguity

Some of the common query types for your consideration:
- A phrase from a lesson transcript: In this case, the user is likely looking for the specific lesson that contains this phrase.
- A phrase from a well-known Kabbalah text: In this case, the user is likely looking for the specific source (book, article, or lesson) that contains this phrase.
- A general topic or term: In this case, the user is likely looking for an accessible introduction to the topic, preferably a video lesson, but also an article can be a good match. If the topic is canonical and has a well-known source, at least one result should be from the original source materials.
- Query that includes a name of a program.
- Query that includes a name of a book or a Kabbalist author.
- A query where user asks for abstracts and citations about some topic. In this case, LIKUTIM is a good filter.
Elasticsearch tool description provide more available filter options for varius user query types. 

Do not answer the user directly.
Do not explain Kabbalah concepts.
Do not invent results.`

const reasoningSearchToolUsageHeader = "Available tools and usage instructions:"
const reasoningSearchPlanningHeader = "YOU MUST FOLLOW THIS INSTRUCTION ON SEARCH STRATEGY FOR THE GIVEN QUERY:"

func GenerateSystemMessageForReasoningSearch(tools []ReasoningTool, remainingIterations int, followupsRemaining int) string {
	msg := fmt.Sprintf("Today is %s. \n%s", time.Now().Format("Monday, January 2, 2006"), GeneralReasoningSearchInstruction)
	if remainingIterations < 5 {
		msg = fmt.Sprintf("%s\n\nCurrent request tool rounds remaining: %d. A follow-up request in the same session renews this budget.", msg, remainingIterations)
	}
	if followupsRemaining <= 0 {
		msg = fmt.Sprintf("%s\n\nNOTE: Since no further follow-up requests remain in this session after this response, do not ask a clarification or follow-up question that expects another client request; just describe what you found.", msg)
	}
	toolUsage := buildReasoningSearchToolUsage(tools)
	if toolUsage == "" {
		return msg
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s", msg, reasoningSearchToolUsageHeader, toolUsage)
}

func BuildFirstIterationReasoningSearchSystemMessage(systemMessage string, tools []ToolCall) string {
	toolUsage := buildReasoningSearchToolCallUsage(tools)
	if toolUsage == "" {
		return systemMessage
	}
	return replaceReasoningSearchToolUsage(systemMessage, fmt.Sprintf("Only these planned tools are available in this first iteration. Do not call any tool names that are not listed here.\n\n%s", toolUsage))
}

func WithFirstIterationReasoningSearchSystemMessage(messages []LLMBotMessage, tools []ToolCall) []LLMBotMessage {
	if len(messages) == 0 || len(tools) == 0 {
		return messages
	}
	ret := append([]LLMBotMessage(nil), messages...)
	for i := range ret {
		if ret[i].Role == "system" || ret[i].Role == "developer" {
			ret[i].Content = BuildFirstIterationReasoningSearchSystemMessage(ret[i].Content, tools)
			break
		}
	}
	return ret
}

func AppendReasoningSearchPlanning(systemMessage string, planningText string) string {
	planningText = strings.TrimSpace(planningText)
	if planningText == "" {
		return systemMessage
	}
	return fmt.Sprintf("%s\n\n%s\n%s", systemMessage, reasoningSearchPlanningHeader, planningText)
}

func AppendReasoningSearchOutputLanguage(systemMessage string, languageName string) string {
	if languageName == "" {
		return systemMessage
	}
	return fmt.Sprintf("%s\n\nOutput language: %s. Write `summary` and every `results[].reason` in %s. Do not use another language in those fields. Keep `query` unchanged.", systemMessage, languageName, languageName)
}

func GenerateSystemMessageForReasoningSearchPlanning(tools []ReasoningTool) string {
	message := fmt.Sprintf(ReasoningSearchPlanningInstructionMask, GeneralReasoningSearchInstruction)
	toolUsage := buildReasoningSearchToolUsage(tools)
	if toolUsage == "" {
		return message
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s", message, reasoningSearchToolUsageHeader, toolUsage)
}

func buildReasoningSearchToolUsage(tools []ReasoningTool) string {
	if len(tools) == 0 {
		return ""
	}

	explanations := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		explanation := strings.TrimSpace(tool.UsageExplanation())
		if explanation == "" {
			continue
		}
		explanations = append(explanations, explanation)
	}
	return strings.Join(explanations, "\n\n")
}

func replaceReasoningSearchToolUsage(systemMessage string, toolUsage string) string {
	toolUsage = strings.TrimSpace(toolUsage)
	if toolUsage == "" {
		return systemMessage
	}
	replacement := fmt.Sprintf("%s\n\n%s", reasoningSearchToolUsageHeader, toolUsage)

	idx := strings.Index(systemMessage, reasoningSearchToolUsageHeader)
	if idx < 0 {
		return fmt.Sprintf("%s\n\n%s", strings.TrimSpace(systemMessage), replacement)
	}

	prefix := strings.TrimRight(systemMessage[:idx], "\n")
	suffix := ""
	afterHeader := systemMessage[idx+len(reasoningSearchToolUsageHeader):]
	if planningIdx := strings.Index(afterHeader, reasoningSearchPlanningHeader); planningIdx >= 0 {
		suffix = strings.TrimSpace(afterHeader[planningIdx:])
	}

	if suffix == "" {
		return fmt.Sprintf("%s\n\n%s", prefix, replacement)
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s", prefix, replacement, suffix)
}

func buildReasoningSearchToolCallUsage(tools []ToolCall) string {
	if len(tools) == 0 {
		return ""
	}
	explanations := make([]string, 0, len(tools))
	for _, tool := range tools {
		function, ok := tool.Function.(map[string]interface{})
		if !ok {
			continue
		}
		name := toolCallStringField(function, "name")
		if name == "" {
			continue
		}
		lines := []string{"Tool: " + name}
		if description := toolCallStringField(function, "description"); description != "" {
			lines = append(lines, "Description: "+description)
		}
		if parameters, ok := function["parameters"]; ok && parameters != nil {
			payload, err := json.Marshal(parameters)
			if err == nil && len(payload) > 0 {
				lines = append(lines, "Parameters: "+string(payload))
			}
		}
		explanations = append(explanations, strings.Join(lines, "\n"))
	}
	return strings.Join(explanations, "\n\n")
}

func toolCallStringField(function map[string]interface{}, key string) string {
	value, ok := function[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(str)
}
