package search_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/search/searchtest"
	"github.com/stretchr/testify/suite"
)

type TokenSuite struct {
	searchtest.SearchSuite
}

func TestToken(t *testing.T) {
	suite.Run(t, new(TokenSuite))
}

func (suite *TokenSuite) True(match bool, values []search.VariableValue, tokensContinue []*search.TokenNode, err error) {
	r := require.New(suite.T())
	r.Nil(err)
	r.Nil(tokensContinue)
	r.True(match)
}

func (suite *TokenSuite) TrueWithPrefix(match bool, values []search.VariableValue, tokensContinue []*search.TokenNode, err error, expectedOrigPhrases []string) {
	r := require.New(suite.T())
	r.Nil(err)
	r.True(match)
	continuePhrases := search.TokenNodesToPhrases(tokensContinue, make(search.VariablesByName), true /*=reduceVariables*/)
	fmt.Printf("continuePhrases: %+v\n", continuePhrases)

	continuePhrasesStrings := []string{}
	for _, phrase := range continuePhrases {
		continuePhrasesStrings = append(continuePhrasesStrings, phrase.OriginalJoin())
	}
	r.Equal(strings.Join(expectedOrigPhrases, "|"), strings.Join(continuePhrasesStrings, "|"))
	r.Equal(len(expectedOrigPhrases), len(continuePhrases))
}

func (suite *TokenSuite) TestTokenNodesToPhrases1() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"one,two words"}))
	a := suite.Tokens("this is one thing for me.", "en")
	phrases := search.TokenNodesToPhrases(a, make(search.VariablesByName), true /*=reduceVariables*/)
	r.Equal(2, len(phrases))
	r.Equal("this is one thing for me.", phrases[0].OriginalJoin())
	// Note we don't know how to bring synonym original phrase, just the already reduces.
	// For now leave as is...
	// Should be : "this is two words thing for me."
	r.Equal("this is two word thing for me.", phrases[1].OriginalJoin())

	r.Nil(suite.UpdateSynonyms([]string{
		"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום",
	}))
	a = suite.Tokens("שיעורי הרב\"ש.", "he")
	for _, phrase := range search.TokenNodesToPhrases(a, make(search.VariablesByName), true /*=reduceVariables*/) {
		r.Equal("שיעורי הרב\"ש.", phrase.OriginalJoin())
	}
}

func UnorderedEqual(a, b []string) bool {
	m := make(map[string]int)
	notMatched := []string{}
	for i := range a {
		m[a[i]]++
	}
	for i := range b {
		if _, ok := m[b[i]]; !ok {
			notMatched = append(notMatched, fmt.Sprintf("Could not find [%s] in |a|", b[i]))
		} else {
			m[b[i]]--
		}
	}
	for a_i, count := range m {
		if count > 0 {
			notMatched = append(notMatched, fmt.Sprintf("%d missing [%s] in |a|", count, a_i))
		} else if count < 0 {
			notMatched = append(notMatched, fmt.Sprintf("%d missing [%s] in |b|", -count, a_i))
		}
	}
	if len(notMatched) > 0 {
		fmt.Printf("|a|: %s\n|b|: %s\n%s\n", strings.Join(a, ","), strings.Join(b, ","), strings.Join(notMatched, "\n"))
	}
	return len(notMatched) == 0
}

func (suite *TokenSuite) TestTokenNodesToPhrasesVariables() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"one,two words"}))
	translations := suite.MakeTranslations("$Var", "en", map[string][]string{
		"one": []string{"one", "only one"},
		"two": []string{"only two", "too you", "two of you"},
	})
	varVariable := search.MakeFileVariable("$Var", "en", translations)
	variables := make(search.VariablesByName)
	variables["$Var"] = &varVariable
	phrase := suite.TokensWithVariables("this is one $Var thing for me.", "en", variables)
	phrases := search.TokenNodesToPhrases(phrase, variables, true /*=reduceVariables*/)
	actualOriginal := []string(nil)
	actualPhrases := []string(nil)
	for i := range phrases {
		actualOriginal = append(actualOriginal, phrases[i].OriginalJoin())
		actualPhrases = append(actualPhrases, phrases[i].Join(" "))
	}

	expectedOriginal := []string{
		"this is one one thing for me.",
		"this is one only one thing for me.",
		"this is one only two thing for me.",
		"this is one only two word thing for me.",
		"this is one too you thing for me.",
		"this is one two of you thing for me.",
		"this is one two word thing for me.",
		"this is two word one thing for me.",
		"this is two word only one thing for me.",
		"this is two word only two thing for me.",
		"this is two word only two word thing for me.",
		"this is two word too you thing for me.",
		"this is two word two of you thing for me.",
		"this is two word two word thing for me.",
	}
	r.True(UnorderedEqual(actualOriginal, expectedOriginal))

	expectedPhrases := []string{
		"on on thing me",
		"on onli on thing me",
		"on onli two thing me",
		"on onli two word thing me",
		"on too you thing me",
		"on two word thing me",
		"on two you thing me",
		"two word on thing me",
		"two word onli on thing me",
		"two word onli two thing me",
		"two word onli two word thing me",
		"two word too you thing me",
		"two word two word thing me",
		"two word two you thing me",
	}
	r.True(UnorderedEqual(actualPhrases, expectedPhrases))
}

func (suite *TokenSuite) TestTokenNodesToPhrasesVariables2() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"two,ttt"}))
	translations := suite.MakeTranslations("$Var", "en", map[string][]string{
		"one": []string{"three"},
		//"two": []string{"only two", "too you", "two of you"},
	})
	varVariable := search.MakeFileVariable("$Var", "en", translations)
	variables := make(search.VariablesByName)
	variables["$Var"] = &varVariable
	phrase := suite.TokensWithVariables("two $Var second.", "en", variables)
	phrases := search.TokenNodesToPhrases(phrase, variables, true /*=reduceVariables*/)
	actualOriginal := []string(nil)
	actualPhrases := []string(nil)
	for i := range phrases {
		actualOriginal = append(actualOriginal, phrases[i].OriginalJoin())
		actualPhrases = append(actualPhrases, phrases[i].Join(" "))
	}

	expectedOriginal := []string{
		"ttt three second.",
		"two three second.",
	}
	r.True(UnorderedEqual(actualOriginal, expectedOriginal))

	expectedPhrases := []string{
		"ttt three second",
		"two three second",
	}
	r.True(UnorderedEqual(actualPhrases, expectedPhrases))
}

func (suite *TokenSuite) TestMatchTokensEn() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"one,two words"}))
	a := suite.Tokens("this is one thing for me.", "en")
	b := suite.Tokens("this is two words thing for me.", "en")
	suite.True(search.TokensSingleMatch(a, b, false, make(search.VariablesByName)))
}

func (suite *TokenSuite) TestTokensNodesToPhrasesWithSynonyms() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"חיים קהילתיים,חיי קהילה,קהילה"}))
	a := suite.Tokens("קהילה יהודית מונטריי", "he")
	phrases := search.TokenNodesToPhrases(a, make(search.VariablesByName), true /*=reduceVariables*/)
	r.Equal(3, len(phrases))
	r.Equal("חיי קהילה יהודית מונטריי", phrases[0].OriginalJoin())
	r.Equal("חיים קהילתיים יהודית מונטריי", phrases[1].OriginalJoin())
	r.Equal("קהילה יהודית מונטריי", phrases[2].OriginalJoin())
}

// This test will fail due to synonyms being before Hunspell
// Commenting out until this is fixed in elastic.
//func (suite *TokenSuite) TestMatchTokensHe1() {
//	r := require.New(suite.T())
//	r.Nil(suite.UpdateSynonyms([]string{"זוהר לעם,זוהר,ספר הזוהר,הזוהר"}))
//	a := suite.Tokens("מבוא לספר הזוהר", "he")
//	b := suite.Tokens("מבוא לזוהר", "he")
//	r.True(search.TokensSingleMatch(a, b))
//}

func (suite *TokenSuite) TestMatchTokensHe2() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"ניסיון,ניב טוב", "ספר הזוהר,זוהר,לספר הזוהר,לספר זוהר,לזוהר"}))
	a := suite.Tokens("ניסיון מבוא לספר הזוהר", "he")
	b := suite.Tokens("ניב טוב מבוא לזוהר", "he")
	suite.True(search.TokensSingleMatch(a, b, false, make(search.VariablesByName)))
}

func (suite *TokenSuite) TestTokenCache() {
	variables := make(search.VariablesByName)
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{}))
	tc := search.MakeTokensCache(2)
	// Cache empty.
	r.False(tc.Has("phrase one", "en"))
	r.Nil(tc.Get("phrase one", "en"))
	// Set one and two phrases.
	a := suite.Tokens("phrase one.", "en")
	tc.Set("phrase one", "en", a)
	b := suite.Tokens("phrase two.", "en")
	tc.Set("phrase two", "en", b)
	// Check both exist.
	r.True(tc.Has("phrase one", "en"))
	suite.True(search.TokensSingleMatch(tc.Get("phrase one", "en"), a, false, variables))
	r.True(tc.Has("phrase two", "en"))
	suite.True(search.TokensSingleMatch(tc.Get("phrase two", "en"), b, false, variables))
	// Add third phrase.
	c := suite.Tokens("phrase three", "en")
	tc.Set("phrase three", "en", c)
	// Check first is out, second and third are in.
	r.False(tc.Has("phrase one", "en"))
	r.Nil(tc.Get("phrase one", "en"))
	r.True(tc.Has("phrase three", "en"))
	suite.True(search.TokensSingleMatch(tc.Get("phrase three", "en"), c, false, variables))
	r.True(tc.Has("phrase two", "en"))
	suite.True(search.TokensSingleMatch(tc.Get("phrase two", "en"), b, false, variables))
	// Now two is fresher then three. Add fourth phrase.
	d := suite.Tokens("phrase four", "en")
	tc.Set("phrase four", "en", d)
	// Make sure third is out and second and fourth are in.
	r.False(tc.Has("phrase three", "en"))
	r.Nil(tc.Get("phrase three", "en"))
	r.True(tc.Has("phrase two", "en"))
	suite.True(search.TokensSingleMatch(tc.Get("phrase two", "en"), b, false, variables))
	r.True(tc.Has("phrase four", "en"))
	suite.True(search.TokensSingleMatch(tc.Get("phrase four", "en"), d, false, variables))
}

//func (suite *TokenSuite) TestTokensMerge() {
//	r := require.New(suite.T())
//	a := suite.Tokens("me is the best", "en")
//	b := suite.Tokens("me is the worst", "en")
//	c := search.MergeTokenGraphs(a, b)
//
//	d := suite.Tokens("best", "en")
//	s, err := search.TokensSingleSearch(d, c)
//	r.Nil(err)
//	r.Equal("me is the best", s)
//
//	e := suite.Tokens("worst", "en")
//	s, err = search.TokensSingleSearch(e, c)
//	r.Nil(err)
//	r.Equal("me is the worst", s)
//
//	r.Nil(suite.UpdateSynonyms([]string{"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום"}))
//
//	a = suite.Tokens("שיעורי הרב\"ש", "he")
//	b = suite.Tokens("שיעורי רב\"ש", "he")
//	c = suite.Tokens("שיעורים עם הרב\"ש", "he")
//
//	d = search.MergeTokenGraphs(c, a)
//	e = search.MergeTokenGraphs(d, b)
//
//	f := suite.Tokens("הרבש", "he")
//	s, err = search.TokensSingleSearch(f, e)
//	r.Nil(err)
//	r.Equal("שיעורי הרב\"ש", s)
//
//	r.Nil(suite.UpdateSynonyms([]string{
//		"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום",
//		"בעל הסולם,בעהס,רב יהודה אשלג,רב יהודה הלוי אשלג,רב יהודה ליב אשלג,רב יהודה ליב הלוי אשלג,רב אשלג,יהודה אשלג",
//	}))
//	toMerge := []string{
//		"בעל הסולם",
//		"רב\"ש",
//	}
//	a = suite.Tokens(toMerge[0], "he")
//	for _, phrase := range toMerge[1:] {
//		a = search.MergeTokenGraphs(a, suite.Tokens(phrase, "he"))
//	}
//	log.Infof("a:\n%s", search.TokenNodesToString(a))
//
//	b = suite.Tokens("רבש", "he")
//	s, err = search.TokensSingleSearch(b, a)
//	r.Nil(err)
//	// Note that we don't know how to match the correct synonym,
//	// we just match something according to alphabetical order.
//	// We should try to match according to the requested original form.
//	r.Equal("רב ברוך שלום הלוי אשלג", s)
//
//	b = suite.Tokens("רב ברוך", "he")
//	s, err = search.TokensSingleSearch(b, a)
//	r.Nil(err)
//	r.Equal("רב ברוך שלום הלוי אשלג", s)
//}

func CompareVariableMaps(expected, actual map[string][]string) bool {
	same := true
	for variable, values := range expected {
		if actualValues, ok := actual[variable]; ok {
			valuesEqual := reflect.DeepEqual(values, actualValues)
			if !valuesEqual {
				fmt.Printf("Variable %s differ on values. Expected: %+v, Actual: %+v\n", variable, values, actualValues)
			}
			same = same && valuesEqual
		} else {
			same = false
			fmt.Printf("Variable %s don't exist in actual.\n", variable)
		}
	}
	for variable, _ := range actual {
		if _, ok := expected[variable]; !ok {
			fmt.Printf("Variable %s don't exist in expected.\n", variable)
			same = false
		}
	}
	return same
}

func CompareVariablesByPhrase(expected, actual search.VariablesByPhrase) bool {
	same := true
	for phrase, vMap := range expected {
		if actualVMap, ok := actual[phrase]; !ok {
			same = false
			fmt.Printf("Phrase %s does not exist in actual\n", phrase)
		} else {
			same = same && CompareVariableMaps(vMap, actualVMap)
		}
	}
	for phrase, _ := range actual {
		if _, ok := expected[phrase]; !ok {
			fmt.Printf("Phrase %s does not exist in expected\n", phrase)
			same = false
		}
	}
	return same
}

func MakeVariablesByPhrase(phrases ...[]string) search.VariablesByPhrase {
	ret := make(search.VariablesByPhrase)
	for j := range phrases {
		phrase := phrases[j][0]
		parts := phrases[j][1:]
		vMap := make(map[string][]string)
		key := ""
		for i := range parts {
			part := parts[i]
			if strings.HasPrefix(part, "$") {
				key = part
			} else {
				vMap[key] = append(vMap[key], part)
			}
		}
		ret[phrase] = vMap
	}
	return ret
}

func (suite *TokenSuite) TestTokensSingleSearch() {
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{}))
	a := suite.Tokens("me is th rest", "en")
	b := suite.Tokens("some thing interesting", "en")
	s, err := search.TokensSingleSearch(a, b, make(search.VariablesByName))
	r.Nil(err)
	r.True(CompareVariablesByPhrase(MakeVariablesByPhrase([]string{"some thing interesting"}), s))

	a = suite.Tokens("רות נ", "he")
	b = suite.Tokens("סדרות לימוד נבחרות", "he")
	s, err = search.TokensSingleSearch(a, b, make(search.VariablesByName))
	r.Nil(err)
	r.True(CompareVariablesByPhrase(MakeVariablesByPhrase([]string{"סדרות לימוד נבחרות"}), s))

	a = suite.Tokens("רות נב", "he")
	b = suite.Tokens("סדרות לימוד נבחרות", "he")
	s, err = search.TokensSingleSearch(a, b, make(search.VariablesByName))
	r.Nil(err)
	r.True(CompareVariablesByPhrase(MakeVariablesByPhrase([]string{"סדרות לימוד נבחרות"}), s))

	a = suite.Tokens("רות נ", "he")
	b = suite.Tokens("סדרות לימוד נבחרות", "he")
	s, err = search.TokensSingleSearch(a, b, make(search.VariablesByName))
	r.Nil(err)
	r.True(CompareVariablesByPhrase(MakeVariablesByPhrase([]string{"סדרות לימוד נבחרות"}), s))

	r.Nil(suite.UpdateSynonyms([]string{}))
	a = suite.Tokens("ברוך", "he")
	b = suite.Tokens("ברוך", "he")
	s, err = search.TokensSingleSearch(a, b, make(search.VariablesByName))
	r.Nil(err)
	r.True(CompareVariablesByPhrase(MakeVariablesByPhrase([]string{"ברוך"}), s))

	r.Nil(suite.UpdateSynonyms([]string{"רבש,רב ברוך שלום הלוי אשלג,רב ברוך שלום,הרב ברוך שלום הלוי אשלג,הרב ברוך שלום"}))
	a = suite.Tokens("שיעורי", "he")
	b = suite.Tokens("שיעורי רב\"ש", "he")
	s, err = search.TokensSingleSearch(a, b, make(search.VariablesByName))
	r.Nil(err)
	r.True(CompareVariablesByPhrase(MakeVariablesByPhrase([]string{"שיעורי רב ברוך שלום הלוי אשלג"}), s))
}

func (suite *TokenSuite) TestMatchTokensWithSynonymsHe() {
	log.Info("TestMatchTokensWithSynonymsHe")
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"רב לייטמן,רב דר מיכאל לייטמן,רב מיכאל לייטמן"}))
	a := suite.Tokens("רב לייטמן", "he")
	b := suite.Tokens("רב דר מיכאל לייטמן", "he")
	suite.True(search.TokensSingleMatch(a, b, false, make(search.VariablesByName)))
}

func (suite *TokenSuite) TestSortWorks() {
	log.Info("TestSortWorks")
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"aaaa,bbbb,cccc,dddd"}))
	a := suite.Tokens("dddd cccc aaaa bbbb", "he")
	phrases := search.TokenNodesToPhrases(a, make(search.VariablesByName), true /*=reduceVariables*/)
	r.Equal(256, len(phrases))
	r.Equal("aaaa aaaa aaaa aaaa", phrases[0].Join(" "))
	r.Equal("aaaa aaaa aaaa bbbb", phrases[1].Join(" "))
	r.Equal("cccc aaaa aaaa aaaa", phrases[128].Join(" "))
	r.Equal("dddd dddd dddd dddd", phrases[255].Join(" "))
}

//func (suite *TokenSuite) TestHaklatot() {
//	log.Info("TestHaklatot")
//	r := require.New(suite.T())
//	r.Nil(suite.UpdateSynonyms([]string{}))
//
//	toMerge := []string{
//		"הקלטות רב\"ש",
//		"הקלטות רבש",
//		"שיעורי הרב\"ש",
//		"שיעורי רב\"ש",
//		"שיעורים עם הרב\"ש",
//	}
//	a := suite.Tokens(toMerge[0], "he")
//	for _, phrase := range toMerge[1:] {
//		a = search.MergeTokenGraphs(a, suite.Tokens(phrase, "he"))
//	}
//	log.Infof("a:\n%s", search.TokenNodesToString(a))
//	r.True(false)
//}

func (suite *TokenSuite) TestPrefixMatch() {
	log.Info("TestPrefixMatch")
	r := require.New(suite.T())
	r.Nil(suite.UpdateSynonyms([]string{"one,two,three"}))
	tokens := suite.Tokens("let's try to match two prefixes of a long.", "en")
	patterns := suite.Tokens("lets try to match three prefixes of a long query.", "en")
	match, values, tokensContinue, err := search.TokensSingleMatch(tokens, patterns, true, make(search.VariablesByName))
	suite.TrueWithPrefix(match, values, tokensContinue, err, []string{" query."})
}
