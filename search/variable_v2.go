package search

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Bnei-Baruch/archive-backend/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/sqlboiler/queries"
)

// Translations language => value => phrases
type TranslationsV2 = map[string]map[string][]string

// Map from variable => language => value => phrases
type VariablesV2 = map[string]TranslationsV2

const (
	START_YEAR = 1996
)

func MakeYearVariablesV2() map[string][]string {
	ret := make(map[string][]string)
	year := START_YEAR
	nowYear := time.Now().Year()
	for year <= nowYear {
		yearStr := fmt.Sprintf("%d", year)
		ret[yearStr] = []string{yearStr}
		year++
	}
	return ret
}

func YearScorePenalty(vMap map[string][]string) float64 {
	if yearStrs, ok := vMap[consts.VAR_YEAR]; ok {
		maxRet := 0.0
		for _, yearStr := range yearStrs {
			nowYear := time.Now().Year()
			if year, err := strconv.Atoi(yearStr); err != nil || year >= nowYear {
				return 1.0
			} else {
				ret := 0.3*(1-float64(nowYear-year)/float64(nowYear-START_YEAR)) + 0.7
				if ret > maxRet {
					maxRet = ret
				}
			}
		}
		return maxRet
	}
	return 1.0
}

func MakeVariablesV2(variablesDir string) (VariablesV2, error) {
	// Loads all variables.
	variables, err := LoadVariablesTranslationsV2(variablesDir)
	if err != nil {
		return nil, err
	}

	years := MakeYearVariablesV2()
	variables[consts.VAR_YEAR] = make(TranslationsV2)
	variables[consts.VAR_TEXT] = make(TranslationsV2)
	for _, lang := range consts.ALL_KNOWN_LANGS {
		// Year
		variables[consts.VAR_YEAR][lang] = years
		// Special free text variable. Proceeded with percolator search.
		variables[consts.VAR_TEXT][lang] = map[string][]string{consts.VAR_TEXT: []string{consts.VAR_TEXT}}
	}

	return variables, nil
}

func LoadVariablesTranslationsV2(variablesDir string) (VariablesV2, error) {
	variables := make(VariablesV2)

	// Load variables from files
	suffix := "variable"
	matches, err := filepath.Glob(filepath.Join(variablesDir, fmt.Sprintf("*.%s", suffix)))
	if err != nil {
		return nil, err
	}
	log.Infof("Globed %d variable translation files.", len(matches))
	for _, variableFile := range matches {
		basename := filepath.Base(variableFile)
		variable := fmt.Sprintf("$%s", snakeCaseToCamelCase(basename[:len(basename)-len(suffix)-1]))
		variableTranslations, err := LoadVariableTranslationsFromFile(variableFile, variable)
		if err != nil {
			return nil, err
		}
		variables[variable] = variableTranslations
	}

	// Load holiday variables from DB
	db, err := sql.Open("postgres", viper.GetString("mdb.url"))
	if err != nil {
		return nil, errors.Wrap(err, "Unable to connect to DB.")
	}
	holidayTranslations, err := LoadHolidayTranslationsFromDB(db)
	if err != nil {
		return nil, err
	}
	variables[consts.VAR_HOLIDAYS] = holidayTranslations
	// Generate source position variables
	positionTranslations, err := GenerateSourcePositionVariables(db)
	if err != nil {
		return nil, err
	}
	variables[consts.VAR_POSITION] = positionTranslations
	// Load source name variables from DB
	sourceNamesTranslationsFromDB, err := LoadSourceNameTranslationsFromDB(db)
	if err != nil {
		return nil, err
	}
	// Combine source name variables from DB with source name variables from file
	if _, ok := variables[consts.VAR_SOURCE]; !ok {
		variables[consts.VAR_SOURCE] = make(TranslationsV2)
	}
	for lang, translations := range sourceNamesTranslationsFromDB {
		if _, ok := variables[consts.VAR_SOURCE][lang]; !ok {
			variables[consts.VAR_SOURCE][lang] = make(map[string][]string)
		}
		for sourceUID, phrasesFromDB := range translations {
			uniquePhrases := map[string]bool{}
			for _, phrase := range phrasesFromDB {
				uniquePhrases[phrase] = true
			}
			if phrasesFromFile, ok := variables[consts.VAR_SOURCE][lang][sourceUID]; ok {
				for _, phrase := range phrasesFromFile {
					uniquePhrases[phrase] = true
				}
			}
			finalPhrases := []string{}
			for uniquePhrase := range uniquePhrases {
				finalPhrases = append(finalPhrases, uniquePhrase)
			}
			variables[consts.VAR_SOURCE][lang][sourceUID] = finalPhrases
		}
	}
	return variables, nil
}

func LoadSourceNameTranslationsFromDB(db *sql.DB) (TranslationsV2, error) {
	translations := make(TranslationsV2)
	//query := `select sn.language, s.uid, sn.name
	//from sources s join source_i18n sn on s.id=sn.source_id`

	query := `select sn.language, s.uid, sn.name
		from sources s join source_i18n sn on s.id=sn.source_id
		left join sources sp on s.parent_id=sp.id
		where sp.uid is null or sp.uid <> '2GAdavz0'` // Filter out Rabash Assorted Notes

	rows, err := queries.Raw(db, query).Query()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve source name translations from DB.")
	}
	defer rows.Close()
	for rows.Next() {
		var lang string
		var uid string
		var phrase string
		err := rows.Scan(&lang, &uid, &phrase)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if _, ok := translations[lang]; !ok {
			translations[lang] = make(map[string][]string)
		}
		translations[lang][uid] = []string{phrase}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}
	return translations, nil
}

func LoadHolidayTranslationsFromDB(db *sql.DB) (TranslationsV2, error) {

	translations := make(TranslationsV2)
	query := `select tn.language, t.uid, tn.label 
	from tags t join tags tp on t.parent_id = tp.id
	join tag_i18n tn on t.id=tn.tag_id
	where tp.uid = '1nyptSIo'`
	//  '1nyptSIo' is a const. uid for 'holidays' parent tag

	rows, err := queries.Raw(db, query).Query()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve from DB the translations for holidays.")
	}
	defer rows.Close()
	for rows.Next() {
		var lang string
		var uid string
		var phrase string
		err := rows.Scan(&lang, &uid, &phrase)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if _, ok := translations[lang]; !ok {
			translations[lang] = make(map[string][]string)
		}
		translations[lang][uid] = []string{phrase}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}
	return translations, nil
}

func LoadVariableTranslationsFromFile(variableFile string, variableName string) (TranslationsV2, error) {
	file, err := os.Open(variableFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading translation file: %s", variableFile)
	}
	defer file.Close()
	log.Infof("Reading %s variable transations file.", variableFile)

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	lineNum := 1
	translations := make(TranslationsV2) // Map from language to value to phrases.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re := regexp.MustCompile(`^(.*),(.*) => (.*)$`)
		matches := re.FindStringSubmatch(line)
		if len(matches) != 4 {
			return nil, errors.New(fmt.Sprintf("[%s:%d] Error reading pattern: [%s]", variableFile, lineNum, line))
		}
		lang := matches[1]
		value := matches[2]
		translation := matches[3]
		if lang == "" || value == "" || translation == "" {
			return nil, errors.New(fmt.Sprintf("[%s:%d] Error reading pattern: [%s]", variableFile, lineNum, line))
		}
		if _, ok := translations[lang]; !ok {
			translations[lang] = make(map[string][]string) // Map from value to phrases.
		}
		translations[lang][value] = append(translations[lang][value], translation)
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "Error reading translation file: %s", variableFile)
	}

	return translations, nil
}

func GenerateSourcePositionVariables(db *sql.DB) (TranslationsV2, error) {
	translations := make(TranslationsV2)
	query := `select max(position) from sources`
	var max int
	err := queries.Raw(db, query).QueryRow().Scan(&max)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve max sources position from DB.")
	}
	for _, lang := range consts.ALL_KNOWN_LANGS {
		values := make(map[string][]string)
		for i := 1; i < max+1; i++ {
			numStr := strconv.Itoa(i)
			values[numStr] = []string{
				numStr,
			}
			if lang == consts.LANG_HEBREW {
				values[numStr] = append(values[numStr], utils.NumberInHebrew(i))
			}
		}
		translations[lang] = values
	}
	return translations, nil
}
