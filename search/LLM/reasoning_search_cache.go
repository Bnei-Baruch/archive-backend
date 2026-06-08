package llm

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	defaultReasoningSearchCacheTTL          = 5 * 24 * time.Hour
	maxReasoningSearchCacheQueryWords       = 8
	maxReasoningSearchCacheWordLengthRunes  = 16
	maxReasoningSearchCacheQueryLengthRunes = 48
)

type ReasoningSearchCacheEntry struct {
	Query   string
	Summary string
	Results []ReasoningSearchResult
}

type reasoningSearchCacheStoreEntry struct {
	value     *ReasoningSearchCacheEntry
	expiresAt time.Time
}

type ReasoningSearchCacheStore struct {
	mu      sync.RWMutex
	entries map[string]*reasoningSearchCacheStoreEntry
	ttl     time.Duration
	stop    chan struct{}
	done    chan struct{}
}

var reasoningSearchCacheWhitespaceRe = regexp.MustCompile(`\s+`)

func reasoningSearchCacheHebrewWeekdayPhrases() []string {
	phrases := []string{}

	withDayWord := []string{
		"יום ראשון",
		"יום שני",
		"יום שלישי",
		"יום רביעי",
		"יום חמישי",
		"יום שישי",
		"יום שבת",
	}
	shortDays := []string{
		"ראשון",
		"שני",
		"שלישי",
		"רביעי",
		"חמישי",
		"שישי",
		"שבת",
	}

	for _, day := range withDayWord {
		phrases = append(phrases, "מ"+day, "ב"+day)
	}
	for _, day := range shortDays {
		phrases = append(phrases, "מ"+day, "ב"+day)
	}

	return phrases
}

func reasoningSearchCacheWeekdayPhrases() []string {
	return append(reasoningSearchCacheHebrewWeekdayPhrases(),
		// English
		"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday",
		// Russian
		"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота",
		// Spanish
		"domingo", "lunes", "martes", "miércoles", "miercoles", "jueves", "viernes", "sábado", "sabado",
		// Italian
		"domenica", "lunedì", "lunedi", "martedì", "martedi", "mercoledì", "mercoledi", "giovedì", "giovedi", "venerdì", "venerdi", "sabato",
		// German
		"sonntag", "montag", "dienstag", "mittwoch", "donnerstag", "freitag", "samstag",
		// French
		"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi",
		// Portuguese
		"domingo", "segunda-feira", "segunda feira", "terça-feira", "terca-feira", "terça feira", "terca feira", "quarta-feira", "quarta feira", "quinta-feira", "quinta feira", "sexta-feira", "sexta feira", "sábado", "sabado",
		// Turkish
		"pazar", "pazartesi", "salı", "sali", "çarşamba", "carsamba", "perşembe", "persembe", "cuma", "cumartesi",
		// Dutch
		"zondag", "maandag", "dinsdag", "woensdag", "donderdag", "vrijdag", "zaterdag",
		// Polish
		"niedziela", "niedzielę", "poniedziałek", "wtorek", "środa", "środę", "czwartek", "piątek", "sobota", "sobotę",
		// Arabic
		"الأحد", "الاحد", "الاثنين", "الثلاثاء", "الأربعاء", "الاربعاء", "الخميس", "الجمعة", "السبت",
		// Hungarian
		"vasárnap", "vasarnap", "hétfő", "hetfo", "kedd", "szerda", "csütörtök", "csutortok", "péntek", "pentek", "szombat",
		// Finnish
		"sunnuntai", "maanantai", "tiistai", "keskiviikko", "torstai", "perjantai", "lauantai",
		// Lithuanian
		"sekmadienis", "pirmadienis", "antradienis", "trečiadienis", "treciadienis", "ketvirtadienis", "penktadienis", "šeštadienis", "sestadienis",
		// Japanese
		"日曜日", "月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日",
		// Bulgarian
		"неделя", "понеделник", "вторник", "сряда", "четвъртък", "петък", "събота",
		// Georgian
		"კვირა", "ორშაბათი", "სამშაბათი", "ოთხშაბათი", "ხუთშაბათი", "პარასკევი", "შაბათი",
		// Norwegian
		"søndag", "sondag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag", "lordag",
		// Swedish
		"söndag", "sondag", "måndag", "mandag", "tisdag", "onsdag", "torsdag", "fredag", "lördag", "lordag",
		// Croatian
		"nedjelja", "ponedjeljak", "utorak", "srijeda", "četvrtak", "cetvrtak", "petak", "subota",
		// Chinese
		"星期日", "星期天", "周日", "周天", "星期一", "周一", "星期二", "周二", "星期三", "周三", "星期四", "周四", "星期五", "周五", "星期六", "周六",
		// Persian
		"یکشنبه", "دوشنبه", "سه‌شنبه", "سه شنبه", "چهارشنبه", "پنجشنبه", "جمعه", "شنبه",
		// Romanian
		"duminică", "duminica", "luni", "marți", "marti", "miercuri", "joi", "vineri", "sâmbătă", "sambata",
		// Hindi
		"रविवार", "सोमवार", "मंगलवार", "बुधवार", "गुरुवार", "शुक्रवार", "शनिवार",
		// Ukrainian
		"неділя", "неділю", "понеділок", "вівторок", "середа", "середу", "четвер", "пʼятниця", "п'ятниця", "субота", "суботу",
		// Macedonian
		"недела", "понеделник", "вторник", "среда", "четврток", "петок", "сабота",
		// Slovenian
		"nedelja", "ponedeljek", "torek", "sreda", "četrtek", "cetrtek", "petek", "sobota",
		// Latvian
		"svētdiena", "svetdiena", "pirmdiena", "otrdiena", "trešdiena", "tresdiena", "ceturtdiena", "piektdiena", "sestdiena",
		// Slovak
		"nedeľa", "nedela", "pondelok", "utorok", "streda", "štvrtok", "stvrtok", "piatok", "sobota",
		// Czech
		"neděle", "nedele", "pondělí", "pondeli", "úterý", "utery", "středa", "streda", "čtvrtek", "ctvrtek", "pátek", "patek", "sobota",
		// Amharic
		"እሑድ", "ሰኞ", "ማክሰኞ", "ረቡዕ", "ሐሙስ", "ዓርብ", "ቅዳሜ",
		// Indonesian
		"minggu", "senin", "selasa", "rabu", "kamis", "jumat", "sabtu",
		// Armenian
		"կիրակի", "երկուշաբթի", "երեքշաբթի", "չորեքշաբթի", "հինգշաբթի", "ուրբաթ", "շաբաթ",
		// Danish
		"søndag", "sondag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag", "lordag",
		// Estonian
		"pühapäev", "pyhapaev", "esmaspäev", "esmaspaev", "teisipäev", "teisipaev", "kolmapäev", "kolmapaev", "neljapäev", "neljapaev", "reede", "laupäev", "laupaev",
		// Greek
		"κυριακή", "δευτέρα", "τρίτη", "τετάρτη", "πέμπτη", "παρασκευή", "σάββατο",
		// Tagalog
		"linggo", "lunes", "martes", "miyerkules", "huwebes", "biyernes", "sabado",
		// Azerbaijani
		"bazar", "bazar ertəsi", "bazar ertesi", "çərşənbə axşamı", "cersenbe axsami", "çərşənbə", "cersenbe", "cümə axşamı", "cume axsami", "cümə", "cume", "şənbə", "senbe",
	)
}

var reasoningSearchCacheVolatilePhrases = []string{
	// English
	"today", "yesterday", "tomorrow", "this week", "last week", "last weekend", "this month", "last month", "this year", "last year", "daily lesson", "morning lesson", "latest lesson", "recent lesson", "today lesson",
	// Hebrew
	"היום", "אתמול", "מחר", "השבוע", "שבוע שעבר", "סוף השבוע", "סוף שבוע", "הסופש", "סופש", "החודש", "חודש שעבר", "השנה", "שנה שעברה", "שיעור יומי", "שיעור בוקר", "השיעור היומי", "השיעור של היום", "היום בשיעור", "בשיעור היום", "בשיעור של היום", "מאתמול", "משבוע", "מספופש", "מסופש", "מסוף השבוע", "מסוף שבוע", "בסופש", "בסוף שבוע", "בסוף השבוע",
	// Russian
	"сегодня", "вчера", "завтра", "эта неделя", "на этой неделе", "прошлая неделя", "прошлые выходные", "этот месяц", "прошлый месяц", "этот год", "прошлый год", "ежедневный урок", "утренний урок", "сегодняшний урок", "субботний урок",
	// Spanish
	"hoy", "ayer", "mañana", "esta semana", "la semana pasada", "el fin de semana pasado", "este mes", "el mes pasado", "este año", "el año pasado", "lección diaria", "lección de la mañana", "lección de hoy",
	// Italian
	"oggi", "ieri", "domani", "questa settimana", "la settimana scorsa", "lo scorso fine settimana", "questo mese", "il mese scorso", "questo anno", "l'anno scorso", "lezione quotidiana", "lezione del mattino", "lezione di oggi",
	// German
	"heute", "gestern", "morgen", "diese woche", "letzte woche", "letztes wochenende", "dieser monat", "letzter monat", "dieses jahr", "letztes jahr", "tägliche lektion", "morgenunterricht", "heutige lektion",
	// French
	"aujourd'hui", "aujourd hui", "hier", "demain", "cette semaine", "la semaine dernière", "le week end dernier", "ce mois ci", "ce mois-ci", "le mois dernier", "cette année", "l'année dernière", "leçon quotidienne", "cours du matin", "leçon d'aujourd'hui", "leçon d aujourd hui",
	// Portuguese
	"hoje", "ontem", "amanhã", "esta semana", "semana passada", "fim de semana passado", "este mês", "mês passado", "este ano", "ano passado", "lição diária", "aula da manhã", "lição de hoje",
	// Turkish
	"bugün", "dün", "yarın", "bu hafta", "geçen hafta", "geçen hafta sonu", "bu ay", "geçen ay", "bu yıl", "geçen yıl", "günlük ders", "sabah dersi", "bugünün dersi",
	// Dutch
	"vandaag", "gisteren", "morgen", "deze week", "vorige week", "afgelopen weekend", "deze maand", "vorige maand", "dit jaar", "vorig jaar", "dagelijkse les", "ochtendles", "les van vandaag",
	// Polish
	"dzisiaj", "dziś", "wczoraj", "jutro", "w tym tygodniu", "w zeszłym tygodniu", "w zeszły weekend", "w tym miesiącu", "w zeszłym miesiącu", "w tym roku", "w zeszłym roku", "codzienna lekcja", "poranna lekcja", "dzisiejsza lekcja",
	// Arabic
	"اليوم", "أمس", "امس", "غدا", "غدًا", "هذا الأسبوع", "الأسبوع الماضي", "عطلة نهاية الأسبوع الماضية", "هذا الشهر", "الشهر الماضي", "هذا العام", "العام الماضي", "الدرس اليومي", "درس الصباح", "درس اليوم",
	// Hungarian
	"ma", "tegnap", "holnap", "ezen a héten", "múlt héten", "múlt hétvégén", "ebben a hónapban", "múlt hónapban", "ebben az évben", "tavaly", "napi lecke", "reggeli lecke", "mai lecke",
	// Finnish
	"tänään", "eilen", "huomenna", "tällä viikolla", "viime viikolla", "viime viikonloppuna", "tässä kuussa", "viime kuussa", "tänä vuonna", "viime vuonna", "päivittäinen oppitunti", "aamuopetus", "tämän päivän oppitunti",
	// Lithuanian
	"šiandien", "vakar", "rytoj", "šią savaitę", "praeitą savaitę", "praėjusį savaitgalį", "šį mėnesį", "praeitą mėnesį", "šiais metais", "praėjusiais metais", "dienos pamoka", "rytinė pamoka", "šiandienos pamoka",
	// Japanese
	"今日", "昨日", "明日", "今週", "先週", "先週末", "今月", "先月", "今年", "去年", "毎日のレッスン", "朝のレッスン", "今日のレッスン",
	// Bulgarian
	"днес", "вчера", "утре", "тази седмица", "миналата седмица", "миналия уикенд", "този месец", "миналия месец", "тази година", "миналата година", "ежедневен урок", "сутрешен урок", "днешният урок",
	// Georgian
	"დღეს", "გუშინ", "ხვალ", "ამ კვირაში", "გასულ კვირას", "გასულ შაბათ კვირას", "ამ თვეში", "გასულ თვეში", "ამ წელს", "გასულ წელს", "ყოველდღიური გაკვეთილი", "დილის გაკვეთილი", "დღევანდელი გაკვეთილი",
	// Norwegian
	"i dag", "idag", "i går", "igår", "i morgen", "denne uken", "forrige uke", "forrige helg", "denne måneden", "forrige måned", "i år", "i fjor", "daglig leksjon", "morgenundervisning", "dagens leksjon",
	// Swedish
	"idag", "i dag", "igår", "i går", "imorgon", "denna vecka", "förra veckan", "förra helgen", "denna månad", "förra månaden", "i år", "förra året", "daglig lektion", "morgonlektion", "dagens lektion",
	// Croatian
	"danas", "jučer", "sutra", "ovaj tjedan", "prošli tjedan", "prošli vikend", "ovaj mjesec", "prošli mjesec", "ove godine", "prošle godine", "dnevna lekcija", "jutarnja lekcija", "današnja lekcija",
	// Chinese
	"今天", "昨天", "明天", "本周", "上周", "上个周末", "本月", "上个月", "今年", "去年", "每日课程", "晨课", "今天的课程",
	// Persian
	"امروز", "دیروز", "فردا", "این هفته", "هفته گذشته", "آخر هفته گذشته", "این ماه", "ماه گذشته", "امسال", "سال گذشته", "درس روزانه", "درس صبح", "درس امروز",
	// Romanian
	"astăzi", "ieri", "mâine", "săptămâna aceasta", "săptămâna trecută", "weekendul trecut", "luna aceasta", "luna trecută", "anul acesta", "anul trecut", "lecția zilnică", "lecția de dimineață", "lecția de azi",
	// Hindi
	"आज", "कल", "इस सप्ताह", "पिछले सप्ताह", "पिछले सप्ताहांत", "इस महीने", "पिछले महीने", "इस साल", "पिछले साल", "दैनिक पाठ", "सुबह का पाठ", "आज का पाठ",
	// Ukrainian
	"сьогодні", "вчора", "завтра", "цього тижня", "минулого тижня", "минулих вихідних", "цього місяця", "минулого місяця", "цього року", "минулого року", "щоденний урок", "ранковий урок", "сьогоднішній урок",
	// Macedonian
	"денес", "вчера", "утре", "оваа недела", "минатата недела", "минатиот викенд", "овој месец", "минатиот месец", "оваа година", "минатата година", "дневна лекција", "утринска лекција", "денешната лекција",
	// Slovenian
	"danes", "včeraj", "jutri", "ta teden", "prejšnji teden", "prejšnji vikend", "ta mesec", "prejšnji mesec", "letos", "lani", "dnevna lekcija", "jutranja lekcija", "današnja lekcija",
	// Latvian
	"šodien", "vakar", "rīt", "šonedēļ", "pagājušajā nedēļā", "pagājušajā nedēļas nogalē", "šomēnes", "pagājušajā mēnesī", "šogad", "pagājušajā gadā", "ikdienas nodarbība", "rīta nodarbība", "šodienas nodarbība",
	// Slovak
	"dnes", "včera", "zajtra", "tento týždeň", "minulý týždeň", "minulý víkend", "tento mesiac", "minulý mesiac", "tento rok", "minulý rok", "denná lekcia", "ranná lekcia", "dnešná lekcia",
	// Czech
	"dnes", "včera", "zítra", "tento týden", "minulý týden", "minulý víkend", "tento měsíc", "minulý měsíc", "letos", "loni", "denní lekce", "ranní lekce", "dnešní lekce",
	// Amharic
	"ዛሬ", "ትናንት", "ነገ", "ይህ ሳምንት", "ያለፈው ሳምንት", "ያለፈው የሳምንት መጨረሻ", "ይህ ወር", "ያለፈው ወር", "ዘንድሮ", "ባለፈው ዓመት", "የዕለቱ ትምህርት", "የጠዋት ትምህርት",
	// Indonesian
	"hari ini", "kemarin", "besok", "minggu ini", "minggu lalu", "akhir pekan lalu", "bulan ini", "bulan lalu", "tahun ini", "tahun lalu", "pelajaran harian", "pelajaran pagi",
	// Armenian
	"այսօր", "երեկ", "վաղը", "այս շաբաթ", "անցած շաբաթ", "անցած հանգստյան օրերին", "այս ամիս", "անցած ամիս", "այս տարի", "անցած տարի", "ամենօրյա դաս", "առավոտյան դաս",
	// Danish
	"i dag", "idag", "i går", "igår", "i morgen", "denne uge", "sidste uge", "sidste weekend", "denne måned", "sidste måned", "i år", "sidste år", "daglig lektion", "morgenlektion", "dagens lektion",
	// Estonian
	"täna", "eile", "homme", "sel nädalal", "eelmisel nädalal", "eelmisel nädalavahetusel", "sel kuul", "eelmisel kuul", "sel aastal", "eelmisel aastal", "igapäevane õppetund", "hommikutund", "tänane õppetund",
	// Greek
	"σήμερα", "χθες", "αύριο", "αυτή την εβδομάδα", "την περασμένη εβδομάδα", "το περασμένο σαββατοκύριακο", "αυτόν τον μήνα", "τον περασμένο μήνα", "φέτος", "πέρυσι", "καθημερινό μάθημα", "πρωινό μάθημα", "σημερινό μάθημα",
	// Tagalog
	"ngayon", "kahapon", "bukas", "ngayong linggo", "nakaraang linggo", "nakaraang weekend", "ngayong buwan", "nakaraang buwan", "ngayong taon", "nakaraang taon", "araw araw na aralin", "umagang aralin", "aralin ngayon",
	// Azerbaijani
	"bu gün", "bugün", "dünən", "sabah", "bu həftə", "keçən həftə", "keçən həftə sonu", "bu ay", "keçən ay", "bu il", "keçən il", "gündəlik dərs", "səhər dərsi", "bugünkü dərs",
}

func init() {
	reasoningSearchCacheVolatilePhrases = append(reasoningSearchCacheVolatilePhrases, reasoningSearchCacheWeekdayPhrases()...)
}

func NewReasoningSearchCacheStore(ttl time.Duration) *ReasoningSearchCacheStore {
	if ttl <= 0 {
		ttl = defaultReasoningSearchCacheTTL
	}

	store := &ReasoningSearchCacheStore{
		entries: map[string]*reasoningSearchCacheStoreEntry{},
		ttl:     ttl,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go store.cleanupLoop()
	return store
}

func (s *ReasoningSearchCacheStore) Get(cacheKey string) (*ReasoningSearchCacheEntry, bool) {
	now := time.Now()

	s.mu.RLock()
	entry, ok := s.entries[cacheKey]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.entries, cacheKey)
		s.mu.Unlock()
		return nil, false
	}
	return cloneReasoningSearchCacheEntry(entry.value), true
}

func (s *ReasoningSearchCacheStore) Set(cacheKey string, value *ReasoningSearchCacheEntry) {
	if strings.TrimSpace(cacheKey) == "" || value == nil {
		return
	}
	now := time.Now()

	s.mu.Lock()
	s.entries[cacheKey] = &reasoningSearchCacheStoreEntry{
		value:     cloneReasoningSearchCacheEntry(value),
		expiresAt: now.Add(s.ttl),
	}
	s.mu.Unlock()
}

func (s *ReasoningSearchCacheStore) Close() error {
	close(s.stop)
	<-s.done
	return nil
}

func (s *ReasoningSearchCacheStore) cleanupLoop() {
	ticker := time.NewTicker(openAIReasoningSessionCleanupInterval(s.ttl))
	defer func() {
		ticker.Stop()
		close(s.done)
	}()

	for {
		select {
		case <-ticker.C:
			s.deleteExpired()
		case <-s.stop:
			return
		}
	}
}

func (s *ReasoningSearchCacheStore) deleteExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}

func ReasoningSearchCacheKeyForQuery(query string) (string, bool) {
	key := normalizeReasoningSearchCacheKeyQuery(query)
	if key == "" {
		return "", false
	}

	if utf8.RuneCountInString(key) > maxReasoningSearchCacheQueryLengthRunes {
		return "", false
	}

	words := strings.Fields(key)
	if len(words) == 0 || len(words) > maxReasoningSearchCacheQueryWords {
		return "", false
	}
	for _, word := range words {
		if utf8.RuneCountInString(word) > maxReasoningSearchCacheWordLengthRunes {
			return "", false
		}
	}

	matchText := normalizeReasoningSearchCacheMatchText(query)
	if matchText == "" || reasoningSearchCacheContainsVolatilePhrase(matchText) {
		return "", false
	}

	return key, true
}

func BuildReasoningSearchCacheEntryFromResponse(response *ReasoningSearchResponse) *ReasoningSearchCacheEntry {
	if response == nil || len(response.Results) == 0 {
		return nil
	}

	results := make([]ReasoningSearchResult, 0, len(response.Results))
	for _, result := range response.Results {
		results = append(results, ReasoningSearchResult{
			MDBUID:           result.MDBUID,
			ResultType:       result.ResultType,
			Reason:           result.Reason,
			Highlights:       append([]ReasoningSearchHighlight(nil), result.Highlights...),
			IsGroupingResult: result.IsGroupingResult,
		})
	}

	summary := ""
	if response.Summary != nil {
		summary = *response.Summary
	}
	return &ReasoningSearchCacheEntry{
		Query:   response.Query,
		Summary: summary,
		Results: results,
	}
}

func cloneReasoningSearchCacheEntry(entry *ReasoningSearchCacheEntry) *ReasoningSearchCacheEntry {
	if entry == nil {
		return nil
	}

	cloned := &ReasoningSearchCacheEntry{
		Query:   entry.Query,
		Summary: entry.Summary,
		Results: make([]ReasoningSearchResult, 0, len(entry.Results)),
	}
	for _, result := range entry.Results {
		cloned.Results = append(cloned.Results, ReasoningSearchResult{
			MDBUID:           result.MDBUID,
			ResultType:       result.ResultType,
			Title:            result.Title,
			Description:      result.Description,
			ContentType:      result.ContentType,
			ProgramName:      result.ProgramName,
			Date:             result.Date,
			Reason:           result.Reason,
			Highlights:       append([]ReasoningSearchHighlight(nil), result.Highlights...),
			IsGroupingResult: result.IsGroupingResult,
		})
	}
	return cloned
}

func normalizeReasoningSearchCacheKeyQuery(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	normalized = strings.ReplaceAll(normalized, "+", " ")
	normalized = reasoningSearchCacheWhitespaceRe.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func normalizeReasoningSearchCacheMatchText(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	normalized = strings.ReplaceAll(normalized, "+", " ")
	normalized = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsSpace(r):
			return r
		default:
			return ' '
		}
	}, normalized)
	normalized = reasoningSearchCacheWhitespaceRe.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func reasoningSearchCacheContainsVolatilePhrase(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	paddedQuery := " " + normalizedQuery + " "
	for _, phrase := range reasoningSearchCacheVolatilePhrases {
		normalizedPhrase := normalizeReasoningSearchCacheMatchText(phrase)
		if normalizedPhrase == "" {
			continue
		}
		if strings.Contains(paddedQuery, " "+normalizedPhrase+" ") || strings.Contains(normalizedQuery, normalizedPhrase) {
			return true
		}
	}
	return false
}

func BuildReasoningSearchCacheSeedAssistantContent(entry *ReasoningSearchCacheEntry) string {
	if entry == nil {
		return ""
	}

	type seedResult struct {
		MDBUID           string   `json:"mdb_uid"`
		ResultType       string   `json:"result_type"`
		Reason           string   `json:"reason"`
		Highlights       []string `json:"highlights"`
		IsGroupingResult bool     `json:"is_grouping_result"`
	}
	type seedPayload struct {
		Query   string       `json:"query"`
		Summary string       `json:"summary"`
		Results []seedResult `json:"results"`
	}

	payload := seedPayload{
		Query:   entry.Query,
		Summary: entry.Summary,
		Results: make([]seedResult, 0, len(entry.Results)),
	}
	for _, result := range entry.Results {
		payload.Results = append(payload.Results, seedResult{
			MDBUID:           result.MDBUID,
			ResultType:       result.ResultType,
			Reason:           result.Reason,
			Highlights:       GetReasoningSearchHighlightSnippetTexts(result.Highlights),
			IsGroupingResult: result.IsGroupingResult,
		})
	}
	raw, err := marshalJSON(payload)
	if err != nil {
		return entry.Summary
	}
	return "Cached initial reasoning search response:\n" + raw
}

func marshalJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
