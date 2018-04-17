package utils

import (
	log "github.com/Sirupsen/logrus"
	"github.com/abadojack/whatlanggo"
	"golang.org/x/text/language"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"golang.org/x/text/language/display"
)

// we use only base languages and ignore locales + scripts
var serverLangs = []language.Tag{
	language.English,    // en fallback
	language.Hebrew,     // he
	language.Russian,    // ru
	language.Spanish,    // es
	language.Italian,    // es
	language.German,     // de
	language.Dutch,      // nl
	language.French,     // fr
	language.Portuguese, // pt
	language.Turkish,    // tr
	language.Polish,     // pl
	language.Arabic,     // ar
	language.Hungarian,  // hu
	language.Finnish,    // fi
	language.Lithuanian, // lt
	language.Japanese,   // ja
	language.Bulgarian,  // bg
	language.Georgian,   // ka
	language.Norwegian,  // no
	language.Swedish,    // sv
	language.Croatian,   // hr
	language.Chinese,    // zh
	language.Persian,    // fa
	language.Romanian,   // ro
	language.Hindi,      // hi
	language.Ukrainian,  // uk
	language.Macedonian, // mk
	language.Slovenian,  // sl
	language.Latvian,    // lv
	language.Slovak,     // sk
	language.Czech,      // cs
}

var GO_TO_MDB = map[language.Tag]string{
	language.English:    consts.LANG_ENGLISH,
	language.Hebrew:     consts.LANG_HEBREW,
	language.Russian:    consts.LANG_RUSSIAN,
	language.Spanish:    consts.LANG_SPANISH,
	language.Italian:    consts.LANG_ITALIAN,
	language.German:     consts.LANG_GERMAN,
	language.Dutch:      consts.LANG_DUTCH,
	language.French:     consts.LANG_FRENCH,
	language.Portuguese: consts.LANG_PORTUGUESE,
	language.Turkish:    consts.LANG_TURKISH,
	language.Polish:     consts.LANG_POLISH,
	language.Arabic:     consts.LANG_ARABIC,
	language.Hungarian:  consts.LANG_HUNGARIAN,
	language.Finnish:    consts.LANG_FINNISH,
	language.Lithuanian: consts.LANG_LITHUANIAN,
	language.Japanese:   consts.LANG_JAPANESE,
	language.Bulgarian:  consts.LANG_BULGARIAN,
	language.Georgian:   consts.LANG_GEORGIAN,
	language.Norwegian:  consts.LANG_NORWEGIAN,
	language.Swedish:    consts.LANG_SWEDISH,
	language.Croatian:   consts.LANG_CROATIAN,
	language.Chinese:    consts.LANG_CHINESE,
	language.Persian:    consts.LANG_PERSIAN,
	language.Romanian:   consts.LANG_ROMANIAN,
	language.Hindi:      consts.LANG_HINDI,
	language.Ukrainian:  consts.LANG_UKRAINIAN,
	language.Macedonian: consts.LANG_MACEDONIAN,
	language.Slovenian:  consts.LANG_SLOVENIAN,
	language.Latvian:    consts.LANG_LATVIAN,
	language.Slovak:     consts.LANG_SLOVAK,
	language.Czech:      consts.LANG_CZECH,
}

func reverseLanguages() map[string]language.Tag {
	n := make(map[string]language.Tag)
	for k, v := range GO_TO_MDB {
		n[v] = k
	}
	return n
}

var MDB_TO_GO = reverseLanguages()

var matcher = language.NewMatcher(serverLangs)

var whatlangoWhitelist = map[whatlanggo.Lang]bool{
	whatlanggo.Eng: true,
	whatlanggo.Heb: true,
	whatlanggo.Rus: true,
	whatlanggo.Spa: true,
	whatlanggo.Ita: true,
	whatlanggo.Deu: true,
	whatlanggo.Nld: true, // Dutch
	whatlanggo.Fra: true,
	whatlanggo.Por: true,
	whatlanggo.Tur: true,
	whatlanggo.Pol: true,
	whatlanggo.Arb: true,
	whatlanggo.Hun: true,
	whatlanggo.Fin: true,
	whatlanggo.Lit: true,
	whatlanggo.Jpn: true,
	whatlanggo.Bul: true,
	whatlanggo.Kat: true, // Georgian
	whatlanggo.Nno: true, // Norwegian
	whatlanggo.Swe: true,
	whatlanggo.Hrv: true,
	whatlanggo.Cmn: true, // Mandarin - Chinese ??
	whatlanggo.Pes: true, // Persian
	whatlanggo.Ron: true, // Romanian
	whatlanggo.Hin: true, // Hindi
	whatlanggo.Ukr: true, // Ukrainian
	whatlanggo.Mkd: true, // Macedonian
	whatlanggo.Slv: true, // Slovenian
	whatlanggo.Lav: true, // Latvian
	//whatlanggo.Slovak: true,  // Slovak
	whatlanggo.Ces: true, // Czech
}

func DetectLanguage(text string, interfaceLanguage string, acceptLanguage string, uiOrder []string) []string {
	bestTag := language.Und
	if len(text) == 0 {
		// If text short, use interfaceLanguage
		bestTag = MDB_TO_GO[interfaceLanguage]
	} else {
		info := whatlanggo.DetectWithOptions(text,
			whatlanggo.Options{
				Whitelist: whatlangoWhitelist,
			})
		iso3 := whatlanggo.LangToString(info.Lang)
		log.Debugf("DetectLanguage: whatlanggo info: %s", whatlanggo.LangToString(info.Lang))

		if iso3 != "" {
			base, err := language.ParseBase(iso3)
			if err == nil {
				bestTag, err = language.Compose(base)
				if err != nil {
					log.Warnf("DetectLanguage: error compose language.Tag for %s: %s", iso3, err.Error())
					bestTag = language.Und
				}
			} else {
				log.Warnf("DetectLanguage: error parsing base for %s: %s", iso3, err.Error())
				bestTag = language.Und
			}
		}
	}
	log.Debugf("DetectLanguage: bestTag 1: %s", display.English.Tags().Name(bestTag))

	if bestTag.IsRoot() {
		log.Debug("DetectLanguage: bestTag is Root, falling back to Accept-Language")
		tags, _, _ := language.ParseAcceptLanguage(acceptLanguage)
		log.Debugf("DetectLanguage: parsing Accept-Language got us %v", tags)
		tag, _, confidence := matcher.Match(tags...)
		log.Debugf("DetectLanguage: matcher found %s with confidence %v",
			display.English.Tags().Name(tag), confidence)
		if confidence != language.No {
			bestTag = tag
		}
	}

	if !bestTag.IsRoot() {
		if l, ok := GO_TO_MDB[bestTag]; ok {
			if order, ok := consts.SEARCH_LANG_ORDER[l]; ok {
				log.Debugf("DetectLanguage: best language is %s", l)
				return order
			}
		}
	}

	log.Debug("DetectLanguage: using default language")
	return consts.SEARCH_LANG_ORDER[interfaceLanguage]
}
