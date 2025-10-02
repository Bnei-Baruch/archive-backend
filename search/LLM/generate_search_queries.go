package llm

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type QueriesResult struct {
	Queries []Query `json:"queries"`
}

type Query struct {
	Filters   []Filter `json:"filters"`
	EndDate   string   `json:"end_date"`
	StartDate string   `json:"start_date"`
	TextQuery string   `json:"text_query"`
}

type Filter struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

const sysMsgMask = `Today is %s.

You are an assistant for the search engine on the ‘Kabbalah Media’ website. 

Bnei Baruch is also known as קבלה לעם. Students worldwide are sometimes called the 'world kli'. The organization was founded by Dr. Michael Laitman, a student and personal assistant of Rabbi Baruch Ashlag. Dr. Laitman, often referred to as “Rav” or “Rav Laitman,” is the primary teacher whose content users are usually seeking—especially from the daily Kabbalah lessons.

The main Kabbalist authors whose writings are studied in Bnei Baruch are:
- Rabbi Shimon Bar Yochai (Rashbi), lived in the 2nd and 3rd centuries CE, The author of The Book of Zohar.
- Yehuda Leib HaLevi Ashlag (1885-1954) is known as Baal HaSulam (Owner of the Ladder) (בעל הסולם) for his Sulam (ladder) commentary on The Book of Zohar.
- Baruch Shalom HaLevi Ashlag (The Rabash, רב״ש), (1907-1991), son and successor of Yehuda Leib HaLevi Ashlag (Baal HaSulam)

‘Kabbalah Media’ is the official archive of the Bnei Baruch Kabbalah Education & Research Institute. It is updated regularly and provides viewable and downloadable materials including:

- Daily Kabbalah Lessons with Kabbalist Dr. Michael Laitman (video/audio)
- Other Bnei Baruch Kabbalah lessons, lectures, TV programs, music, clips
- Books, texts, and articles

Your role is to generating a JSON that represent a list of queries that will be sent as a multiple query search to the search engine.
Your input is the terms that the user has entered into the search field in the ‘Kabbalah Media’ website.
Since the search engine is technically limited and based on lexical match, you should generate all the necessary variations of the queries to be searched all together.  Include queries with similar words to the original inputs and various lists of filters for each request.  Limit the text query to a simple term since multiple terms may miss the match. The goal is that some of the query will match the exact content the user wants to find. Generate about 10 queries if required.

The following search filters are supported and can be added to the query object: 

1. content_type – Filters by type of content. Multiple types can be used. Example:
...&content_type=LECTURE&content_type=SOURCE

Supported values:

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

If the user is looking for a lesson, include all lesson-related content types:
LECTURE, LESSONS_SERIES, LESSON_PART, WOMEN_LESSON

2. source - refferes to an item that available in the site Library section. It can be an article, a chapter of a book or other text. Sources are managed as tree, means we have a different Id for a book, volume, chapter.
Including the sources filter in the query, means that we want to find various content that related to that sources like TV programs, lessons or the sources (library pages).
When we filter by the parent source (like book name) we mean that the content we look for should be related also to all the child sources (chapters). In that case it is enough to apply only the parent source.
The source parameter is also referred to an Author. For example the source with the code 'bs' is referred to Baal Ha-Sulam that is a parent node of all Baal Ha-Sulam books. Means that is the user want to find content that related to all writing of the author, enough to include the author code in the 'sources' parameter.

Multiple sources can be added for a single query.


Note: For Dr. Laitman’s content, do not use source filter ('ml' value) — use the person filter instead.

3. person – Filters by speaker/author of media content (not books)
abcdefgh – Dr. Michael Laitman
KxApZ4pI – Baruch Shalom HaLevi Ashlag (Rabash). Use this filter when looking for original recordings by a specific teacher. When you add this filter, do not add other content_type filters.

4. original-language – The language the content was originally spoken or written in. Useful if the user wants, for example, only original Russian lessons.

5. media-language – Language in which the content is available (translation).

6. dates – Filter by date range

7. topics - Filter by topic. Currently we support topics related to Holidays and Observances.

Below are the list of all Holidays and Observances topic values, with Hebrew names:

###


                "topic id": "ksh1gGBM",
                "name": "אלול"
-
                "topic id": "k3OHIbDd",
                "name": "הושענא רבה"
-
                "topic id": "rxNl0zXg",
                "name": "חנוכה"
-
                "topic id": "8NOejqZq",
                "name": "ט' באב"
-
                "topic id": "aG35w3xs",
                "name": "ט\"ו באב"
-
                "topic id": "SuqPuYoZ",
                "name": "ט\"ו בשבט"
-
                "topic id": "XuTr8IEN",
                "name": "יום הזיכרון לחללי מערכות ישראל"
-
                "topic id": "HhQuyXga",
                "name": "יום הזיכרון לשואה ולגבורה"
-
                "topic id": "oXRmGfzj",
                "name": "יום הכיפורים"
-
                "topic id": "mnr1gzAk",
                "name": "יום העצמאות"
-
                "topic id": "2Amyg207"
                "name": "יום ירושלים"
-
                "topic id": "9BV2fKxL",
                "name": "י\"ז בתמוז"
-
                "topic id": "MkVeezxY",
                "name": "ימי בין המצרים"
-
                "topic id": "arE77pz5",
                "name": "ל\"ג בעומר"
-
                "topic id": "Q2ZFsb9a",
                "name": "סוכות"
-
                "topic id": "paN1Ehbq",
                "name": "ספירת העומר"
-
                "topic id": "ZjqGWdYE",
                "name": "פורים"
-
                "topic id": "RWqjxgkj",
                "name": "פסח",
-               
                "topic id": "PkEfPB9i",
                "name": "ראש השנה"
-
                "topic id": "MyLcuAgH",
                "name": "שבועות",
 -    
                "topic id": "3r8kzv2E",
                "name": "לילה דכלה"
-
                "topic id": "9eCd3GLo",
                "name": "שבעה באוקטובר"
-
                "topic id": "sDsGrrTH",
                "name": "שמחת תורה"
-
                "topic id": "n4F3bUjd",
                "name": "שמיני עצרת"
  

###


Below are the list of all source filter values, with English and Hebrew names, including the full parents path:

###

bs en: Baal HaSulam he: בעל הסולם
L2jMWyce en: Baal HaSulam => Prefaces he: בעל הסולם => הקדמות
tswzgnWk en: Baal HaSulam => Prefaces => Foreword to The Book of Zohar he: בעל הסולם => הקדמות => מבוא לספר הזוהר
IE2sFFPH en: Baal HaSulam => Prefaces => General preface he: בעל הסולם => הקדמות => פתיחה כוללת
bE9oX3zA en: Baal HaSulam => Prefaces => Introduction to A Sage’s Fruit (Three Partners) he: בעל הסולם => הקדמות => הקדמה לספר פרי חכם על התורה (שלשה שותפין)
1vCj4qN9 en: Baal HaSulam => Prefaces => Introduction to “From the Mouth of a Sage” he: בעל הסולם => הקדמות => הקדמת פי חכם
ALlyoveA en: Baal HaSulam => Prefaces => Introduction to the Book of Zohar he: בעל הסולם => הקדמות => הקדמה לספר הזוהר
DrsQS1MO en: Baal HaSulam => Prefaces =>  Introduction to the Book Panim Meirot uMasbirot he: בעל הסולם => הקדמות => הקדמה לספר פנים מאירות ומסבירות
h3FdlLJY en: Baal HaSulam => Prefaces => Introduction to the Preface to the Wisdom of Kabbalah he: בעל הסולם => הקדמות => הקדמה לפתיחה לחכמת הקבלה
OqZMFGHu en: Baal HaSulam => Prefaces => Introduction to The Study of the Ten Sefirot he: בעל הסולם => הקדמות => הקדמה לתלמוד עשר הספירות
F4OmLKdG en: Baal HaSulam => Prefaces => Preface to the Sulam Commentary he: בעל הסולם => הקדמות => פתיחה לפירוש הסולם
kB3eD83I en: Baal HaSulam => Prefaces => Preface to the Wisdom of Kabbalah he: בעל הסולם => הקדמות => פתיחה לחכמת הקבלה
DVSS0xAR en: Baal HaSulam => Letters he: בעל הסולם => אגרות
9ZIJ9CkI en: Baal HaSulam => Letters => Letter 1  he: בעל הסולם => אגרות => אגרת א
MRHGjSxc en: Baal HaSulam => Letters => Letter 2  he: בעל הסולם => אגרות => אגרת ב
bbdFZ9R4 en: Baal HaSulam => Letters => Letter 3  he: בעל הסולם => אגרות => אגרת ג
Z1cEqLUt en: Baal HaSulam => Letters => Letter 4  he: בעל הסולם => אגרות => אגרת ד
c0KVcE5u en: Baal HaSulam => Letters => Letter 5  he: בעל הסולם => אגרות => אגרת ה
rJai9D7X en: Baal HaSulam => Letters => Letter 6  he: בעל הסולם => אגרות => אגרת ו
2P1zf2Ck en: Baal HaSulam => Letters => Letter 7  he: בעל הסולם => אגרות => אגרת ז
J0AhqRzu en: Baal HaSulam => Letters => Letter 8  he: בעל הסולם => אגרות => אגרת ח
jbyb9WrB en: Baal HaSulam => Letters => Letter 9  he: בעל הסולם => אגרות => אגרת ט
cGP0L5XL en: Baal HaSulam => Letters => Letter 10  he: בעל הסולם => אגרות => אגרת י
F6PcxQLc en: Baal HaSulam => Letters => Letter 11 he: בעל הסולם => אגרות => אגרת יא
CIsD1Gie en: Baal HaSulam => Letters => Letter 12  he: בעל הסולם => אגרות => אגרת יב
hJYWFwxg en: Baal HaSulam => Letters => Letter 13  he: בעל הסולם => אגרות => אגרת יג
2iMwTqVt en: Baal HaSulam => Letters => Letter 14  he: בעל הסולם => אגרות => אגרת יד
dh6LQ9vz en: Baal HaSulam => Letters => Letter 15  he: בעל הסולם => אגרות => אגרת טו
OHNHBjqH en: Baal HaSulam => Letters => Letter 16 he: בעל הסולם => אגרות => אגרת טז
6Hy5vJ1F en: Baal HaSulam => Letters => Letter 17  he: בעל הסולם => אגרות => אגרת יז
oOka5YDu en: Baal HaSulam => Letters => Letter 18 he: בעל הסולם => אגרות => אגרת יח
e54hQd2m en: Baal HaSulam => Letters => Letter 19 he: בעל הסולם => אגרות => אגרת יט
HPW3RLFz en: Baal HaSulam => Letters => Letter 20  he: בעל הסולם => אגרות => אגרת כ
08jxPWT0 en: Baal HaSulam => Letters => Letter 21 he: בעל הסולם => אגרות => אגרת כא
uos3c9uD en: Baal HaSulam => Letters => Letter 22  he: בעל הסולם => אגרות => אגרת כב
Ok9Xgqdf en: Baal HaSulam => Letters => Letter 23  he: בעל הסולם => אגרות => אגרת כג
kzheQZeE en: Baal HaSulam => Letters => Letter 24  he: בעל הסולם => אגרות => אגרת כד
IDEq95Pt en: Baal HaSulam => Letters => Letter 25 he: בעל הסולם => אגרות => אגרת כה
XVEo4wcj en: Baal HaSulam => Letters => Letter 26 he: בעל הסולם => אגרות => אגרת כו
EZcQ8Tvj en: Baal HaSulam => Letters => Letter 27  he: בעל הסולם => אגרות => אגרת כז
KVXLWcMR en: Baal HaSulam => Letters => Letter 28 he: בעל הסולם => אגרות => אגרת כח
knjuDP7C en: Baal HaSulam => Letters => Letter 29 he: בעל הסולם => אגרות => אגרת כט
ptyFffh4 en: Baal HaSulam => Letters => Letter 30  he: בעל הסולם => אגרות => אגרת ל
lkxUNAGw en: Baal HaSulam => Letters => Letter 31 he: בעל הסולם => אגרות => אגרת לא
coFOcACU en: Baal HaSulam => Letters => Letter 32 he: בעל הסולם => אגרות => אגרת לב
lMdfAIaG en: Baal HaSulam => Letters => Letter 33 he: בעל הסולם => אגרות => אגרת לג
lchRFEiU en: Baal HaSulam => Letters => Letter 34 he: בעל הסולם => אגרות => אגרת לד
hYFEdxM0 en: Baal HaSulam => Letters => Letter 35 he: בעל הסולם => אגרות => אגרת לה
bycLz1Rw en: Baal HaSulam => Letters => Letter 36 he: בעל הסולם => אגרות => אגרת לו
6LRiFl7D en: Baal HaSulam => Letters => Letter 37 he: בעל הסולם => אגרות => אגרת לז
ocV7prBK en: Baal HaSulam => Letters => Letter 38  he: בעל הסולם => אגרות => אגרת לח
YsBDndDL en: Baal HaSulam => Letters => Letter 39 he: בעל הסולם => אגרות => אגרת לט
Ldgel5zV en: Baal HaSulam => Letters => Letter 40 he: בעל הסולם => אגרות => אגרת מ
ZLCc4nyD en: Baal HaSulam => Letters => Letter 41 he: בעל הסולם => אגרות => אגרת מא
KSOhDjL5 en: Baal HaSulam => Letters => Letter 42 he: בעל הסולם => אגרות => אגרת מב
u0Exuxtx en: Baal HaSulam => Letters => Letter 43 he: בעל הסולם => אגרות => אגרת מג
MBZFn0AR en: Baal HaSulam => Letters => Letter 44  he: בעל הסולם => אגרות => אגרת מד
emi7jolp en: Baal HaSulam => Letters => Letter 45 he: בעל הסולם => אגרות => אגרת מה
L6Cj1vk1 en: Baal HaSulam => Letters => Letter 46 he: בעל הסולם => אגרות => אגרת מו
o4BjSAcN en: Baal HaSulam => Letters => Letter 47  he: בעל הסולם => אגרות => אגרת מז
EBH9O7Bk en: Baal HaSulam => Letters => Letter 48 he: בעל הסולם => אגרות => אגרת מח
GW2ZAf5C en: Baal HaSulam => Letters => Letter 49  he: בעל הסולם => אגרות => אגרת מט
ib0w1sqv en: Baal HaSulam => Letters => Letter 50  he: בעל הסולם => אגרות => אגרת נ
hcBy8i3l en: Baal HaSulam => Letters => Letter 51 he: בעל הסולם => אגרות => אגרת נא
po2GCQfE en: Baal HaSulam => Letters => Letter 52 he: בעל הסולם => אגרות => אגרת נב
Ew63ydqt en: Baal HaSulam => Letters => Letter 53  he: בעל הסולם => אגרות => אגרת נג
nzHnNGkx en: Baal HaSulam => Letters => Letter 54 he: בעל הסולם => אגרות => אגרת נד
0syWpVDi en: Baal HaSulam => Letters => Letter 55 he: בעל הסולם => אגרות => אגרת נה
H6dVORc5 en: Baal HaSulam => Letters => Letter 56  he: בעל הסולם => אגרות => אגרת נו
0KJCqCgi en: Baal HaSulam => Letters => Letter 57  he: בעל הסולם => אגרות => אגרת נז
Dpk3cmId en: Baal HaSulam => Letters => Letter 58 he: בעל הסולם => אגרות => אגרת נח
oMq3uU8L en: Baal HaSulam => Letters => Letter 59 he: בעל הסולם => אגרות => אגרת נט
0QygF8Ib en: Baal HaSulam => Letters => Letter 60 he: בעל הסולם => אגרות => אגרת ס
ODkpSOQI en: Baal HaSulam => Letters => Letter 61 he: בעל הסולם => אגרות => אגרת סא
qMeV5M3Y en: Baal HaSulam => Articles he: בעל הסולם => מאמרים
R1vjNKZU en: Baal HaSulam => Articles => 600,000 Souls he: בעל הסולם => מאמרים => שישים ריבוא נשמות
2Ui1zkVO en: Baal HaSulam => Articles => A Handmaid Who Is Heir to Her Mistress he: בעל הסולם => מאמרים => שפחה כי תירש גבירתה
904O3Q4u en: Baal HaSulam => Articles => Anyone Who Is Sorry for the Public he: בעל הסולם => מאמרים => כל המצטער עם הציבור
VyDjtKiN en: Baal HaSulam => Articles => A Speech for the Completion of The Zohar he: בעל הסולם => מאמרים => מאמר לסיום הזוהר
ffgmgx4U en: Baal HaSulam => Articles => A Word of Truth he: בעל הסולם => מאמרים => דבר אמת
b7A99JPf en: Baal HaSulam => Articles => Body and Soul he: בעל הסולם => מאמרים => גוף ונפש
CASW48fT en: Baal HaSulam => Articles => Building the Future Society he: בעל הסולם => מאמרים => בנין החברה העתידית
6K5za3Hd en: Baal HaSulam => Articles => Concealment and Disclosure of the Face of the Creator - 1 he: בעל הסולם => מאמרים => הסתר וגילוי פנים של השי"ת - א
OaIuUScv en: Baal HaSulam => Articles => Concealment and Disclosure of the Face of the Creator - 2 he: בעל הסולם => מאמרים => הסתר וגילוי פנים של השי"ת - ב
2Lla7iBU en: Baal HaSulam => Articles => Disclosing a Portion, Covering Two he: בעל הסולם => מאמרים => גילוי טפח וכיסוי טפחיים
0Z2kNkRf en: Baal HaSulam => Articles => Exile and Redemption he: בעל הסולם => מאמרים => הגלות והגאולה
VvUkgQDO en: Baal HaSulam => Articles => Four Worlds he: בעל הסולם => מאמרים => ד' עולמות
DmGOtJlx en: Baal HaSulam => Articles => From My Flesh I Shall See God he: בעל הסולם => מאמרים => מבשרי אחזה אלוקי
5zNaHw76 en: Baal HaSulam => Articles => Inheritance of the Land he: בעל הסולם => מאמרים => ירושת הארץ
iYZIxsNe en: Baal HaSulam => Articles => Man’s Actions and Tactics he: בעל הסולם => מאמרים => פעולות האדם ותחבולותיו
2bscFWf4 en: Baal HaSulam => Articles => Matan Torah [The Giving of the Torah]  he: בעל הסולם => מאמרים => מתן תורה
5hgo3hY6 en: Baal HaSulam => Articles => Matter and Form in the Wisdom of Kabbalah he: בעל הסולם => מאמרים => החומר והצורה בחכמת הקבלה
Zo6BQz8E en: Baal HaSulam => Articles => Newspaper "The Nation" he: בעל הסולם => מאמרים => עיתון ״האומה״
r49DoqEl en: Baal HaSulam => Articles => Not the Time for the Livestock to Be Gathered he: בעל הסולם => מאמרים => לא עת האסף המקנה
vGdAkoI5 en: Baal HaSulam => Articles => One Commandment he: בעל הסולם => מאמרים => מצווה אחת
hqUTKcZz en: Baal HaSulam => Articles => Peace in the World he: בעל הסולם => מאמרים => השלום בעולם
AhwsJadS en: Baal HaSulam => Articles => Remembering he: בעל הסולם => מאמרים => סגולת זכירה
oMxAKcYA en: Baal HaSulam => Articles => Rewarded - I Will Hasten It; Not Rewarded - in Its Time he: בעל הסולם => מאמרים => זכו אחישנה, לא זכו בעתה
7pFJSNnu en: Baal HaSulam => Articles => Righteous and Wicked he: בעל הסולם => מאמרים => צדיקים ורשעים
R4dCCqrK en: Baal HaSulam => Articles => The Acting Mind he: בעל הסולם => מאמרים => שכל הפועל
itcVAcFn en: Baal HaSulam => Articles => The Arvut (Mutual Guarantee) he: בעל הסולם => מאמרים => הערבות
SObSZgT9 en: Baal HaSulam => Articles => The Essence of Religion and Its Purpose he: בעל הסולם => מאמרים => מהות הדת ומטרתה
DdbFBXFd en: Baal HaSulam => Articles => The Essence of the Wisdom of Kabbalah he: בעל הסולם => מאמרים => מהותה של חכמת הקבלה
4AtF9tGS en: Baal HaSulam => Articles => The Freedom he: בעל הסולם => מאמרים => מאמר החירות
qEIUUafR en: Baal HaSulam => Articles => The History of the Wisdom of Kabbalah he: בעל הסולם => מאמרים => תולדות חוכמת הקבלה
oIrPpKn2 en: Baal HaSulam => Articles => The Last Generation he: בעל הסולם => מאמרים => הדור האחרון
eKY6PhmO en: Baal HaSulam => Articles => The Love of God and the Love of Man he: בעל הסולם => מאמרים => אהבת ה' ואהבת הבריות
ZD0zSUIr en: Baal HaSulam => Articles => The Meaning of Conception and Birth he: בעל הסולם => מאמרים => סוד העיבור - לידה
SNRedHSE en: Baal HaSulam => Articles => The Meaning of His Names he: בעל הסולם => מאמרים => סוד שמותיו
5MwfWLG9 en: Baal HaSulam => Articles => The Meaning of the Chaf in Anochi he: בעל הסולם => מאמרים => סוד הכף דאנכי
28Cmp7gl en: Baal HaSulam => Articles => The Peace he: בעל הסולם => מאמרים => השלום
2TtUVozR en: Baal HaSulam => Articles => The Prophecy of Baal HaSulam he: בעל הסולם => מאמרים => נבואתו של בעל הסולם
pBOfaadG en: Baal HaSulam => Articles => The Quality of the Wisdom of the Hidden in General he: בעל הסולם => מאמרים => תכונתה של חכמת הנסתר בכללה
euK8EscW en: Baal HaSulam => Articles => The Shofar of the Messiah he: בעל הסולם => מאמרים => שופר של משיח
EBFcdqIV en: Baal HaSulam => Articles => The Solution he: בעל הסולם => מאמרים => הפתרון
CeULdtUD en: Baal HaSulam => Articles => The Teaching of the Kabbalah and Its Essence he: בעל הסולם => מאמרים => תורת הקבלה ומהותה
qcsfyhcb en: Baal HaSulam => Articles => The Wisdom of Israel Compared to External Wisdoms he: בעל הסולם => מאמרים => חכמת ישראל בערך חכמת חיצוניים
mnkZ8gjP en: Baal HaSulam => Articles => The Wisdom of Kabbalah and Philosophy he: בעל הסולם => מאמרים => חכמת הקבלה והפילוסופיה
ImXxdgf9 en: Baal HaSulam => Articles => This Is for Judah he: בעל הסולם => מאמרים => וזאת ליהודה
ww9syE82 en: Baal HaSulam => Articles => Time to Act he: בעל הסולם => מאמרים => עת לעשות
ga6rlGG2 en: Baal HaSulam => Articles => You Have Made Me in Behind and Before he: בעל הסולם => מאמרים => אחור וקדם צרתני
xtKmrbb9 en: Baal HaSulam => Study of the Ten Sefirot he: בעל הסולם => תלמוד עשר הספירות also called TES or תעס
e2jgBKLL en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 he: בעל הסולם => תלמוד עשר הספירות => כרך א'
9xNFLSSp en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א'
7scSATcZ en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Chapter 1 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => פרק א'
LIMg3y94 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Chapter 2 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => פרק ב'
rhRuFdIP en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => הסתכלות פנימית
YR9r5s6q en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => לוח השאלות לפירוש המלות
QCnCAagn en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => לוח השאלות לענינים
EiUPsO0e en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => לוח התשובות לפירוש המלות
nnGQFc43 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 1 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק א' => לוח התשובות לענינים
XlukqLH8 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב'
EgYCDrzH en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Chapter 1 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => פרק א'
hwh1BLAL en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Chapter 2 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => פרק ב'
PtDrdt9v en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => הסתכלות פנימית
ydlHgmBg en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => לוח השאלות לפירוש המלות
VBX3VORk en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => לוח השאלות לענינים
YhAlwjNh en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => לוח התשובות לפירוש המלות
F1dDm1OY en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 2 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ב' => לוח התשובות לענינים
AerA1hNN en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג'
37cdWCQP en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 1 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק א'
oL32UjM6 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 2 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ב'
p2bm9quF en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 3 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ג'
8ajjCTtg en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 4 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ד'
ulorFJ0c en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 5 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ה'
olYs3pHe en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 6 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ו'
kiTxkfet en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 7 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ז'
nZicYkes en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 8 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ח'
1bAtXKoc en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 9 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ט'
gqhAImdo en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 10 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק י'
AnVscbAJ en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 11 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק י"א
EnFXBYxy en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 12 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק י"ב
B8ZHQh1D en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 13 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק י"ג
i3Wim559 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 14 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק י"ד
nOLBngLq en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Chapter 15 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => פרק ט"ו
1yRKGJfc en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => הסתכלות פנימית
zCY8AVzl en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => לוח השאלות לפירוש המלות
S5mViX7z en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => לוח השאלות לענינים
4KFDFg3m en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => לוח התשובות לפירוש המלות
Levgq2jH en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 3 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ג' => לוח התשובות לענינים
1kDKQxJb en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד'
h97a60Mc en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Chapter 1 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => פרק א'
MGl4GjjS en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Chapter 2 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => פרק ב'
EU5TTtcy en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Chapter 3 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => פרק ג'
ldrFtcev en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Chapter 4 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => פרק ד'
RTzg7sXm en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Chapter 5 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => פרק ה'
jnRyYLeP en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Chapter 6 he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => פרק ו'
BjZJBha8 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => הסתכלות פנימית
PNEWuQYa en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => לוח השאלות לפירוש המלות
Ut8PE5aO en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => לוח התשובות לפירוש המלות
sL3XV5Dr en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => לוח השאלות לענינים
1t0TL11u en: Baal HaSulam => Study of the Ten Sefirot => Vol. 1 => Part 4 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך א' => חלק ד' => לוח התשובות לענינים
SoWJRPDf en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 he: בעל הסולם => תלמוד עשר הספירות => כרך ב'
o5lXptLo en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה'
4pmOtkWY en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 => Part 5 he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה' => חלק ה' עם אור פנימי
fSmz8o3A en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה' => לוח השאלות לפירוש המלות
uyAWnLqJ en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה' => לוח התשובות לפירוש המלות
2zI4yXlE en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה' => לוח השאלות לענינים
H9JMve9K en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה' => לוח התשובות לענינים
9Poika27 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 5 => Additional Explanation about the Matter of the Inversion of the Panim and the Making Order of the Kelim he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ה' => הוספת ביאור על ענין הפכת פנים, וסדר התהוות כלים
eNwJXy4s en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו'
DztxuIK7 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Part 6 he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => חלק ו' עם אור פנימי
aA8oiLQA en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => הסתכלות פנימית
Py2DZ1ES en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => סדר סבה ומסובב
kqEIM6f0 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => לוח השאלות לפירוש המלות
jOttKP5x en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => לוח התשובות לפירוש המלות
AkCMSd9U en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => לוח השאלות לענינים
tDNxLdgR en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => לוח התשובות לענינים
KGNYqv7h en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Questions Regarding Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => שאלות לסדר סבה ומסובב
HXvOiFNY en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 6 => Answers of Questions Regarding Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ו' => התשובות לסדר סבה ומסובב
ahipVtPu en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז'
FWrR48Bb en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Part 7 he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => חלק ז' עם אור פנימי
77Yq5OF7 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => הסתכלות פנימית
rutwuYR8 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => סדר סבה ומסובב
dgNJLnA7 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => לוח השאלות לפירוש המלות
P5q0Zmm6 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => לוח התשובות לפירוש המלות
hdywOExs en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => לוח השאלות לענינים
wOz46Ac9 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => לוח התשובות לענינים
teM5Yc6B en: Baal HaSulam => Study of the Ten Sefirot => Vol. 2 => Part 7 => Answer of Questions Regarding Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ב' => חלק ז' => התשובה לסדר סבה ומסובב
cJfmz0It en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 he: בעל הסולם => תלמוד עשר הספירות => כרך ג'
Pscnn3pP en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח'
NlYgsUxe en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Part 8 he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => חלק ח' עם אור פנימי
twSOJdmU en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => הסתכלות פנימית
OF3SI33X en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => סדר סבה ומסובב
JQn92bOZ en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => לוח השאלות לפירוש המלות
yBEWcZWe en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => לוח התשובות לפירוש המלות
yG5OG3aS en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => לוח השאלות לענינים
BeNvMLuu en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => לוח התשובות לענינים
qk1TqqNn en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 8 => Answer of Questions Regarding Cause and Consequence he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ח' => תשובת סבה ומסובב
Lfu7W3CD en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט'
z6MVO6UG en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 => Part 9 he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט' => חלק ט' עם אור פנימי
n74IfjUw en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט' => הסתכלות פנימית
oFfVR1sj en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט' => לוח השאלות לפירוש המלות
X714OgYa en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט' => לוח התשובות לפירוש המלות
56K73eNi en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט' => לוח השאלות לענינים
DhYu7bEh en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 9 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק ט' => לוח התשובות לענינים
n03vXCJl en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י'
1zYk2z7m en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 => Part 10 he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י' => חלק י' עם אור פנימי
L5xO0yDG en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י' => הסתכלות פנימית
9gSOlEJl en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י' => לוח השאלות לפירוש המלות
LRQKRR2e en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י' => לוח התשובות לפירוש המלות
ac3R2k8O en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י' => לוח השאלות לענינים
Vw89jmcY en: Baal HaSulam => Study of the Ten Sefirot => Vol. 3 => Part 10 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ג' => חלק י' => לוח התשובות לענינים
vFzrs05u en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 he: בעל הסולם => תלמוד עשר הספירות => כרך ד'
UGcGGSpP en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 11 he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"א
1kY2wOt9 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 11 => Part 11 he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"א => חלק י"א עם אור פנימי
eUDvJPEz en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 11 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"א => לוח השאלות לפירוש המלות
38p1N7aA en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 11 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"א => לוח התשובות לפירוש המלות
h5CQQmY5 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 11 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"א => לוח השאלות לענינים
awbSqHpG en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 11 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"א => לוח התשובות לענינים
NpLQT0LX en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב
OyVZGDKM en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 => Part 12 he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב => חלק י"ב עם אור פנימי
nO9GGoAX en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב => הסתכלות פנימית
XCjEhLDK en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב => לוח השאלות לפירוש המלות
ga3l0sjf en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב => לוח התשובות לפירוש המלות
0quwYeDZ en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב => לוח השאלות לענינים
1uquRur7 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 4 => Part 12 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ד' => חלק י"ב => לוח התשובות לענינים
lwWdDYpb en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 he: בעל הסולם => תלמוד עשר הספירות => כרך ה'
AUArdCkH en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג
TndRK0X8 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 => Part 13 he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג => חלק י"ג עם אור פנימי
s7NzunI1 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 => Inner Observation he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג => הסתכלות פנימית
w7qMVrQ0 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג => לוח השאלות לפירוש המלות
jdYR1m8g en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג => לוח התשובות לפירוש המלות
DDYm2kER en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג => לוח השאלות לענינים
JRgKUhzy en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 13 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ג => לוח התשובות לענינים
tit6XNAo en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 14 he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ד
LrYhEexG en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 14 => Part 14 he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ד => חלק י"ד עם אור פנימי
9Bh6AztV en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 14 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ד => לוח השאלות לפירוש המלות
2mI59NJA en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 14 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ד => לוח התשובות לפירוש המלות
oB3J4iLO en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 14 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ד => לוח השאלות לענינים
T2hJcy9X en: Baal HaSulam => Study of the Ten Sefirot => Vol. 5 => Part 14 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ה' => חלק י"ד => לוח התשובות לענינים
QryM4tO1 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 he: בעל הסולם => תלמוד עשר הספירות => כרך ו'
FaKUG7ru en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 15 he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ו
It7YBuab en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 15 => Part 15 he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ו => חלק ט"ו עם אור פנימי
feo7h6Fe en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 15 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ו => לוח השאלות לפירוש המלות
PtJHZD93 en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 15 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ו => לוח התשובות לפירוש המלות
ur26eJ0c en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 15 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ו => לוח השאלות לענינים
vpnmTC4W en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 15 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ו => לוח התשובות לענינים
mW6eON0z en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 16 he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ז
SU8TzXty en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 16 => Part 16 he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ז => חלק ט"ז עם אור פנימי
li1hvo4P en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 16 => Table of Questions for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ז => לוח השאלות לפירוש המלות
tcBbUnBZ en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 16 => Table of Answers for the Meaning of the Words he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ז => לוח התשובות לפירוש המלות
TaGXjokb en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 16 => Table of Questions for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ז => לוח השאלות לענינים
P727ETJu en: Baal HaSulam => Study of the Ten Sefirot => Vol. 6 => Part 16 => Table of Answers for Topics he: בעל הסולם => תלמוד עשר הספירות => כרך ו' => חלק ט"ז => לוח התשובות לענינים
qMUUn22b en: Baal HaSulam => Shamati he: בעל הסולם => שמעתי
hFeGidcS en: Baal HaSulam => Shamati => There Is None Else Besides Him he: בעל הסולם => שמעתי => אין עוד מלבדו
5zOm2XGW en: Baal HaSulam => Shamati => Shechina [Divinity] in Exile he: בעל הסולם => שמעתי => ענין שכינתא בגלותא
WrEIEnv7 en: Baal HaSulam => Shamati => The Matter of Spiritual Attainment he: בעל הסולם => שמעתי => ענין ההשגה הרוחנית
L6FUHzfm en: Baal HaSulam => Shamati =>  What Is the Reason for the Heaviness One Feels when Annulling before the Creator in the Work? he: בעל הסולם => שמעתי => מהו סיבת הכבידות, שהאדם מרגיש בבטול לה', בעבודה
1onNjaw2 en: Baal HaSulam => Shamati => Lishma Is an Awakening from Above, and Why Do We Need an Awakening from Below? he: בעל הסולם => שמעתי => לשמה זהו אתערותא דלעילא. ולמה צריכים אתערותא דלתתא?
Lwu1K6tc en: Baal HaSulam => Shamati => What Is Support in the Torah, in the Work? he: בעל הסולם => שמעתי => מהו סמכין בתורה, בעבודה
vPfTRoUl en: Baal HaSulam => Shamati => What Is, “A Habit Becomes a Second Nature,” in the Work? he: בעל הסולם => שמעתי => מהו, ההרגל נעשה טבע שני, בעבודה
hdNwa3MW en: Baal HaSulam => Shamati => What Is the Difference between a Shade of Kedusha and a Shade of Sitra Achra? he: בעל הסולם => שמעתי => מהו, הבדל בין צל דקדושה לצל דס"א
pTlets5W en: Baal HaSulam => Shamati => What Are Three Things that Broaden One’s Mind in the Work? he: בעל הסולם => שמעתי => מהו, ג' דברים שמרחיבים דעתו של אדם, בעבודה
pp4XA8ne en: Baal HaSulam => Shamati => What Is “Hurry, My Beloved,” in the Work? he: בעל הסולם => שמעתי => מהו, ברח דודי, בעבודה
ufeCjuVV en: Baal HaSulam => Shamati => Joy with Trembling he: בעל הסולם => שמעתי => ענין גילה ברעדה, בעבודה
vZbqW4mZ en: Baal HaSulam => Shamati => The Essence of Man’s Work he: בעל הסולם => שמעתי => עיקר עבודת האדם
hrc0fZLf en: Baal HaSulam => Shamati => A Pomegranate he: בעל הסולם => שמעתי => ענין רמון
KVBSNnDA en: Baal HaSulam => Shamati => What Is the Exaltedness of the Creator? he: בעל הסולם => שמעתי => מהו רוממות ה'
JMcmIr1P en: Baal HaSulam => Shamati => What Is Other Gods in the Work? he: בעל הסולם => שמעתי => מהו, אלהים אחרים, בעבודה
JpjAhvAQ en: Baal HaSulam => Shamati => What Is the Day of the Lord and the Night of the Lord, in the Work? he: בעל הסולם => שמעתי => מהו יום ה' וליל ה', בעבודה
tS8lJn2r en: Baal HaSulam => Shamati => What Does It Mean that the Sitra Achra Is Called “Malchut without a Crown”? he: בעל הסולם => שמעתי => מהו, שהס"א נקראת, מלכותא בלי תגא
3geW64zG en: Baal HaSulam => Shamati => My Soul Shall Weep in Secret – 1 he: בעל הסולם => שמעתי => מהו, במסתרים תבכה נפשי, בעבודה - א
tk2qRJuw en: Baal HaSulam => Shamati => What Is “The Creator Hates the Bodies,” in the Work? he: בעל הסולם => שמעתי => מהו, שהקב"ה שונא את הגופים, בעבודה
UEikDufF en: Baal HaSulam => Shamati => Lishma [for Her sake] he: בעל הסולם => שמעתי => ענין לשמה
rFTvkF9Q en: Baal HaSulam => Shamati => When One Feels Oneself in a State of Ascent he: בעל הסולם => שמעתי => בזמן שהאדם מרגיש את עצמו בבחינת עליה
KxcMnarm en: Baal HaSulam => Shamati => Torah Lishma he: בעל הסולם => שמעתי => תורה לשמה
oGFs0t6h en: Baal HaSulam => Shamati => You Who Love the Lord, Hate Evil he: בעל הסולם => שמעתי => אוהבי ה' שנאו רע
EUiMc0WV en: Baal HaSulam => Shamati => He Will Save Them from the Hand of the Wicked he: בעל הסולם => שמעתי => מיד רשעים יצילם
zFvyyADm en: Baal HaSulam => Shamati => Things that Come from the Heart he: בעל הסולם => שמעתי => דברים היוצאים מהלב
wdKyLHUi en: Baal HaSulam => Shamati => One’s Future Depends and Is Tied to Gratitude for the Past he: בעל הסולם => שמעתי => העתיד של האדם תלוי וקשור בהודאה על העבר
e7VJ9nJs en: Baal HaSulam => Shamati => What Is “The Lord Is High and the Low Will See”? - 1 he: בעל הסולם => שמעתי => מהו רם ה' ושפל יראה - א
AlcJhnYS en: Baal HaSulam => Shamati => I Shall Not Die but Live he: בעל הסולם => שמעתי => לא אמות כי אחיה
cBo1eIEb en: Baal HaSulam => Shamati => When Thoughts Come to a Person he: בעל הסולם => שמעתי => כשבאים הרהורים לאדם
IKjYS3I4 en: Baal HaSulam => Shamati => The Most Important Is to Want Only to Bestow he: בעל הסולם => שמעתי => עיקר לרצות רק להשפיע
An4SjeQ8 en: Baal HaSulam => Shamati => Anyone Who Pleases the Spirit of the People he: בעל הסולם => שמעתי => כל שרוח הבריות נוח הימנו
wGBT8d17 en: Baal HaSulam => Shamati => A Lot Is an Awakening from Above he: בעל הסולם => שמעתי => גורל הוא סוד אתערותא דלעילא
ppows6nq en: Baal HaSulam => Shamati => The Lots on Yom Kippur and with Haman he: בעל הסולם => שמעתי => ענין גורלות, שהיה ביום כפורים, ואצל המן
HN4sUqKR en: Baal HaSulam => Shamati => The Advantage of a Land he: בעל הסולם => שמעתי => יתרון ארץ בכל הוא
F63UqPv7 en: Baal HaSulam => Shamati => Concerning the Vitality of Kedusha he: בעל הסולם => שמעתי => בענין החיות דקדושה
sqCAWyUT en: Baal HaSulam => Shamati => What Are the Three Bodies in Man? he: בעל הסולם => שמעתי => מהו, ג' בחינות גופים באדם
nzvEGISM en: Baal HaSulam => Shamati => An Article for Purim he: בעל הסולם => שמעתי => מאמר לפורים
CjOoVMfG en: Baal HaSulam => Shamati => The Fear of God Is His Treasure he: בעל הסולם => שמעתי => יראת ה' הוא אוצרו
rKYq43C2 en: Baal HaSulam => Shamati => And They Sewed Fig Leaves he: בעל הסולם => שמעתי => ויתפרו עלה תאנה
czTv8O3p en: Baal HaSulam => Shamati => What Is the Measure of Faith in the Rav? he: בעל הסולם => שמעתי => אמונת רבו, מהו השיעור
C6PNfOkA en: Baal HaSulam => Shamati => What Is Greatness and Smallness in Faith? he: בעל הסולם => שמעתי => מהו קטנות וגדלות באמונה
cDko5YMK en: Baal HaSulam => Shamati => What Is the Acronym Elul in the Work? he: בעל הסולם => שמעתי => מהו, שראשי תיבות אלול "אני לדודי ודודי לי" מרמזת בעבודה
NhxfSmLU en: Baal HaSulam => Shamati => Concerning Truth and Faith he: בעל הסולם => שמעתי => ענין אמת ואמונה
8lPKkxCe en: Baal HaSulam => Shamati => Mind and Heart he: בעל הסולם => שמעתי => מוחא ולבא
VOYXPJcN en: Baal HaSulam => Shamati => Two Discernments in the Torah and in the Work he: בעל הסולם => שמעתי => ב' בחינות בתורה ובעבודה
DBRyBIGo en: Baal HaSulam => Shamati => The Domination of Israel over the Klipot he: בעל הסולם => שמעתי => שליטת ישראל על הקליפות
iQoFjhJ4 en: Baal HaSulam => Shamati => In the Place Where You Find His Greatness he: בעל הסולם => שמעתי => במקום שאתה מוצא גדלותו
5lTLSB7e en: Baal HaSulam => Shamati => The Primary Basis he: בעל הסולם => שמעתי => עיקר היסוד
VxQwkWmA en: Baal HaSulam => Shamati => The Most Important Are the Mind and the Heart he: בעל הסולם => שמעתי => עיקר הוא מוחא וליבא
RmNXT0Kp en: Baal HaSulam => Shamati => Two States he: בעל הסולם => שמעתי => שני מצבים
KnFUnp3u en: Baal HaSulam => Shamati => If You Encounter This Villain he: בעל הסולם => שמעתי => אם פגע בך מנוול זה
paz3tF1U en: Baal HaSulam => Shamati => A Transgression Does Not Extinguish a Mitzva he: בעל הסולם => שמעתי => אין עבירה מכבה מצווה
X3vhrRJz en: Baal HaSulam => Shamati => The Matter of Limitation he: בעל הסולם => שמעתי => ענין הגבלה
n4KHSc3c en: Baal HaSulam => Shamati => The Purpose of the Work – 1 he: בעל הסולם => שמעתי => מטרת העבודה - א
328te3TV en: Baal HaSulam => Shamati => Haman from the Torah, from Where? he: בעל הסולם => שמעתי => המן מן התורה מנין
Z4GqnV9R en: Baal HaSulam => Shamati => Torah Is Called Indication he: בעל הסולם => שמעתי => תורה נקרא יורה
oD6vzC4X en: Baal HaSulam => Shamati => Will Bring Him Closer to His Will he: בעל הסולם => שמעתי => יקריב אותו לרצונו
r5uZ7K0C en: Baal HaSulam => Shamati => Joy Is a “Reflection” of Good Deeds he: בעל הסולם => שמעתי => השמחה היא בחינת "מראה" ממעשים טובים
8jddbZDS en: Baal HaSulam => Shamati => Concerning the Rod and the Serpent he: בעל הסולם => שמעתי => ענין מטה ונחש
gFZZSmTY en: Baal HaSulam => Shamati => A Mitzva that Comes through Transgression he: בעל הסולם => שמעתי => מצוה הבאה בעבירה
WAGG71GI en: Baal HaSulam => Shamati => Round About Him It Storms Mightily he: בעל הסולם => שמעתי => וסביביו נשערה מאד
xGyY0WF1 en: Baal HaSulam => Shamati => Descends and Incites, Ascends and Complains he: בעל הסולם => שמעתי => יורד ומסית עולה ומקטרג
piMNPL1j en: Baal HaSulam => Shamati => I Was Borrowed on, and I Repay he: בעל הסולם => שמעתי => לוו עלי ואני פורע
tLVuQ6Hh en: Baal HaSulam => Shamati => From Lo Lishma, We Come to Lishma he: בעל הסולם => שמעתי => מתוך שלא לשמה באים לשמה
8SjQLMI2 en: Baal HaSulam => Shamati => Concerning the Revealed and the Concealed he: בעל הסולם => שמעתי => ענין נגלה וענין נסתר
Wn2mmIpk en: Baal HaSulam => Shamati => Concerning the Giving of the Torah – 1 he: בעל הסולם => שמעתי => ענין מתן תורה - א
DNEFmmo6 en: Baal HaSulam => Shamati => Depart from Evil he: בעל הסולם => שמעתי => סור מרע
WHhza84o en: Baal HaSulam => Shamati => Man's Connection to the Sefirot he: בעל הסולם => שמעתי => קשר האדם אל הספירות
5eMML0AF en: Baal HaSulam => Shamati => First Will Be the Correction of the World he: בעל הסולם => שמעתי => מקודם יהיה תיקון העולם
sdt6n9yj en: Baal HaSulam => Shamati => With a Mighty Hand and with Fury Poured Out he: בעל הסולם => שמעתי => ביד חזקה ובחימה שפוכה
ENQy5Rdc en: Baal HaSulam => Shamati => My Soul Shall Weep in Secret – 2 he: בעל הסולם => שמעתי => במסתרים תבכה נפשי - ב
SUgA2j0W en: Baal HaSulam => Shamati => Confidence Is the Clothing for the Light he: בעל הסולם => שמעתי => הבטחון הוא הלבוש להאור
WQnQIHT5 en: Baal HaSulam => Shamati => After the Tzimtzum he: בעל הסולם => שמעתי => לאחר הצמצום
3Ljoqo9f en: Baal HaSulam => Shamati => World, Year, Soul he: בעל הסולם => שמעתי => ענין עולם שנה נפש
bwauq5dB en: Baal HaSulam => Shamati => There Is a Discernment of the Next World, and There Is a Discernment of This World he: בעל הסולם => שמעתי => יש בחינת עולם הבא, ויש בחינת עולם הזה
cVnwhXZS en: Baal HaSulam => Shamati => On All Your Offerings You Shall Offer Salt he: בעל הסולם => שמעתי => על כל קרבנך תקריב מלח
gPNDiV2G en: Baal HaSulam => Shamati => One's Soul Shall Teach Him he: בעל הסולם => שמעתי => נשמת אדם תלמדנו
hNvHwQkn en: Baal HaSulam => Shamati => The Torah, the Creator, and Israel Are One he: בעל הסולם => שמעתי => אורייתא וקב"ה וישראל חד הוא 
tqlsaaHO en: Baal HaSulam => Shamati => Atzilut and BYA he: בעל הסולם => שמעתי => אצילות ובי"ע
I9lmXFSw en: Baal HaSulam => Shamati => Concerning Achor be Achor he: בעל הסולם => שמעתי => ענין אחור באחור
CvtxED7l en: Baal HaSulam => Shamati => Concerning Raising MAN he: בעל הסולם => שמעתי => ענין העלאת מ"ן
opauHcDq en: Baal HaSulam => Shamati => The Prayer that One Should Always Pray he: בעל הסולם => שמעתי => התפילה שצריכין להתפלל תמיד
U1hvL47u en: Baal HaSulam => Shamati => Concerning the Right Vav and the Left Vav he: בעל הסולם => שמעתי => ענין ו' ימינית, ו' שמאלית
BuGLRvNX en: Baal HaSulam => Shamati => What Is “He Drove the Man Out of the Garden of Eden so He Would Not Take from the Tree of Life”? he: בעל הסולם => שמעתי => מהו, ויגרש את האדם מגן עדן, מטעם שלא יקח מעץ החיים
8VrXchag en: Baal HaSulam => Shamati => What Is the Fruit of a Citrus Tree, in the Work? he: בעל הסולם => שמעתי => מהו, פרי עץ הדר, בעבודה
HhX01FJm en: Baal HaSulam => Shamati => And They Built Arei Miskenot he: בעל הסולם => שמעתי => ויבן ערי מסכנות
ZZwECbC2 en: Baal HaSulam => Shamati => Shabbat Shekalim he: בעל הסולם => שמעתי => שבת שקלים
nokxC20b en: Baal HaSulam => Shamati => All the Work Is Only Where There Are Two Ways – 1 he: בעל הסולם => שמעתי => כל העבודה הוא רק במקום שיש ב' דרכים - א
u4TdgN6t en: Baal HaSulam => Shamati => To Understand the Words of The Zohar he: בעל הסולם => שמעתי => בכדי להבין את דברי הזה"ק
GpKjDPjJ en: Baal HaSulam => Shamati => In The Zohar, Beresheet he: בעל הסולם => שמעתי => בזהר בראשית
rpODNrgD en: Baal HaSulam => Shamati => Concerning the Replaceable he: בעל הסולם => שמעתי => ענין בני תמורה
bzwCjyqk en: Baal HaSulam => Shamati => Explaining the Discernment of Luck he: בעל הסולם => שמעתי => ביאור לבחינת מזלא
k6vNo4hw en: Baal HaSulam => Shamati => Concerning Fins and Scales he: בעל הסולם => שמעתי => ענין סנפיר וקשקשת
YlsVSfQd en: Baal HaSulam => Shamati => And You Shall Keep Your Souls he: בעל הסולם => שמעתי => ושמרתם את נפשותיכם
7MOmfqor en: Baal HaSulam => Shamati => Concerning Removing the Foreskin he: בעל הסולם => שמעתי => ענין הסרת הערלה
cGO0h5sW en: Baal HaSulam => Shamati => What Is Waste of Barn and Winery, in the Work? he: בעל הסולם => שמעתי => מהו פסולת גורן ויקב, בעבודה
u7CWhNpz en: Baal HaSulam => Shamati => Waste of Barn and Winery he: בעל הסולם => שמעתי => ענין פסולת גורן ויקב
Ftl9v7iT en: Baal HaSulam => Shamati => Spirituality Is Called That Which Will Never Be Lost he: בעל הסולם => שמעתי => רוחניות נקרא, מה שלא יתבטל לעולם
NBba9GTc en: Baal HaSulam => Shamati => He Did Not Say Wicked or Righteous he: בעל הסולם => שמעתי => רשע או צדיק לא קאמר
N505b8uM en: Baal HaSulam => Shamati => The Written Torah and the Oral Torah – 1 he: בעל הסולם => שמעתי => תורה שבכתב ותורה שבעל פה - א
PW905OyJ en: Baal HaSulam => Shamati => A Commentary on the Psalm, “For the Winner over Roses” he: בעל הסולם => שמעתי => ביאור להזמר "למנצח על שושנים"
av4R4Ve6 en: Baal HaSulam => Shamati => And You Shall Take You the Fruit of a Citrus Tree he: בעל הסולם => שמעתי => ולקחתם לכם פרי עץ הדר
jRNM0kiY en: Baal HaSulam => Shamati => Whose Heart Makes Him Willing he: בעל הסולם => שמעתי => ידבנו לבו
OuMzibSy en: Baal HaSulam => Shamati => And the Saboteur Was Sitting he: בעל הסולם => שמעתי => והמחבל הוי יתיב
IQDT5zRM en: Baal HaSulam => Shamati => A Bastard Wise Disciple Precedes a Commoner High Priest he: בעל הסולם => שמעתי => ממזר תלמיד חכם קודם לכהן גדול עם הארץ
KZHWtyep en: Baal HaSulam => Shamati => What the Twelve Challahs on Shabbat Imply he: בעל הסולם => שמעתי => מהו הרמז של י"ב חלות בשבת
7QxvUnFf en: Baal HaSulam => Shamati => Concerning the Two Angels he: בעל הסולם => שמעתי => ענין ב' המלאכים
Csvhsilm en: Baal HaSulam => Shamati => If You Leave Me One Day, I Will Leave You Two he: בעל הסולם => שמעתי => אם תעזבני יום, יומים אעזבך
tqSroCkI en: Baal HaSulam => Shamati => Two Kinds of Meat he: בעל הסולם => שמעתי => ב' מיני בשר
d5b6LPDs en: Baal HaSulam => Shamati => A Field that the Lord Has Blessed he: בעל הסולם => שמעתי => שדה אשר ברכו ה'
O9z1hCRl en: Baal HaSulam => Shamati => Breath, Sound, and Speech he: בעל הסולם => שמעתי => הבל, קול ודיבור
UKbi6hAy en: Baal HaSulam => Shamati => The Three Angels he: בעל הסולם => שמעתי => שלשת המלאכים
KPVSYTK7 en: Baal HaSulam => Shamati => The Eighteen Prayer he: בעל הסולם => שמעתי => תפילת שמונה עשרה
Wln5Y29a en: Baal HaSulam => Shamati => Prayer he: בעל הסולם => שמעתי => ענין תפילה
7rKlp6ua en: Baal HaSulam => Shamati => Still, Vegetative, Animate, and Speaking he: בעל הסולם => שמעתי => ענין דומם, צומח, חי, מדבר
K8L76zp4 en: Baal HaSulam => Shamati => He Who Said, “Mitzvot Do Not Require Intention” he: בעל הסולם => שמעתי => למאן דאמר מצוות אין צריכות כוונה
nyEk9Wu8 en: Baal HaSulam => Shamati => You Labored and Did Not Find, Do Not Believe he: בעל הסולם => שמעתי => יגעת ולא מצאת אל תאמין
VMJzzr6o en: Baal HaSulam => Shamati => To Understand the Matter of the Knees Which Have Bowed to Baal he: בעל הסולם => שמעתי => להבין ענין ברכים אשר כרעו לבעל
4uYa8NZ8 en: Baal HaSulam => Shamati => That Disciple Who Learned in Secret he: בעל הסולם => שמעתי => ההוא תלמיד דלמד בחשאי
pkQ8orJv en: Baal HaSulam => Shamati => The Reason for Not Eating Nuts on Rosh Hashanah he: בעל הסולם => שמעתי => טעם על מנהג שלא אוכלין אגוזים בראש השנה
ZNNbdDP5 en: Baal HaSulam => Shamati => She Is Like Merchant-Ships he: בעל הסולם => שמעתי => היתה כאניות סוחר
t4MJ36g8 en: Baal HaSulam => Shamati => Understanding What Is Written in Shulchan Aruch he: בעל הסולם => שמעתי => להבין מה שמבואר בשולחן ערוך
RGx0jQI1 en: Baal HaSulam => Shamati => His Divorce and His Hand Come as One he: בעל הסולם => שמעתי => ענין גיטו וידו באין כאחד
CqYrKzR2 en: Baal HaSulam => Shamati => A Shabbat of Beresheet and of the Six Thousand Years he: בעל הסולם => שמעתי => שבת בראשית - ודשיתא אלפי שני
55LYqXPX en: Baal HaSulam => Shamati => He Who Delights the Shabbat he: בעל הסולם => שמעתי => המענג את השבת
OPndqRM0 en: Baal HaSulam => Shamati => A Sage Comes to Town he: בעל הסולם => שמעתי => חכם בא לעיר
9OBAXhlB en: Baal HaSulam => Shamati => The Difference between Core, Self, and Added Abundance he: בעל הסולם => שמעתי => להבין ההפרש בין עיקר ועצמות, ותוספת שפע
voeMqIXC en: Baal HaSulam => Shamati => Dew Drips from that Galgalta to Zeir Anpin he: בעל הסולם => שמעתי => מהאי גלגלתא נטיף טלא לז"א
4sU63POC en: Baal HaSulam => Shamati => The Shechina in the Dust he: בעל הסולם => שמעתי => בחינת שכינתא בעפרא
jKdw2a8j en: Baal HaSulam => Shamati => Tiberias of Our Sages, Good Is Your Sight he: בעל הסולם => שמעתי => טבריא דרז"ל, טובה ראיתך
NtpGN0xc en: Baal HaSulam => Shamati => Who Comes to Purify he: בעל הסולם => שמעתי => הבא לטהר
sbIzSvPu en: Baal HaSulam => Shamati => In the Sweat of Your Face Shall You Eat Bread – 1 he: בעל הסולם => שמעתי => בזיעת אפיך תאכל לחם - א
mIXBB2s9 en: Baal HaSulam => Shamati => The Lights of Shabbat he: בעל הסולם => שמעתי => אורות דשבת
67UYG1Aq en: Baal HaSulam => Shamati => Wine that Causes Drunkenness he: בעל הסולם => שמעתי => יין המשכר
WQXaPzwD en: Baal HaSulam => Shamati => Clean and Righteous Do Not Kill he: בעל הסולם => שמעתי => נקי וצדיק אל תהרוג
fZlK2OH1 en: Baal HaSulam => Shamati => The Difference between the First Letters and the Last Letters he: בעל הסולם => שמעתי => החילוק בין אגרות הראשונות לאגרות האחרונות
vQlJCvQG en: Baal HaSulam => Shamati => Zelophehad Was Gathering Wood he: בעל הסולם => שמעתי => צלפחד היה מקושש עצים
udjDlN06 en: Baal HaSulam => Shamati => Concerning Fear that Sometimes Comes Upon a Person he: בעל הסולם => שמעתי => ענין יראה ופחד שבא לפעמים להאדם
HluhsVG8 en: Baal HaSulam => Shamati => The Difference between the Six Workdays and Shabbat he: בעל הסולם => שמעתי => הבדל מששת ימי המעשה לשבת
lwkmSZGa en: Baal HaSulam => Shamati => How I Love Your Torah he: בעל הסולם => שמעתי => מה אהבתי תורתך
6gw77tds en: Baal HaSulam => Shamati => The Holiday of Passover he: בעל הסולם => שמעתי => ענין חג הפסח
rvDFQFsP en: Baal HaSulam => Shamati => The Essence of the War he: בעל הסולם => שמעתי => עיקר המלחמה
0XV5DAz5 en: Baal HaSulam => Shamati => Only Good to Israel he: בעל הסולם => שמעתי => אך טוב לישראל
ejs92F2y en: Baal HaSulam => Shamati => There Is a Certain People he: בעל הסולם => שמעתי => ישנו עם אחד
xpK9RH59 en: Baal HaSulam => Shamati => What Is He Will Give Wisdom Specifically to the Wise he: בעל הסולם => שמעתי => מהו יהיב חכמתא לחכימין דוקא
ocFpiso2 en: Baal HaSulam => Shamati => A Commentary on The Zohar he: בעל הסולם => שמעתי => פירוש על זהר
apeOy2Tr en: Baal HaSulam => Shamati => The Work of Reception and Bestowal he: בעל הסולם => שמעתי => ענין העבודה של קבלה והשפעה
Q0LefNOT en: Baal HaSulam => Shamati => The Scrutiny of Bitter and Sweet, True and False he: בעל הסולם => שמעתי => יש בירור מר ומתוק, אמת ושקר
Oi5iT82l en: Baal HaSulam => Shamati => Why We Need to Extend Hochma he: בעל הסולם => שמעתי => למה צריכים להמשיך בחינת חכמה
LRl1l6Fr en: Baal HaSulam => Shamati => Sing unto the Lord, for He Has Done Pride he: בעל הסולם => שמעתי => זמרו לה', כי גאות עשה
qmQTeblv en: Baal HaSulam => Shamati => And Israel Saw the Egyptians he: בעל הסולם => שמעתי => וירא ישראל את מצרים
R96Onp9a en: Baal HaSulam => Shamati => For Bribe Blinds the Eyes of the Wise he: בעל הסולם => שמעתי => כי השוחד יעור עיני חכמים
4GW5T0i5 en: Baal HaSulam => Shamati => A Thought Is a Result of the Desire he: בעל הסולם => שמעתי => המחשבה היא תולדה מהרצון
JLSdNwC2 en: Baal HaSulam => Shamati => There Cannot Be an Empty Space in the World he: בעל הסולם => שמעתי => אי אפשר להיות חלל ריק בעולם
L85OaIfl en: Baal HaSulam => Shamati => The Cleanness of the Body he: בעל הסולם => שמעתי => נקיות הגוף
2DgydMsJ en: Baal HaSulam => Shamati => Lest He Took from the Tree of Life he: בעל הסולם => שמעתי => פן לקח מעץ החיים
u841sfZX en: Baal HaSulam => Shamati => I Am Asleep but My Heart Is Awake he: בעל הסולם => שמעתי => אני ישנה וליבי ער
gDJwrcQC en: Baal HaSulam => Shamati => The Reason for Not Eating at Each Other's Home on Passover he: בעל הסולם => שמעתי => טעם שלא נוהגים לאכול אחד אצל השני בפסח
TarnTofh en: Baal HaSulam => Shamati => And It Came to Pass in the Course of Those Many Days he: בעל הסולם => שמעתי => ויהי בימים הרבים ההם
hjq3z1FI en: Baal HaSulam => Shamati => The Reason for Concealing the Matzot he: בעל הסולם => שמעתי => טעם הצנע במצות
KOCRM1mS en: Baal HaSulam => Shamati => Concerning the Giving of the Torah – 2 he: בעל הסולם => שמעתי => ענין מתן תורה - ב
Ahc3hYSM en: Baal HaSulam => Shamati => Concerning the Hazak We Say After Completing the Series he: בעל הסולם => שמעתי => ענין "חזק" שאומרים אחר סיום הסדרה
srUl7uF1 en: Baal HaSulam => Shamati => What the Authors of The Zohar Said he: בעל הסולם => שמעתי => ענין מה שאמרו בעלי זהר
Pu6DpR9r en: Baal HaSulam => Shamati => There Is a Difference between Corporeality and Spirituality he: בעל הסולם => שמעתי => יש הפרש בין גשמיות לרוחניות
buvvjKh2 en: Baal HaSulam => Shamati => An Explanation to Elisha's Request of Elijah he: בעל הסולם => שמעתי => ביאור לבקשת אלישע מאליהו
s0lcc226 en: Baal HaSulam => Shamati => Two Discernments in Attainment he: בעל הסולם => שמעתי => ב' בחינות בהשגה
H1bzRHvu en: Baal HaSulam => Shamati => The Reason Why It Is Called Shabbat Teshuva he: בעל הסולם => שמעתי => טעם למה שקורין שבת תשובה
wrx3kWbJ en: Baal HaSulam => Shamati => The Customs of Israel he: בעל הסולם => שמעתי => מנהגי ישראל
nfrc5UUk en: Baal HaSulam => Shamati => Concerning a Complete Righteous he: בעל הסולם => שמעתי => ענין צדיק גמור
OyJ3OJRE en: Baal HaSulam => Shamati => You Shall Not Have in Your Pocket a Big Stone he: בעל הסולם => שמעתי => לא יהיה בכיסך אבן גדולה
C92p0RKS en: Baal HaSulam => Shamati => In The Zohar, Emor – 1 he: בעל הסולם => שמעתי => זהר, אמור - א
a74oxCyl en: Baal HaSulam => Shamati => The Matter of Preventions and Delays he: בעל הסולם => שמעתי => ענין המניעות והעיכובים
177BUIlc en: Baal HaSulam => Shamati => Why We Say LeChaim he: בעל הסולם => שמעתי => מדוע אומרים לחיים
qxtEj1Ii en: Baal HaSulam => Shamati => Concealment he: בעל הסולם => שמעתי => ענין הסתר
B7euDRtR en: Baal HaSulam => Shamati => And If the Way Be Too Far for You he: בעל הסולם => שמעתי => והיה כי ירחק ממך
nhqEB8h5 en: Baal HaSulam => Shamati => When Drinking Brandy after the Havdala he: בעל הסולם => שמעתי => בעת שתיית י"ש אחר ההבדלה
zZOOI09z en: Baal HaSulam => Shamati => Atonements he: בעל הסולם => שמעתי => ענין כפרות
q81nS7zb en: Baal HaSulam => Shamati => Three Partners in Man he: בעל הסולם => שמעתי => ענין ג' שותפין באדם
d9ZzhYwI en: Baal HaSulam => Shamati => Three Lines he: בעל הסולם => שמעתי => ענין ג' קוין
u1BNoYRX en: Baal HaSulam => Shamati => In The Zohar, Emor – 2 he: בעל הסולם => שמעתי => בזהר, אמור - ב
JB67bfDs en: Baal HaSulam => Shamati => Honor he: בעל הסולם => שמעתי => ענין כבוד
hnN3Dh1K en: Baal HaSulam => Shamati => Moses and Solomon he: בעל הסולם => שמעתי => משה ושלמה
xbEQtpbv en: Baal HaSulam => Shamati => The Discernment of Messiah he: בעל הסולם => שמעתי => בחינת משיח
C5ZC7xiW en: Baal HaSulam => Shamati => The Difference between Faith and Intellect he: בעל הסולם => שמעתי => ההבדל בין אמונה להשכל
H9D2moKn en: Baal HaSulam => Shamati => The Uneducated, the Fear of Shabbat Is on Him he: בעל הסולם => שמעתי => עם הארץ, אימת שבת עליו
qS4HuQIM en: Baal HaSulam => Shamati => Make Your Shabbat a Weekday, and Do Not Need People he: בעל הסולם => שמעתי => עשה שבתך חול ואל תצטרך לבריות
8uy1xBc2 en: Baal HaSulam => Shamati => Choosing Labor he: בעל הסולם => שמעתי => להכריע ביגיעה
GJSMibUd en: Baal HaSulam => Shamati => All the Work Is Only Where There Are Two Ways – 2 he: בעל הסולם => שמעתי => כל העבודה היא רק במקום שיש ב' דרכים - ב
XjuvEsCA en: Baal HaSulam => Shamati => The Action Affects the Thought he: בעל הסולם => שמעתי => המעשה פועל על המחשבה
wew3rcLq en: Baal HaSulam => Shamati => Every Act Leaves an Imprint he: בעל הסולם => שמעתי => כל פעולה עושה רושם
Y2kjYKfx en: Baal HaSulam => Shamati => The Time of Descent he: בעל הסולם => שמעתי => זמן הירידה
UPyGUH53 en: Baal HaSulam => Shamati => The Lots he: בעל הסולם => שמעתי => ענין הגורלות
hnBUEevl en: Baal HaSulam => Shamati => One Wall Serves Both he: בעל הסולם => שמעתי => ענין כותל אחד משמש לשניהם
RPJQltgy en: Baal HaSulam => Shamati => The Complete Seven he: בעל הסולם => שמעתי => ז' שלמים
0zNAuA0C en: Baal HaSulam => Shamati => Rewarded - I Will Hasten It he: בעל הסולם => שמעתי => זכו אחישנה
2hdqjOhH en: Baal HaSulam => Shamati => A Grip for the External Ones he: בעל הסולם => שמעתי => אחיזה לחיצונים
q6qVNKpW en: Baal HaSulam => Shamati => Book, Author, Story he: בעל הסולם => שמעתי => ספר סופר סיפור
Up40AuPP en: Baal HaSulam => Shamati => Freedom he: בעל הסולם => שמעתי => חירות
XzkCbPBH en: Baal HaSulam => Shamati => To Every Man of Israel he: בעל הסולם => שמעתי => לכל איש ישראל
hgiUBUTX en: Baal HaSulam => Shamati => The Hizdakchut of the Masach he: בעל הסולם => שמעתי => הזדככות המסך
2jNsmxu8 en: Baal HaSulam => Shamati => Spirituality and Corporeality he: בעל הסולם => שמעתי => רוחניות וגשמיות
9IuOLWLz en: Baal HaSulam => Shamati => In the Sweat of Your Face Shall You Eat Bread – 2 he: בעל הסולם => שמעתי => בזיעת אפיך תאכל לחם - ב
n2Mihfdf en: Baal HaSulam => Shamati => Man's Pride Shall Bring Him Low he: בעל הסולם => שמעתי => גאות אדם תשפלנו
xR8XABaM en: Baal HaSulam => Shamati => The Purpose of the Work - 2 he: בעל הסולם => שמעתי => מטרת העבודה - ב
Ul5U2DhU en: Baal HaSulam => Shamati => Wisdom Cries Out in the Streets he: בעל הסולם => שמעתי => החכמה בחוץ תרונה
resVnuKb en: Baal HaSulam => Shamati => Faith and Pleasure he: בעל הסולם => שמעתי => ענין אמונה ותענוג
ChELpgJB en: Baal HaSulam => Shamati => Receiving in order to Bestow he: בעל הסולם => שמעתי => ענין קבלה להשפיע
bZVcZYjT en: Baal HaSulam => Shamati => Labor he: בעל הסולם => שמעתי => ענין היגיעה
QnxFkVOq en: Baal HaSulam => Shamati => Three Conditions in Prayer he: בעל הסולם => שמעתי => ג' תנאים בתפילה
M53Vu2o2 en: Baal HaSulam => Shamati => A Sightly Flaw in You he: בעל הסולם => שמעתי => מום יפה שבך
Kn0se4td en: Baal HaSulam => Shamati => As Though Standing before a King he: בעל הסולם => שמעתי => כעומד בפני מלך
nzJTZqYV en: Baal HaSulam => Shamati => Embrace of the Right, Embrace of the Left he: בעל הסולם => שמעתי => חיבוק הימין וחיבוק השמאל
QpHrSYsP en: Baal HaSulam => Shamati => Acknowledging the Desire he: בעל הסולם => שמעתי => ענין גילוי החסרון
MBvLrfSm en: Baal HaSulam => Shamati => Known in the Gates he: בעל הסולם => שמעתי => נודע בשערים
81IpTHcN en: Baal HaSulam => Shamati => Concerning Faith he: בעל הסולם => שמעתי => ענין אמונה
OqzwS7hX en: Baal HaSulam => Shamati => Right and Left he: בעל הסולם => שמעתי => ימין ושמאל
RS6B9xHs en: Baal HaSulam => Shamati => If I Am Not for Me, Who Is for Me? he: בעל הסולם => שמעתי => אם אין אני לי מי לי
O5AD7oa9 en: Baal HaSulam => Shamati => The Torah and the Creator Are One he: בעל הסולם => שמעתי => אורייתא וקוב"ה חד הוא
Kq35tLjM en: Baal HaSulam => Shamati => Devotion he: בעל הסולם => שמעתי => ענין מסירות נפש
xJQWcmc3 en: Baal HaSulam => Shamati => Suffering he: בעל הסולם => שמעתי => ענין יסורים
1Grgyqyp en: Baal HaSulam => Shamati => Multiple Authorities he: בעל הסולם => שמעתי => רשות הכל
5cZY5x2S en: Baal HaSulam => Shamati => The Part Given to the Sitra Achra to Separate It from the Kedusha he: בעל הסולם => שמעתי => ענין חלק שנותנין לס"א, כדי שיפרד מהקדושה
rI6eLc0m en: Baal HaSulam => Shamati => Clothing, Sack, Lie, Almond he: בעל הסולם => שמעתי => לבוש - שק - שקר - שקד
S8jzH3Uq en: Baal HaSulam => Shamati => Yesod de Nukva and Yesod de Dechura he: בעל הסולם => שמעתי => ענין יסוד נוקבא ויסוד דדכורא
wUb2fOWP en: Baal HaSulam => Shamati => Raising Oneself he: בעל הסולם => שמעתי => להגביה את עצמו
f3HJApYj en: Baal HaSulam => Shamati => The Written Torah and the Oral Torah – 2 he: בעל הסולם => שמעתי => תורה שבכתב ותורה שבעל פה - ב
gE6oshy2 en: Baal HaSulam => Shamati => The Reward for a Mitzva–a Mitzva he: בעל הסולם => שמעתי => שכר מצוה, מצוה
ZOtU3iua en: Baal HaSulam => Shamati => Fish before Meat he: בעל הסולם => שמעתי => דגים קודמים לבשר
4NuvpBwy en: Baal HaSulam => Shamati => Haman Pockets he: בעל הסולם => שמעתי => כיסי המן
bLiRLMFR en: Baal HaSulam => Shamati => The Lord Is High and the Low Will See - 2 he: בעל הסולם => שמעתי => רם ה' ושפל יראה - ב
EjIpuTVg en: Baal HaSulam => Shamati => The Purity of the Vessels of Reception he: בעל הסולם => שמעתי => טהרת הכלי קבלה
oYO5BR38 en: Baal HaSulam => Shamati => Completing the Labor he: בעל הסולם => שמעתי => השלמת היגיעה
LQpAHZAG en: Baal HaSulam => Shamati => Pardon, Forgiveness, and Atonement he: בעל הסולם => שמעתי => ענין מחילה סליחה וכפרה
bQKmarXE en: Baal HaSulam => Shamati => He Who Ceases Words of Torah and Engages in Conversation he: בעל הסולם => שמעתי => הפוסק מדברי תורה ועוסק בדברי שיחה
lzIeKies en: Baal HaSulam => Shamati => Looking in the Book Again he: בעל הסולם => שמעתי => מסתכל בספר מחדש
1Fx1eyuW en: Baal HaSulam => Shamati => My Adversaries Curse Me All the Day he: בעל הסולם => שמעתי => כי חרפוני צוררי כל היום
eoGGCnLJ en: Baal HaSulam => Shamati => For Man Shall Not See Me and Live he: בעל הסולם => שמעתי => כי לא יראני האדם וחי
wVDLjw8Z en: Baal HaSulam => Shamati => Happy Is the Man Who Does Not Forget You and the Son of Man Who Exerts in You he: בעל הסולם => שמעתי => אשרי איש שלא ישכחך ובן אדם יתאמץ בך
sn1W3PTR en: Baal HaSulam => Shamati => The Difference between Mochin of Shavuot and that of Shabbat at Minchah he: בעל הסולם => שמעתי => החילוק בין מוחין דשבועות לדשבת במנחה
ISMcdEeb en: Baal HaSulam => Shamati => Seek Your Seekers when They Seek Your Face he: בעל הסולם => שמעתי => דרוש נא דורשיך בדרשם פניך
FaZdHvzA en: Baal HaSulam => Shamati => Call Upon Him When He Is Near he: בעל הסולם => שמעתי => קראוהו בהיותו קרוב
72pUdWd6 en: Baal HaSulam => Shamati => What Is the Matter of Delighting the Poor on a Good Day, in the Work? he: בעל הסולם => שמעתי => מהו הענין לשמח העניים ביום טוב, בעבודה
BUrnqFTr en: Baal HaSulam => Shamati => Examining the Shade on the Night of Hosha’ana Rabbah he: בעל הסולם => שמעתי => ענין בדיקת הצל בליל הושענא רבה
aNhGvjiz en: Baal HaSulam => Shamati => All the Worlds he: בעל הסולם => שמעתי => כל העולמות
xQRH1fmu en: Baal HaSulam => Shamati => Prior to the Creation of the Newborn he: בעל הסולם => שמעתי => קודם יצירת הולד
GuPSvLG2 en: Baal HaSulam => Shamati => An Explanation about Luck he: בעל הסולם => שמעתי => ביאור על מזלא
0mkq7ck1 en: Baal HaSulam => Shamati => A Thought Is Regarded as Nourishment he: בעל הסולם => שמעתי => מחשבה היא בחינת מזונות
O3K6kGhW en: Baal HaSulam => Shamati => Let His Friend Begin he: בעל הסולם => שמעתי => שחברו יתחיל
t1K8lcXU en: Baal HaSulam => Passover Haggadah he: בעל הסולם => הגדה של פסח
Nw0ew8p4 en: Baal HaSulam => Gatehouse of Intentions he: בעל הסולם => בית שער הכוונות
KdNxQuSo en: Baal HaSulam => Even Sapir he: בעל הסולם => אבן ספיר
Y33XXcRQ en: Baal HaSulam => Ha-Ilan (The Tree) he: בעל הסולם => האילן
B7htYIL6 en: Baal HaSulam => The Bright Light he: בעל הסולם => אור הבהיר
SqBA6XOl en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות
mpwf90JE en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א'
FstrMHye en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 1 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף א'
9LQGloNw en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 2 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ב'
KeEWtHeL en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 3 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ג'
st8w3WzP en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 4 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ד'
w7GZ2F2W en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 5 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ה'
qqmJ7Ymi en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 6 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ו'
r5ILu6Hm en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 7 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ז'
0pOqdUf7 en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 8 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ח'
CQI9htD4 en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 9 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ט'
rNb47q8s en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 10 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף י'
WiMwqvYn en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 11 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף י"א
nxmbeW8U en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 12 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף י"ב
Xw9zHaRI en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 13 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף י"ג
X8KeZb3z en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 14 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף י"ד
2YY6jOgl en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 15 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ט"ו
TfgARIe0 en: Baal HaSulam => Tree of Life , with the Panim Meirot uMasbirot Commentary => Vol. 1 => Branch 16 he: בעל הסולם => עץ חיים עם פירוש פנים מאירות ומסבירות => כרך א' => ענף ט"ז
F7qDRCMu en: Baal HaSulam => Poems of Baal HaSulam he: בעל הסולם => שירי בעל הסולם
qaNEHQvj en: Baal HaSulam => Poems of Baal HaSulam => A Poem of Sanctity he: בעל הסולם => שירי בעל הסולם => שיר קודש
LcvoBxcv en: Baal HaSulam => Poems of Baal HaSulam => A Psalm – His Foundation Is in the Mountains of Holiness he: בעל הסולם => שירי בעל הסולם => שיר, יסודתו בהררי קודש
1aEgHGx5 en: Baal HaSulam => Poems of Baal HaSulam => I Am Content he: בעל הסולם => שירי בעל הסולם => דייני
QOdUhUWS en: Baal HaSulam => Poems of Baal HaSulam => The Bright One he: בעל הסולם => שירי בעל הסולם => הבהיר
TZywesWn en: Baal HaSulam => A Sage’s Fruit - Talks (heb) he: בעל הסולם => פרי חכם - שיחות
WWh3NuPD en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => An Essay Instead of Introduction he: בעל הסולם => פרי חכם - שיחות => מאמר במקום הקדמה
nwWMlOcH en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Mind of the Operator he: בעל הסולם => פרי חכם - שיחות => שכל הפועל
jhcaDDP0 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Real Adhesion Is Knowing he: בעל הסולם => פרי חכם - שיחות => הדבקות האמיתית היא בחינת ידיעה
FvG09Vxk en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Their Rock Is Not as Our Rock he: בעל הסולם => פרי חכם - שיחות => לא כצורנו צורם
Yg5AXZzz en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => EKYEH Has Sent Me to You he: בעל הסולם => פרי חכם - שיחות => אהי' שלחני אליכם
zQ7I1HwL en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Corporal Movement and Spiritual Movement he: בעל הסולם => פרי חכם - שיחות => תנועה גשמית ותנועה רוחנית
zz0AL5Z4 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => General Correction and Individual Correction he: בעל הסולם => פרי חכם - שיחות => תיקון כלל ופרט
cElDtebi en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Sin of Adam haRishon – the Sin of King Saul he: בעל הסולם => פרי חכם - שיחות => חטא אדה"ר - חטא שאול המלך
kz8FOhIv en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => A Righteous One Rules by the Fear of God he: בעל הסולם => פרי חכם - שיחות => צדיק מושל יראת אלקים
ezOJuAHk en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Faith, Craftsman he: בעל הסולם => פרי חכם - שיחות => אמונה אומן
iKKO3W1j en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => As the Morning Light, the Sun Will Shine he: בעל הסולם => פרי חכם - שיחות => וכאור בוקר יזרח שמש
xX7CWKxh en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Inversion of the Forms he: בעל הסולם => פרי חכם - שיחות => התהפכות הצורות
kzhM2p2p en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Pleasantness of Songs and Melodies he: בעל הסולם => פרי חכם - שיחות => נעימות השירים והנגינה
HNGN2av8 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concealments he: בעל הסולם => פרי חכם - שיחות => סיתומים
VG5Te86l en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => That which Is Known and That which Is Learned he: בעל הסולם => פרי חכם - שיחות => המפורסם והמושכל
d8SrLoVz en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Future Consolation he: בעל הסולם => פרי חכם - שיחות => נחמה העתידה
sKUzZ5eK en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Rearing the Sons he: בעל הסולם => פרי חכם - שיחות => חינוך הבנים
PBD1sWNd en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Corporeal Food and Spiritual Food he: בעל הסולם => פרי חכם - שיחות => מזון גשמי ומזון רוחני
eYECwzyg en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => A Commandment that Protects he: בעל הסולם => פרי חכם - שיחות => סוד מצוה דמגני
eurucFwu en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Difference between This World and the World After the Revival he: בעל הסולם => פרי חכם - שיחות => חילוק מעוה"ז לעולם שלאחר התחיה
KwKV47r7 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Envy he: בעל הסולם => פרי חכם - שיחות => קנאה
QRGwtSp2 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Still in Holiness he: בעל הסולם => פרי חכם - שיחות => דומם דקדושה
rr0vo6Uj en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => In Darkness Shall a Man Walk he: בעל הסולם => פרי חכם - שיחות => אך בצלם יתהלך איש
CTRcOlFD en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Only Good and Mercy he: בעל הסולם => פרי חכם - שיחות => אך טוב וחסד
ZJ0LqIRH en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Downfall of the Pride – the Inheritance of the Lowly he: בעל הסולם => פרי חכם - שיחות => מפלת הגאים - ירושת השפלים
N3b0rcUv en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Sefirot he: בעל הסולם => פרי חכם - שיחות => ספירות
6h3RnB8l en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The First Restriction - 1 he: בעל הסולם => פרי חכם - שיחות => צמצום א' - 1
uZaC5Ws5 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The First Restriction - 2 he: בעל הסולם => פרי חכם - שיחות => צמצום א' - 2
AquZTB40 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Three Sights in the Restriction he: בעל הסולם => פרי חכם - שיחות => שלש מראות בצמצום
DGfX5Hhq en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning the Two Ends he: בעל הסולם => פרי חכם - שיחות => עניין שני הקצים
U9nFfSNV en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Meaning of the Restriction he: בעל הסולם => פרי חכם - שיחות => סוד הצמצום
9gcbXQNO en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Matter of the Restriction he: בעל הסולם => פרי חכם - שיחות => דבר הצמצום
xbrVSVKJ en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => סוד הציץ he: בעל הסולם => פרי חכם - שיחות => סוד הציץ
zErqvher en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => סבת הצמצום he: בעל הסולם => פרי חכם - שיחות => סבת הצמצום
YN5nKpqo en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => חלל פנוי he: בעל הסולם => פרי חכם - שיחות => חלל פנוי
4tqkmFZJ en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => ולדבקה בו he: בעל הסולם => פרי חכם - שיחות => ולדבקה בו
4EWiQA2b en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Correction of the MANTZEPACH he: בעל הסולם => פרי חכם - שיחות => תיקון המנצפ"ך
plu7mrOp en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => סוד "ונוגה כאור תהיה" he: בעל הסולם => פרי חכם - שיחות => סוד "ונוגה כאור תהיה"
7hc0RiDP en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Seven Walls he: בעל הסולם => פרי חכם - שיחות => שבע חומות
ISFgurbl en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Order of the Correction he: בעל הסולם => פרי חכם - שיחות => סדר התיקון
F8DadVqs en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Permanent Pleasure, and the Finest Service he: בעל הסולם => פרי חכם - שיחות => תענוג הקבוע, ועבדות המעולה
DdcenwVc en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Governance he: בעל הסולם => פרי חכם - שיחות => ממשלה
BWnsOGug en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => A Threefold Cord Is Not Quickly Broken he: בעל הסולם => פרי חכם - שיחות => והחוט המשולש לא במהרה יינתק
AfBp8IP3 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Upper Ones Above and Lower Ones Below he: בעל הסולם => פרי חכם - שיחות => עליונים למעלה ותחתונים למטה
LbXG2FNC en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning the Daughter of Zur he: בעל הסולם => פרי חכם - שיחות => ענין בת צור
f8qH7GVF en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => תיקון השמאל he: בעל הסולם => פרי חכם - שיחות => תיקון השמאל
7QricenG en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Emanation of a Soul he: בעל הסולם => פרי חכם - שיחות => אצילות נשמה
nYIja16x en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Pride he: בעל הסולם => פרי חכם - שיחות => גבהות הלב
pW5NA7aU en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Great Is He Who Enjoys His Labor he: בעל הסולם => פרי חכם - שיחות => גדול הנהנה מיגיעו
w5UMjtbD en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Bestowal and Pleasure he: בעל הסולם => פרי חכם - שיחות => השפעה ותענוג
kukV3vko en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => I the Lord Do Not Change he: בעל הסולם => פרי חכם - שיחות => אני הוי' לא שניתי
lB2ioabj en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Man’s Actions and His Tactics he: בעל הסולם => פרי חכם - שיחות => פעולות האדם ותחבולותיו. ציור אחד יוצא מכל המעשים
dLZ5mLWC en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => אחדות הפעולות he: בעל הסולם => פרי חכם - שיחות => אחדות הפעולות
KnzwP431 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => הדבר שהיה בכלל he: בעל הסולם => פרי חכם - שיחות => הדבר שהיה בכלל
OFnORJG7 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => ומבשרי אחזה he: בעל הסולם => פרי חכם - שיחות => ומבשרי אחזה
ZpJvmlkB en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => התאחדות המעשים למעשה אחד he: בעל הסולם => פרי חכם - שיחות => התאחדות המעשים למעשה אחד
BG9vKcLz en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => התאחדות הנבראים he: בעל הסולם => פרי חכם - שיחות => התאחדות הנבראים
k9Zrxwhy en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning the Second Katnut he: בעל הסולם => פרי חכם - שיחות => ענין קטנות ב'
ZMpfUZ4t en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Blessed Is the Glory of the Name he: בעל הסולם => פרי חכם - שיחות => ברוך שם כבוד
Eu6UFpqj en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Observing the Torah Depends on the Land he: בעל הסולם => פרי חכם - שיחות => שמירת התורה תלויה בארץ
N4XZ8IsP en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => קב"ה וישראל חד הוא he: בעל הסולם => פרי חכם - שיחות => קב"ה וישראל חד הוא
nfBjzZPl en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => If Cedars Succumb to Fire he: בעל הסולם => פרי חכם - שיחות => אם בארזים נפלה שלהבת
JhdWYRnc en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Six-Hundred Thousand Souls he: בעל הסולם => פרי חכם - שיחות => ששים רבוא נשמות
vRNgjAIN en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The End of Correction he: בעל הסולם => פרי חכם - שיחות => גמר התיקון
pbYojKpp en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Meaning of His Names he: בעל הסולם => פרי חכם - שיחות => סוד שמותיו ית'
ZzqDO0Nd en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Pride, A Voluntary or Mandatory Will he: בעל הסולם => פרי חכם - שיחות => גאוה, רצון בחירי ומחוייב 
7okPbmXS en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Elijah and Elisha he: בעל הסולם => פרי חכם - שיחות => אליהו ואלישע
fsC9w0fK en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Who Has Preceded Me he: בעל הסולם => פרי חכם - שיחות => מי הקדמני
oaOGOcTU en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Meaning of Prophecy he: בעל הסולם => פרי חכם - שיחות => סוד הנבואה
3DaNoFGz en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Segula of the Torah he: בעל הסולם => פרי חכם - שיחות => סגולות התורה
dDYezIIt en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => What Applies to Corporeality Applies to Spirituality he: בעל הסולם => פרי חכם - שיחות => מה שנוהג בגשמיות נוהג ברוחניות
1FRkvnrV en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Segula of Sleep he: בעל הסולם => פרי חכם - שיחות => סגולת השינה
33Yj32WY en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Signet of the Creator Is Truth he: בעל הסולם => פרי חכם - שיחות => חותמו של הקב"ה אמת
K96Fc8QE en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => An Additional Merit to the Wholeness he: בעל הסולם => פרי חכם - שיחות => שבח הנוסף על סוד השלימות
i2c5yiXF en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Meaning of Old Age he: בעל הסולם => פרי חכם - שיחות => ענין זקנה
P4crMEs6 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Power of the Soul he: בעל הסולם => פרי חכם - שיחות => כח הנשמה
GVFomAms en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => זכוך הגוף he: בעל הסולם => פרי חכם - שיחות => זכוך הגוף
LSfWEfL6 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Nature’s Systems he: בעל הסולם => פרי חכם - שיחות => מערכות הטבע
ZsBS9GTl en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Meaning of a Reward he: בעל הסולם => פרי חכם - שיחות => ענין פרס
ffFZQnVC en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Corporeal Offspring and Spiritual Offspring he: בעל הסולם => פרי חכם - שיחות => תולדות גשמיים ותולדות רוחניים
NVyxXrmu en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Perception and Perceived he: בעל הסולם => פרי חכם - שיחות => שכל ומושכל
g6cwecCQ en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Know the God of Your Father and Serve Him he: בעל הסולם => פרי חכם - שיחות => דע את אלקי אביך ועבדהו
lBH8dsLm en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => עבודה באהבה he: בעל הסולם => פרי חכם - שיחות => עבודה באהבה
2D8mQ4cd en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => A Righteous Eats to Satiate His Soul he: בעל הסולם => פרי חכם - שיחות => צדיק אוכל לשובע נפשו
9y2WiLWf en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Pregnant and the Woman in Labor Together he: בעל הסולם => פרי חכם - שיחות => הרה וילדת יחדו
Az6QxP47 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Time and Place he: בעל הסולם => פרי חכם - שיחות => זמן ומקום
rwpwPu0v en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning the Spiritual Form he: בעל הסולם => פרי חכם - שיחות => ענין צורה רוחנית
cVvr7spz en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Deficiency and Wholeness he: בעל הסולם => פרי חכם - שיחות => חסרון ושלימות
utPIIbyc en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Externality and Internality - 1 he: בעל הסולם => פרי חכם - שיחות => חיצוניות ופנימיות-1
o1bsRVV2 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Son and Servant he: בעל הסולם => פרי חכם - שיחות => בן ועבד
geDstins en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Wholeness - 1 he: בעל הסולם => פרי חכם - שיחות => השלימות-1
hnQLzQua en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Learning the Holy Torah he: בעל הסולם => פרי חכם - שיחות => לימוד התורה הק'
6um2DvRW en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Anyone Who Is Rewarded with the Creator Bestowing Upon Him HBD he: בעל הסולם => פרי חכם - שיחות => כל שזכה שהקב"ה משפיע לו חב"ד
QzaxfIJw en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Lord Is High and the Low Will See he: בעל הסולם => פרי חכם - שיחות => רם הוי ושפל יראה
xYl1s7mI en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Unless the Lord Builds a House he: בעל הסולם => פרי חכם - שיחות => אם הוי' לא יבנה בית
Uge5vbmp en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Meaning of Empty Vessels he: בעל הסולם => פרי חכם - שיחות => רזא דכלים ריקים
E3Oy2JBc en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Creator Has Three Worlds he: בעל הסולם => פרי חכם - שיחות => ג' עלמין אית ליה לקב"ה
7J1xbBaO en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning Externality and Internality - 2 he: בעל הסולם => פרי חכם - שיחות => ענין חיצוניות ופנימיות - 2
Dfodoac1 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Between Pleasure and Joy he: בעל הסולם => פרי חכם - שיחות => אין בין תענוג לשמחה
vQVXmLZG en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Wisdom of Feeling the Pleasure he: בעל הסולם => פרי חכם - שיחות => חכמת הרגש התענוג
yiZpDYgC en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Devotion he: בעל הסולם => פרי חכם - שיחות => מסירות נפש
cv6KIbx1 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Shabbat Comes, Rest Comes he: בעל הסולם => פרי חכם - שיחות => באה שבת באה מנוחה
vvrbEhuy en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Merit of Shabbat he: בעל הסולם => פרי חכם - שיחות => מעלת השבת
4uBUOPtO en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning a Virtuous Woman he: בעל הסולם => פרי חכם - שיחות => ענין אשת חיל
ux9MvS5Z en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Explaining the Midrash “And God Concluded” he: בעל הסולם => פרי חכם - שיחות => ביאור המדרש ויכל אלקים
iE7iAQQF en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => A Speech for Shabbat he: בעל הסולם => פרי חכם - שיחות => מאמר השבת
PF5OIfsv en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => You Shall Observe the Shabbat he: בעל הסולם => פרי חכם - שיחות => ושמרתם את השבת
BryKqmY0 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Two Sabbaths he: בעל הסולם => פרי חכם - שיחות => ב' שבתות
pPiWYwED en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => לכולם נתת בן זוג he: בעל הסולם => פרי חכם - שיחות => לכולם נתת בן זוג
cCd6V7El en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Shabbat and Shmita he: בעל הסולם => פרי חכם - שיחות => שבת ושמיטה
zuAzsmBC en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Value of Movement – and Rest he: בעל הסולם => פרי חכם - שיחות => ערך התנועה  - והמנוחה
5XX8740w en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => And the Children of Israel Shall Observe the Shabbat he: בעל הסולם => פרי חכם - שיחות => ושמרו בני ישראל את השבת
dYUpQED0 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Wholeness - 2 he: בעל הסולם => פרי חכם - שיחות => השלימות - 2
Aw7Mmw58 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Great Wonders he: בעל הסולם => פרי חכם - שיחות => פלאי פלאות ג' בחינות
oepkPtvt en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Reality of the Work he: בעל הסולם => פרי חכם - שיחות => מציאות העבודה
77AjzRBQ en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Anything that Is Real Has Truth he: בעל הסולם => פרי חכם - שיחות => כל שיש לו מציאות יש לו אמת
c84p3zen en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Concerning the Housewife he: בעל הסולם => פרי חכם - שיחות => ענין עקרת בית הנפש הוא עקרת הבית
TYiO3fDt en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => הבונה בית למלכו he: בעל הסולם => פרי חכם - שיחות => הבונה בית למלכו
3KWW0rKL en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Yesod of Ima El Shadai he: בעל הסולם => פרי חכם - שיחות => יסוד אמא א-ל שד"י
0nFMi2IA en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => מצה למצוה he: בעל הסולם => פרי חכם - שיחות => מצה למצוה
BXdaTQv5 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => ה' ספירות he: בעל הסולם => פרי חכם - שיחות => ה' ספירות
t0HyvkYj en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => הויות he: בעל הסולם => פרי חכם - שיחות => הויות
vEg9j0CB en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => אדם הראשון נולד מהול he: בעל הסולם => פרי חכם - שיחות => אדם הראשון נולד מהול
S0d6ClcA en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Rainbow he: בעל הסולם => פרי חכם - שיחות => קשת
J1Oc99H8 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => מילה - פריעה - אטיפא דמא he: בעל הסולם => פרי חכם - שיחות => מילה - פריעה - אטיפא דמא
O5xP6JD0 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Acceptance of the Prayer he: בעל הסולם => פרי חכם - שיחות => קבלת התפילה
ztBZthME en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Changing Place and Changing the Times he: בעל הסולם => פרי חכם - שיחות => שינוי מקום ושינוי העתים
9O2I0aS3 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Lord, Open My Lips he: בעל הסולם => פרי חכם - שיחות => הוי' שפתי תפתח
zzrbK72e en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => With Fear and Love he: בעל הסולם => פרי חכם - שיחות => בדחילו ורחימו
hSn08q3g en: Baal HaSulam => A Sage’s Fruit - Talks (heb) =>  Delving in Prayer  he: בעל הסולם => פרי חכם - שיחות => עיון תפילה
83eKdJsw en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Correction of the Disgrace, “From Where Did You Come?” he: בעל הסולם => פרי חכם - שיחות => תקון  הבושה "מאין באת"
GB6RWnh4 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Mercy he: בעל הסולם => פרי חכם - שיחות => רחמים
O4R8TF2s en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => A Psalm of Asaph he: בעל הסולם => פרי חכם - שיחות => מזמור לאסף
8wjGbUje en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => There Are Four that the Mind Cannot Tolerate he: בעל הסולם => פרי חכם - שיחות => ארבעה אין הדעת סובלתן
0mDWHrPF en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Creator Wants the Heart he: בעל הסולם => פרי חכם - שיחות => רחמנא ליבא בעי
F9ODSHol en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Blemishing the Eyes he: בעל הסולם => פרי חכם - שיחות => פגימת עינים
jGwSAE2D en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Two Fears he: בעל הסולם => פרי חכם - שיחות => ב' יראות
kd670fx4 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => These Cry and Those Cry he: בעל הסולם => פרי חכם - שיחות => הללו בוכין והללו בוכין
6lpvonhf en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Anyone Who Is Sorry for the Public he: בעל הסולם => פרי חכם - שיחות => כל המצטער עם הצבור
iLG3ETIq en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Argument between the Assembly of Israel and the Creator he: בעל הסולם => פרי חכם - שיחות => הויכוח שבין כ"י להשי"ת
t1rBekj6 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Everything Is in the Hands of Heaven he: בעל הסולם => פרי חכם - שיחות => הכל בידי שמים
3pewLp85 en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => תיקון הנשמות he: בעל הסולם => פרי חכם - שיחות => תיקון הנשמות
65j2ZKfx en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => Revealing the Uniqueness he: בעל הסולם => פרי חכם - שיחות => גילוי היחוד
HUZ6vxtp en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Merit of Returning he: בעל הסולם => פרי חכם - שיחות => היתרון שבהשבה
cXLQa4ue en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Two Ways by which the World Behaves he: בעל הסולם => פרי חכם - שיחות => שני הדרכים שבהם העולם מתנהג
7qT6nilJ en: Baal HaSulam => A Sage’s Fruit - Talks (heb) => The Revelation of the Face he: בעל הסולם => פרי חכם - שיחות => גילוי פנים
L7RTwFbP en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) he: בעל הסולם => פרי חכם - על התורה
7KJ10upq en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Introduction he: בעל הסולם => פרי חכם - על התורה => הקדמה
gNFySbbH en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Creation of the World with the Quality of Mercy he: בעל הסולם => פרי חכם - על התורה => בריאת העולם במדת הרחמים
DY5tURP1 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Tohu and Bohu [Formlessness and Void] - 1 he: בעל הסולם => פרי חכם - על התורה => צדיא וריקניא - 1
fVAALy4e en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Work of Creation he: בעל הסולם => פרי חכם - על התורה => מעשה בראשית
gOF2lhAu en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Tohu and Bohu [Formlessness and Void] - 2 he: בעל הסולם => פרי חכם - על התורה => צדיא וריקניא - 2
gFk2abSH en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Let Us Make Man he: בעל הסולם => פרי חכם - על התורה => נעשה אדם
J2NJC2IT en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Construction of Man and His Creation he: בעל הסולם => פרי חכם - על התורה => בנין האדם ויצירתו
iYVlpwSk en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Generations of the Heaven and the Earth he: בעל הסולם => פרי חכם - על התורה => תולדות השמים  והארץ
3NFrC9Zu en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Even the king is served from the field he: בעל הסולם => פרי חכם - על התורה => מלך לשדה נעבד
RtqAElS8 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Creation of the First Man he: בעל הסולם => פרי חכם - על התורה => בריאת אדם הראשון
Prf5GZFz en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Sin of the Tree of Knowledge he: בעל הסולם => פרי חכם - על התורה => חטא עץ הדעת
woStdazO en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => And Noah, a Man of the Soil, Began he: בעל הסולם => פרי חכם - על התורה => ויחל נח - ויטע כרם
zwtc9GeA en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Sin of the Generation of the Flood he: בעל הסולם => פרי חכם - על התורה => ענין חטא דור המבול
2fFnVnzI en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Demise of the Holy Ones he: בעל הסולם => פרי חכם - על התורה => עניין הסתלקות הקדושים
rmgD2SyH en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => If They Merit, I Will Hasten It; If They Do Not Merit, In Due Time he: בעל הסולם => פרי חכם - על התורה => זכו אחישנה לא זכו בעתה
kAFmQ7Ob en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Vayera he: בעל הסולם => פרי חכם - על התורה => וירא
rJ9oz6l2 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => A True Matter he: בעל הסולם => פרי חכם - על התורה => דבר אמת
UXMG4W3c en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => I Will Draw for Your Camels Also he: בעל הסולם => פרי חכם - על התורה => וגם לגמליך אשאב
Qr7A34Bc en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Toldot [Generations] he: בעל הסולם => פרי חכם - על התורה => תולדות
dISr5hWJ en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Sun and Moon – Esau and Jacob he: בעל הסולם => פרי חכם - על התורה => שמש ולבנה - עשו ויעקב
AQ8mlxNB en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Nimrod Started and Esau Finished he: בעל הסולם => פרי חכם - על התורה => נמרוד החל ועשו כלה
1gihuOB3 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Isaac Loved Esau he: בעל הסולם => פרי חכם - על התורה => ויאהב יצחק את עשו
u0KSYa6y en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => VaYetze he: בעל הסולם => פרי חכם - על התורה => ויצא
Mup3JrZS en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Not the Time for the Livestock to Be Gathered he: בעל הסולם => פרי חכם - על התורה => לא עת האסף המקנה
GRnc5CYt en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Name of That Place Mahanaim he: בעל הסולם => פרי חכם - על התורה => שם המקום מחנים
dabIfRfP en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => VaYishlach [Jacob Sent] he: בעל הסולם => פרי חכם - על התורה => וישלח
bB0tmMtW en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Jacob Prepared Himself for a Gift, for Prayer, and for War he: בעל הסולם => פרי חכם - על התורה => יעקב שהתקין עצמו לדורון לתפילה ולמלחמה
f5ly4em7 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Matter of Jacob Wanting to Reveal the End he: בעל הסולם => פרי חכם - על התורה => ענין בקש יעקב לגלות את הקץ
qBxUVeSN en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Moses’ Basket he: בעל הסולם => פרי חכם - על התורה => תבת משה
C9k1g9Jd en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Meaning of Shovavim TaT he: בעל הסולם => פרי חכם - על התורה => סוד שובבי"ם ת"ת
JbCqydHI en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Moses Was Herding the Flock of Jethro he: בעל הסולם => פרי חכם - על התורה => ומשה היה רועה את צאן יתרו
ERyPzvW4 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Moses’ Questions in the Sight of the Bush he: בעל הסולם => פרי חכם - על התורה => שאלות משה במראה הסנה
l2eeTqPQ en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Rolling Land and Fixed Signs he: בעל הסולם => פרי חכם - על התורה => ארץ מתגלגלת,  ומזלות קבועים
dqzTEAcc en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => EKYEH Sent Me to You he: בעל הסולם => פרי חכם - על התורה => אהי' שלחני אליכם
kgAm6Vhd en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Three Tokens he: בעל הסולם => פרי חכם - על התורה => שלשת האותות
qQ1fQ3iV en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Corporeality and Spirituality he: בעל הסולם => פרי חכם - על התורה => גשמיות ורוחניות
JkcnBwPE en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Bread from Heaven he: בעל הסולם => פרי חכם - על התורה => לחם מן השמים
XmxTx5tw en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => And Moses’ Hands Were Heavy he: בעל הסולם => פרי חכם - על התורה => וידי משה כבדים
2UWol1hR en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => And They Journeyed from Rephidim he: בעל הסולם => פרי חכם - על התורה => ויסעו מרפידים
vFIBuFKN en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Commandment To-Do – I Am the Lord Your God he: בעל הסולם => פרי חכם - על התורה => מצות עשה, אנכי הוי' אלקיך
AHTL1THQ en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Go Out Free for Nothing he: בעל הסולם => פרי חכם - על התורה => לחפשי חנם
m0o2Ss40 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Shechina in the Lower Ones he: בעל הסולם => פרי חכם - על התורה => שכינתא בתחתונים
XLnPUS3p en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => On the Seventh Day He Stopped and Rested he: בעל הסולם => פרי חכם - על התורה => ביום השביעי שבת וינפש
rY09FGSm en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => And The Children of Israel Shall Observe the Sabbath he: בעל הסולם => פרי חכם - על התורה => ושמרו בני ישראל את השבת
MNgbbjAU en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The King’s Honor he: בעל הסולם => פרי חכם - על התורה => כבוד המלך
Xi8VwVtv en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Light of the Menorah [Lampstand] he: בעל הסולם => פרי חכם - על התורה => אור המנורה
ybkMtrVW en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Eldad and Meidad he: בעל הסולם => פרי חכם - על התורה => אלדד ומידד
xiobTVvT en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Man Moses Was Very Humble he: בעל הסולם => פרי חכם - על התורה => האיש משה ענו
sKxImRji en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Correction of the Spies he: בעל הסולם => פרי חכם - על התורה => תיקון המרגלים
Rf7y2U1A en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Concerning the Tzitzit he: בעל הסולם => פרי חכם - על התורה => ענין ציצית
eAqKGPHg en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Meaning of Budding, Blossoming, and Almonds he: בעל הסולם => פרי חכם - על התורה => סוד ציץ ופרח ושקדים
lC4yROBE en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Korah he: בעל הסולם => פרי חכם - על התורה => קרח
Bzn1LwtT en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Korah and His Followers he: בעל הסולם => פרי חכם - על התורה => קרח ועדתו
FXSAzBJt en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Day of the First Fruit he: בעל הסולם => פרי חכם - על התורה => יום הביכורים
YRd90ENk en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Day Follows the Night he: בעל הסולם => פרי חכם - על התורה => היום נמשך אחר הלילה
NG2ADHB0 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => First Born and Simple – the Night after the Day he: בעל הסולם => פרי חכם - על התורה => בכור ופשוט - הלילה אחר היום
fYpDOIx8 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Purpose of Knowing he: בעל הסולם => פרי חכם - על התורה => תכלית הידיעה
NaokmSJM en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Fear the Lord Your God he: בעל הסולם => פרי חכם - על התורה => את ה' אלקיך תירא
20piHTKG en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => Concerning Meat of Lust he: בעל הסולם => פרי חכם - על התורה => ענין בשר תאוה
S3k2Olsn en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => You Are the Sons of the Lord Your God he: בעל הסולם => פרי חכם - על התורה => בנים אתם להוי' אלקיכם
Wo57gy75 en: Baal HaSulam => A Sage’s Fruit - about the Torah (heb) => The Coupling of the Creator and His Shechina he: בעל הסולם => פרי חכם - על התורה => סוד זווג קוב"ה ושכינתיה
rb en: Rabash he: רב"ש
SJDw9tHs en: Rabash => Prefaces he: רב"ש => הקדמות
KEOQdQoK en: Rabash => Prefaces => Introduction to articles he: רב"ש => הקדמות => הקדמה למאמרים
k6rrW2Ht en: Rabash => Prefaces => Introduction to the book Pri Haham. Letters he: רב"ש => הקדמות => הקדמה לאגרות
b8SHlrfH en: Rabash => Letters he: רב"ש => אגרות
VUcyhrcS en: Rabash => Letters => Letter 1 he: רב"ש => אגרות => אגרת א
y3y2zfjA en: Rabash => Letters => Letter 2 he: רב"ש => אגרות => אגרת ב
Wn5BUr9y en: Rabash => Letters => Letter 3 he: רב"ש => אגרות => אגרת ג
LmVzTAog en: Rabash => Letters => Letter 4 he: רב"ש => אגרות => אגרת ד
1R5y4TZQ en: Rabash => Letters => Letter 5 he: רב"ש => אגרות => אגרת ה
evQdKKjA en: Rabash => Letters => Letter 6 he: רב"ש => אגרות => אגרת ו
joF8hcu8 en: Rabash => Letters => Letter 7 he: רב"ש => אגרות => אגרת ז
xu2KGyCi en: Rabash => Letters => Letter 8 he: רב"ש => אגרות => אגרת ח
q0zshf9p en: Rabash => Letters => Letter 9 he: רב"ש => אגרות => אגרת ט
K2wD6Iau en: Rabash => Letters => Letter 10 he: רב"ש => אגרות => אגרת י
hIxgB0uN en: Rabash => Letters => Letter 11 he: רב"ש => אגרות => אגרת יא
KTlMyzVq en: Rabash => Letters => Letter 12-1 he: רב"ש => אגרות => אגרת יב/א
ttEQUqs2 en: Rabash => Letters => Letter 12-2 he: רב"ש => אגרות => אגרת יב/ב
7SlpRtOc en: Rabash => Letters => Letter 13 he: רב"ש => אגרות => אגרת יג
ak24YYLg en: Rabash => Letters => Letter 14 he: רב"ש => אגרות => אגרת יד
lLR38a5Q en: Rabash => Letters => Letter 15 he: רב"ש => אגרות => אגרת טו
0UXdbMmk en: Rabash => Letters => Letter 16 he: רב"ש => אגרות => אגרת טז
lxim7Qb9 en: Rabash => Letters => Letter 17 he: רב"ש => אגרות => אגרת יז
O6yqyPhj en: Rabash => Letters => Letter 18 he: רב"ש => אגרות => אגרת יח
gR1KcKCm en: Rabash => Letters => Letter 19 he: רב"ש => אגרות => אגרת יט
LNe2MYRQ en: Rabash => Letters => Letter 20 he: רב"ש => אגרות => אגרת כ
LpOSTRQu en: Rabash => Letters => Letter 21 he: רב"ש => אגרות => אגרת כא
ngp8zskc en: Rabash => Letters => Letter 22 he: רב"ש => אגרות => אגרת כב
c6c40g2x en: Rabash => Letters => Letter 23 he: רב"ש => אגרות => אגרת כג
9EnDrL4d en: Rabash => Letters => Letter 24 he: רב"ש => אגרות => אגרת כד
6y8qHNDJ en: Rabash => Letters => Letter 25 he: רב"ש => אגרות => אגרת כה
Zm8XzyCb en: Rabash => Letters => Letter 26 he: רב"ש => אגרות => אגרת כו
I0MzYhSM en: Rabash => Letters => Letter 27 he: רב"ש => אגרות => אגרת כז
tH9vXh0A en: Rabash => Letters => Letter 28 he: רב"ש => אגרות => אגרת כח
NyGRNpfE en: Rabash => Letters => Letter 29 he: רב"ש => אגרות => אגרת כט
AMrvZedZ en: Rabash => Letters => Letter 30 he: רב"ש => אגרות => אגרת ל
v3p2aLSP en: Rabash => Letters => Letter 31 he: רב"ש => אגרות => אגרת לא
PMGGxvtJ en: Rabash => Letters => Letter 32 he: רב"ש => אגרות => אגרת לב
iU9IutbW en: Rabash => Letters => Letter 33 he: רב"ש => אגרות => אגרת לג
UgpkiWIP en: Rabash => Letters => Letter 34 he: רב"ש => אגרות => אגרת לד
OpOSSO5S en: Rabash => Letters => Letter 35 he: רב"ש => אגרות => אגרת לה
ONF8ueuk en: Rabash => Letters => Letter 36 he: רב"ש => אגרות => אגרת לו
Yj3LOGl4 en: Rabash => Letters => Letter 37 he: רב"ש => אגרות => אגרת לז
wy8Onnwz en: Rabash => Letters => Letter 38-1 he: רב"ש => אגרות => אגרת לח/א
gA65hqWi en: Rabash => Letters => Letter 38-2 he: רב"ש => אגרות => אגרת לח/ב
rSBoEkQ9 en: Rabash => Letters => Letter 39 he: רב"ש => אגרות => אגרת לט
mVQJIMlj en: Rabash => Letters => Letter 40 he: רב"ש => אגרות => אגרת מ
0lqzOMwM en: Rabash => Letters => Letter 41 he: רב"ש => אגרות => אגרת מא
lbuuR7FL en: Rabash => Letters => Letter 42 he: רב"ש => אגרות => אגרת מב
LaCQRJCP en: Rabash => Letters => Letter 43 he: רב"ש => אגרות => אגרת מג
mpDmZBtM en: Rabash => Letters => Letter 44 he: רב"ש => אגרות => אגרת מד
XIz15iFn en: Rabash => Letters => Letter 45 he: רב"ש => אגרות => אגרת מה
3WEeLKc5 en: Rabash => Letters => Letter 46 he: רב"ש => אגרות => אגרת מו
K7oB6P8s en: Rabash => Letters => Letter 47 he: רב"ש => אגרות => אגרת מז
fsamVSrs en: Rabash => Letters => Letter 48 he: רב"ש => אגרות => אגרת מח
yZM1RVqq en: Rabash => Letters => Letter 49 he: רב"ש => אגרות => אגרת מט
cT6IAU0f en: Rabash => Letters => Letter 50 he: רב"ש => אגרות => אגרת נ
EIjQxUF7 en: Rabash => Letters => Letter 51 he: רב"ש => אגרות => אגרת נא
YyBVTuJx en: Rabash => Letters => Letter 52 he: רב"ש => אגרות => אגרת נב
X48ut8mT en: Rabash => Letters => Letter 53 he: רב"ש => אגרות => אגרת נג
pT4ptnxl en: Rabash => Letters => Letter 54 he: רב"ש => אגרות => אגרת נד
EVtHRlzz en: Rabash => Letters => Letter 55 he: רב"ש => אגרות => אגרת נה
1O1EMReC en: Rabash => Letters => Letter 56 he: רב"ש => אגרות => אגרת נו
bo5TsQh3 en: Rabash => Letters => Letter 57 he: רב"ש => אגרות => אגרת נז
RvWUxQeo en: Rabash => Letters => Letter 58 he: רב"ש => אגרות => אגרת נח
TiGBd0YK en: Rabash => Letters => Letter 59 he: רב"ש => אגרות => אגרת נט
p7JgDhrS en: Rabash => Letters => Letter 60 he: רב"ש => אגרות => אגרת ס
r3U7NLLb en: Rabash => Letters => Letter 61 he: רב"ש => אגרות => אגרת סא
YkCg0RF5 en: Rabash => Letters => Letter 62 he: רב"ש => אגרות => אגרת סב
7XDwRi1f en: Rabash => Letters => Letter 63 he: רב"ש => אגרות => אגרת סג
b1zQfKle en: Rabash => Letters => Letter 64 he: רב"ש => אגרות => אגרת סד
AioP1ukM en: Rabash => Letters => Letter 65 he: רב"ש => אגרות => אגרת סה
V7H2ld8J en: Rabash => Letters => Letter 66 he: רב"ש => אגרות => אגרת סו
DvjrPmWd en: Rabash => Letters => Letter 67 he: רב"ש => אגרות => אגרת סז
PBrkeif3 en: Rabash => Letters => Letter 68 he: רב"ש => אגרות => אגרת סח
I4vGUx8T en: Rabash => Letters => Letter 69 he: רב"ש => אגרות => אגרת סט
5gviexV4 en: Rabash => Letters => Letter 70 he: רב"ש => אגרות => אגרת ע
BXU65SE6 en: Rabash => Letters => Letter 71 he: רב"ש => אגרות => אגרת עא
2VeaMtY7 en: Rabash => Letters => Letter 72 he: רב"ש => אגרות => אגרת עב
cJ8ohcBP en: Rabash => Letters => Letter 73 he: רב"ש => אגרות => אגרת עג
55qZ0Oh5 en: Rabash => Letters => Letter 74 he: רב"ש => אגרות => אגרת עד
31JyPQsS en: Rabash => Letters => Letter 75 he: רב"ש => אגרות => אגרת עה
FJ4UyZxq en: Rabash => Letters => Letter 76 he: רב"ש => אגרות => אגרת עו
WHAm5P7e en: Rabash => Letters => Letter 77 he: רב"ש => אגרות => אגרת עז
26PlNbIk en: Rabash => Letters => Letter 78 he: רב"ש => אגרות => אגרת עח
rQ6sIUZK en: Rabash => Articles he: רב"ש => מאמרים
IHYcOU8k en: Rabash => Articles => Purpose of Society - 1 he: רב"ש => מאמרים => מטרת החברה - א
he3tEpLu en: Rabash => Articles => Purpose of Society - 2 he: רב"ש => מאמרים => מטרת החברה - ב
M53FJnYF en: Rabash => Articles => Concerning Love of Friends he: רב"ש => מאמרים => בענין אהבת חברים
jXgT6Pa1 en: Rabash => Articles => Love of Friends - 1 he: רב"ש => מאמרים => אהבת חברים - א
gzm3fAe8 en: Rabash => Articles => Each One Shall Help His Friend he: רב"ש => מאמרים => איש את רעהו יעזורו
L1OKGSxg en: Rabash => Articles => What Does the Rule "Love Thy Friend as Thyself" Give Us he: רב"ש => מאמרים => מה נותן לנו הכלל של ואהבת לרעך
O9a0iCL0 en: Rabash => Articles => Love of Friends - 2 he: רב"ש => מאמרים => אהבת חברים - ב
QpKRILZk en: Rabash => Articles => According to What Is Explained Concerning “Love Thy Friend as Thyself” he: רב"ש => מאמרים => לפי מה שמבואר בענין ואהבת לרעך
zz7MH7nx en: Rabash => Articles => Which Keeping of Torah and Mitzvot Purifies the Heart he: רב"ש => מאמרים => איזה קיום תורה ומצות מזכך את הלב
tuNiurqI en: Rabash => Articles => One Should Always Sell the Beams of His House he: רב"ש => מאמרים => לעולם ימכור אדם קורות ביתו
8HGXFFx3 en: Rabash => Articles => Achieve in Order Not to Have to Reincarnate? he: רב"ש => מאמרים => לאיזה דרגה האדם צריך להגיע שלא יצטרך להתגלגל
mYp4uD1t en: Rabash => Articles => Concerning Ancestral Merit he: רב"ש => מאמרים => ענין זכות אבות
r0GUBxQQ en: Rabash => Articles => Concerning the Importance of Society he: רב"ש => מאמרים => ענין חשיבות החברה
ify4ela3 en: Rabash => Articles => Sometimes Spirituality Is Called “a Soul” he: רב"ש => מאמרים => לפעמים מכנים את הרוחניות בשם "נשמה"
qfILR5s0 en: Rabash => Articles => Forevermore One Sells All That Is His and Marries a Wise Disciple's Daughter he: רב"ש => מאמרים => לעולם ימכור אדם כל מה שיש לו וישא בת ת"ח
wtWxVvqk en: Rabash => Articles => Can Something Negative Come Down from Above he: רב"ש => מאמרים => היתכן שירד משמים דבר שלילי
qB3kWFbS en: Rabash => Articles => Concerning Bestowal he: רב"ש => מאמרים => ענין השפעה
9nU3N2k2 en: Rabash => Articles => Concerning the Importance of Friends he: רב"ש => מאמרים => בענין חשיבות החברים
aeUpX57j en: Rabash => Articles => The Agenda of the Assembly - 1 he: רב"ש => מאמרים => סדר ישיבת החברה
HMJEoWye en: Rabash => Articles => And It Shall Come to Pass When You Come to the Land that the Lord Your God Gives You he: רב"ש => מאמרים => והיה כי תבוא אל הארץ אשר ה' אלקיך נתן לך
8SncGPLY en: Rabash => Articles => You Stand Today, All of You he: רב"ש => מאמרים => אתם נצבים היום כולכם
UAiHPiou en: Rabash => Articles => Make for Yourself a Rav and Buy Yourself a Friend - 1 he: רב"ש => מאמרים => עשה לך רב וקנה לך חבר - א
2r0erVTw en: Rabash => Articles => The Meaning of Branch and Root he: רב"ש => מאמרים => ענין ענף ושורש
BIpjOLWK en: Rabash => Articles => The Meaning of Truth and Faith he: רב"ש => מאמרים => ענין אמת ואמונה
hzV8xj1m en: Rabash => Articles => These Are the Generations of Noah he: רב"ש => מאמרים => אלה תולדות נח
GTjHa8FZ en: Rabash => Articles => Go Forth From Your Land he: רב"ש => מאמרים => לך לך מארצך
0fNfKt8E en: Rabash => Articles => And the Lord Appeared to Him at the Oaks of Mamre he: רב"ש => מאמרים => וירא אליו ה' באלוני ממרא
JIdvHBJJ en: Rabash => Articles => The Life of Sarah he: רב"ש => מאמרים => חיי שרה
y5BKi0y2 en: Rabash => Articles => Make for Yourself a Rav and Buy Yourself a Friend - 2 he: רב"ש => מאמרים => עשה לך רב וקנה לך חבר - ב
chcRilv6 en: Rabash => Articles => Jacob Went Out he: רב"ש => מאמרים => ויתרוצצו הבנים בקרבה
8kqLytH9 en: Rabash => Articles => And Jacob Went Out he: רב"ש => מאמרים => ויצא יעקב
2rDSqq0W en: Rabash => Articles => Concerning the Debate between Jacob and Laban he: רב"ש => מאמרים => בענין הוויכוח בין יעקב ללבן
lUHtRZ42 en: Rabash => Articles => Jacob Dwelled in the Land Where His Father Had Lived he: רב"ש => מאמרים => וישב יעקב בארץ מגורי אביו
HotkF4i5 en: Rabash => Articles => Mighty Rock of My Salvation he: רב"ש => מאמרים => מעוז צור ישועתי
8DD4tB3o en: Rabash => Articles => I Am the First and I Am the Last he: רב"ש => מאמרים => אני ראשון ואני אחרון
0JtxBjoY en: Rabash => Articles => And Hezekiah Turned His Face to the Wall he: רב"ש => מאמרים => ויסב חזקיהו פניו אל הקיר
fqCzgTvP en: Rabash => Articles => But the More They Afflicted Them he: רב"ש => מאמרים => וכאשר יענו אותו
OzlVussk en: Rabash => Articles => Know Today and Reply to Your Heart he: רב"ש => מאמרים => וידעת היום והשבות אל לבבך
2BR2ZJaN en: Rabash => Articles => Concerning the Slanderers he: רב"ש => מאמרים => ענין המשטינים
6dFuj7yI en: Rabash => Articles => Come unto Pharaoh - 1 he: רב"ש => מאמרים => בא אל פרעה - א
whmXpBfw en: Rabash => Articles => He who Hardens His Heart he: רב"ש => מאמרים => מי שחיזק לבו
zghGZeXQ en: Rabash => Articles => We Should Always Discern between Torah and Work he: רב"ש => מאמרים => יש תמיד להבחין בין תורה לעבודה
54EIkJ0X en: Rabash => Articles => The Whole of the Torah Is One Holy Name he: רב"ש => מאמרים => כל התורה היא שם אחד קדוש
uHWIXsHt en: Rabash => Articles => On My Bed at Night he: רב"ש => מאמרים => על משכבי בלילות
Z7UrEUYU en: Rabash => Articles => Three Times in the Work he: רב"ש => מאמרים => ג' זמנים בעבודה
TZQ5QKT4 en: Rabash => Articles => In Every Thing We Must Discern between Light and Kli he: רב"ש => מאמרים => בכל דבר יש להבחין בין אור לכלי
amy4DrAi en: Rabash => Articles => Show Me Your Glory he: רב"ש => מאמרים => הראני את כבודך
prVVNKgY en: Rabash => Articles => Repentance he: רב"ש => מאמרים => מאמר התשובה
pPYp6ptf en: Rabash => Articles => The Spies he: רב"ש => מאמרים => מאמר המרגלים
UtOdu8dE en: Rabash => Articles => The Lord Is Near to All Who Call upon Him he: רב"ש => מאמרים => קרוב ה' לכל קוראיו
2lPmvjOD en: Rabash => Articles => Three Prayers he: רב"ש => מאמרים => ג' תפלות 
CtSCccqc en: Rabash => Articles => One Does Not Regard Oneself as Wicked he: רב"ש => מאמרים => אין אדם משים עצמו רשע
F2Gu70v8 en: Rabash => Articles => Concerning the Reward of the Receivers he: רב"ש => מאמרים => בענין השכר המקבלים
vAtZkwOg en: Rabash => Articles => The Felons of Israel he: רב"ש => מאמרים => פושעי ישראל
yVCJSI1u en: Rabash => Articles => And I Pleaded with the Lord he: רב"ש => מאמרים => ואתחנן אל ה'
6q1mxCQD en: Rabash => Articles => When a Person Knows What Is Fear of the Creator he: רב"ש => מאמרים => כשאדם יודע מהי יראת ה'
sByulJEa en: Rabash => Articles => And There Was Evening and There Was Morning he: רב"ש => מאמרים => ויהי ערב ויהי בוקר
pjF1TnMJ en: Rabash => Articles => Who Testifies to a Person he: רב"ש => מאמרים => מי מעיד על האדם
VIFMP3fZ en: Rabash => Articles => A Righteous Who Is Happy, a Righteous Who Is Suffering he: רב"ש => מאמרים => צדיק וטוב לו, צדיק ורע לו
78dZN0eH en: Rabash => Articles => Hear Our Voice he: רב"ש => מאמרים => שמע קולנו
OnWQPVvx en: Rabash => Articles => Moses Went he: רב"ש => מאמרים => וילך משה
KVm5OSbv en: Rabash => Articles => Lend Ear, O Heaven he: רב"ש => מאמרים => האזינו השמים
G3YNMhx2 en: Rabash => Articles => Man Is Rewarded with Righteousness and Peace through the Torah he: רב"ש => מאמרים => מהו שע"י תורה זוכה האדם לצדקה ולשלום
njyR42n9 en: Rabash => Articles => Concerning Hesed [Mercy] he: רב"ש => מאמרים => ענין החסד
ynstTClg en: Rabash => Articles => Concerning Respecting the Father he: רב"ש => מאמרים => בענין כבוד אב
2nYogn9T en: Rabash => Articles => Confidence he: רב"ש => מאמרים => ענין בטחון
0G3JOBRv en: Rabash => Articles => The Importance of a Prayer of Many he: רב"ש => מאמרים => חשיבותה של תפילת רבים
Pplo29Za en: Rabash => Articles => Concerning Help that Comes from Above he: רב"ש => מאמרים => בענין העזרה הבאה מלמעלה
OdBdnhw6 en: Rabash => Articles => Concerning the Hanukkah Candle he: רב"ש => מאמרים => בענין נר חנוכה
XYXyx3Yk en: Rabash => Articles => Concerning Prayer he: רב"ש => מאמרים => ענין תפלה
RuftdnOm en: Rabash => Articles => A Real Prayer Is over a Real Deficiency he: רב"ש => מאמרים => תפלה אמיתית היא על חסרון אמיתי
HnW0CM9V en: Rabash => Articles => What Is the Main Deficiency for which One Should Pray? he: רב"ש => מאמרים => החסרון העקרי שעליו להתפלל, מהו
RVlfRn0W en: Rabash => Articles => Come unto Pharaoh – 2 he: רב"ש => מאמרים => בא אל פרעה - ב
jvqBafbo en: Rabash => Articles => What Is the Need to Borrow Vessels from the Egyptians? he: רב"ש => מאמרים => מהו הצורך לשאילת כלים מהמצרים
E9tXXYJv en: Rabash => Articles => A Prayer of Many he: רב"ש => מאמרים => תפלת רבים
dvyNpMMt en: Rabash => Articles => The Lord Has Chosen Jacob for Himself he: רב"ש => מאמרים => כי יעקב בחר לו יה
oR4gtgR7 en: Rabash => Articles => The Agenda of the Assembly - 2 he: רב"ש => מאמרים => סדר ישיבת התועדות
0nZpYzNj en: Rabash => Articles => Who Causes the Prayer he: רב"ש => מאמרים => מי הוא הגורם לתפילה
oQDtEtn5 en: Rabash => Articles => Concerning Joy he: רב"ש => מאמרים => ענין שמחה
I0HKLPBX en: Rabash => Articles => Should One Sin and Be Guilty he: רב"ש => מאמרים => והיה כי יחטא ואשם
rlfCtHnc en: Rabash => Articles => Concerning Above Reason he: רב"ש => מאמרים => ענין למעלה מהדעת
710qDaOT en: Rabash => Articles => If a Woman Inseminates he: רב"ש => מאמרים => אשה כי תזריע
WoE6QFg9 en: Rabash => Articles => Concerning Fear and Joy he: רב"ש => מאמרים => ענין יראה ושמחה
tvakPjiC en: Rabash => Articles => The Difference between Charity and Gift he: רב"ש => מאמרים => ההבדל בין צדקה למתנה
3ZVmhIG8 en: Rabash => Articles => The Measure of Practicing Mitzvot [Commandments] he: רב"ש => מאמרים => שיעור מעשי המצות
YeBcTsJm en: Rabash => Articles => A Near Way and a Far Way he: רב"ש => מאמרים => דרך קרובה ודרך רחוקה
JbOa0tTI en: Rabash => Articles => The Creator and Israel Went into Exile he: רב"ש => מאמרים => הקב"ה וישראל יצאו בגלות
VLuOsaN3 en: Rabash => Articles => A Congregation Is No Less than Ten he: רב"ש => מאמרים => אין עדה פחות מעשרה
63G16wpa en: Rabash => Articles => Lishma and Lo Lishma he: רב"ש => מאמרים => לשמה ושלא לשמה
xuj6yyt6 en: Rabash => Articles => The Klipa [Shell/Peel] that Precedes the Fruit he: רב"ש => מאמרים => ענין קליפה שקדמה לפרי
GZaeoDzY en: Rabash => Articles => Concerning Yenika [Suckling] and Ibur [Impregnation] he: רב"ש => מאמרים => ענין יניקה, ועיבור
u9Frmax3 en: Rabash => Articles => The Reason for Straightening the Legs and Covering the Head During the Prayer he: רב"ש => מאמרים => ענין שבתפלה צריכים ליישור רגלים ולכסוי ראש
aOQqhPKf en: Rabash => Articles => What Are Commandments that a Person Tramples with His Feet he: רב"ש => מאמרים => מהו מצות שאדם דש בעקביו
aSZQSYTe en: Rabash => Articles => Judges and Officers he: רב"ש => מאמרים => ענין שופטים ושוטרים
LyCHKK53 en: Rabash => Articles => The Fifteenth of Av he: רב"ש => מאמרים => חמשה עשר באב
8iqBPcUa en: Rabash => Articles => What Is Preparation for the Selichot [Forgiveness] he: רב"ש => מאמרים => הכנה לסליחות מהו
dR5sqn3i en: Rabash => Articles => The Good Who Does Good, to the Bad and to the Good he: רב"ש => מאמרים => הטוב ומטיב לרעים ולטובים
swRcZglf en: Rabash => Articles => The Importance of Recognition of Evil he: רב"ש => מאמרים => ענין חשיבות הכרת הרע
rnsIfDYK en: Rabash => Articles => All of Israel Have a Part in the Next World he: רב"ש => מאמרים => כל ישראל יש להם חלק לעולם הבא
NhxBZ1Yh en: Rabash => Articles => It is Forbidden to Hear a Good Thing From a Bad Person he: רב"ש => מאמרים => מאדם רע אסור לשמוע דבר טוב
iMFoTFLm en: Rabash => Articles => What Is the Advantage in the Work More than in the Reward? he: רב"ש => מאמרים => מהו היתרון שיש בעבודה יותר משכר
cXVknwWN en: Rabash => Articles => The Importance of Faith that Is Always Present he: רב"ש => מאמרים => חשיבותה של האמונה, שנוהגת תמיד
gsibxkK4 en: Rabash => Articles => The Miracle of Hanukkah he: רב"ש => מאמרים => נס החנוכה
D1iuY5Cm en: Rabash => Articles => The Difference between Mercy and Truth and Untrue Mercy he: רב"ש => מאמרים => ההבדל בין חסד ואמת, לחסד שאינו אמת
dh5oiAeP en: Rabash => Articles => One’s Greatness Depends on the Measure of One’s Faith in the Future he: רב"ש => מאמרים => גדלות האדם תלויה בשיעור אמונתו בעתיד
uJ34bVH5 en: Rabash => Articles => What Is the Substance of Slander and Against Whom Is It? he: רב"ש => מאמרים => מהו החומר דלשון הרע וכנגד מי הוא
48xIxpnS en: Rabash => Articles => Purim, and the Commandment: Until He until He Does Not Know he: רב"ש => מאמרים => פורים, שהמצוה עד דלא ידע
cMeFP01l en: Rabash => Articles => What Is Half a Shekel in the Work - 1 he: רב"ש => מאמרים => מהו מחצית השקל בעבודה - א
89q8MRQh en: Rabash => Articles => Why the Festival of Matzot Is Called Passover he: רב"ש => מאמרים => מדוע חג המצות נקרא "פסח"
KwE2IyGL en: Rabash => Articles => The Connection between Passover, Matza, and Maror he: רב"ש => מאמרים => הקשר בין פסח, מצה ומרור
WPLnImvI en: Rabash => Articles => Two Discernments in Holiness he: רב"ש => מאמרים => ב' בחינות בקדושה
GjJryblQ en: Rabash => Articles => The Difference between the Work of the General Public and the Work of the Individual  he: רב"ש => מאמרים => ההבדל בין עבודת הכלל ופרט
yPfHd070 en: Rabash => Articles => The Severity of Teaching Idol Worshippers the Torah he: רב"ש => מאמרים => מהות חומרת איסור לימוד תורה לעכויים
tRJZ11Xs en: Rabash => Articles => What Is Preparation for Reception of the Torah - 1 he: רב"ש => מאמרים => הכנה לקבלת התורה מהו - א
p9d3brlk en: Rabash => Articles => What Are Revealed and Concealed in the Work of the Creator? he: רב"ש => מאמרים => מהו נסתר ונגלה בעבודה ה'
gCman0H8 en: Rabash => Articles => What Is Man’s Private Possession? he: רב"ש => מאמרים => רכוש הפרטי של אדם מהו
Y6aUyW1F en: Rabash => Articles => What Are Dirty Hands in the Work of the Creator? he: רב"ש => מאמרים => מהו ידים מלוכלכות בעבודת ה'
dy0XoB0d en: Rabash => Articles => What Is the Gift that a Person Asks of the Creator? he: רב"ש => מאמרים => מהי המתנה שאדם מבקש מה'
uvx3U7uZ en: Rabash => Articles => Peace After a Dispute Is More Important than Having No Disputes At All he: רב"ש => מאמרים => שלום אחר מחלוקת יותר חשוב משאין מחלוקת כלל
DIKpvrLy en: Rabash => Articles => What is Unfounded Hatred in the Work he: רב"ש => מאמרים => שנאת חינם בעבודה מהו
jssgSMdA en: Rabash => Articles => What Is Heaviness of the Head in the Work? he: רב"ש => מאמרים => כובד ראש בעבודה מהו
wzG9o93N en: Rabash => Articles => What Is a Light Commandment he: רב"ש => מאמרים => מהי מצוה קלה
dtX7hvFC en: Rabash => Articles => What Are “Blessing” and “Curse” in the Work? he: רב"ש => מאמרים => מהו קללה וברכה בעבודה
sjeWEDkG en: Rabash => Articles => What Is Do Not Add and Do Not Take Away in the Work? he: רב"ש => מאמרים => מהו לא תוסיף ולא תגרע בעבודה
Mvd6uHHi en: Rabash => Articles => What Is “According to the Sorrow, So Is the Reward”? he: רב"ש => מאמרים => מהו לפום צערא אגרא
h6dZnpEq en: Rabash => Articles => What Is a War Over Authority in the Work – 1 he: רב"ש => מאמרים => מהי מלחמת הרשות, בעבודה - א
cQYbzWhd en: Rabash => Articles => What Is Making a Covenant in the Work he: רב"ש => מאמרים => מהו כריתת ברית בעבודה
wSi0R6Zo en: Rabash => Articles => Why Life Is Divided into Two Discernments he: רב"ש => מאמרים => מדוע החיים נחלק לב' בחינות
5gTAv5Jq en: Rabash => Articles => What Is the Extent of Teshuva [Repentance]? he: רב"ש => מאמרים => עד כמה שיעור התשובה
ts8yVrgh en: Rabash => Articles => What It Means that the Name of the Creator is “Truth” he: רב"ש => מאמרים => מהו ששמו של הקב"ה נקרא אמת
VEqM1iFZ en: Rabash => Articles => What Is the Prayer for Help and for Forgiveness in the Work? he: רב"ש => מאמרים => מהי התפלה על עזרה ועל סליחה, בעבודה
zhJfCGHC en: Rabash => Articles => What Is, “When Israel Are in Exile, the Shechina Is with Them,” in the Work? he: רב"ש => מאמרים => מהו בעבודה, ישראל שגלו - שכינה עמהם
HNGCfJBb en: Rabash => Articles => What Is the Difference between a Field and a Man of the Field, in the Work? he: רב"ש => מאמרים => מהו ההבדל בין שדה לאיש שדה בעבודה
Uj6eOBAJ en: Rabash => Articles => What Is the Importance of the Groom, that His Iniquities Are Forgiven? he: רב"ש => מאמרים => מהי חשיבות החתן, שמוחלין לו עוונותיו
NMmjiWtI en: Rabash => Articles => What Does It Mean that One Who Prays Should Explain His Words Properly? he: רב"ש => מאמרים => מהו שהמתפלל צריך לפרש דבריו כראוי
ieMzaq3B en: Rabash => Articles => What Does It Mean that the Righteous Suffers Afflictions? he: רב"ש => מאמרים => מהו שהצדיק סובל רעות
108WXSGi en: Rabash => Articles => What Are the Four Qualities of Those Who Go to the Seminary, in the Work? he: רב"ש => מאמרים => מהו הד' מידות בהולכי בית המדרש, בעבודה
OPUXaZq6 en: Rabash => Articles => What Are the Two Discernments before Lishma? he: רב"ש => מאמרים => מהו הב' הבחנות שלפני לשמה
g1SHtFsp en: Rabash => Articles => What Are Torah and Work in the Way of the Creator? he: רב"ש => מאמרים => מהו תורה ומלאכה בדרך ה'
ZRoWVoV3 en: Rabash => Articles => What Is “the People’s Shepherd Is the Whole People” in the Work? he: רב"ש => מאמרים => מהו רועה העם הוא כל העם, בעבודה
jmrf8Gud en: Rabash => Articles => The Need for Love of Friends he: רב"ש => מאמרים => הצורך לאהבת חברים
4Z0iSPJQ en: Rabash => Articles => What Is “There Is No Blessing in an Empty Place” in the Work? he: רב"ש => מאמרים => מהו אין ברכה במקום ריק, בעבודה
S1J4xrdu en: Rabash => Articles => What Is the Foundation on which Kedusha [Holiness] Is Built? he: רב"ש => מאמרים => מהו היסוד שהקדושה נבנית עליו
uTJuZwwj en: Rabash => Articles => The Main Difference between a Beastly Soul and a Godly Soul he: רב"ש => מאמרים => ההבחן העקרי בין נפש הבהמית לנפש אלקית
MoTXt3jg en: Rabash => Articles => When Is One Considered “A Worker of the Creator” in the Work? he: רב"ש => מאמרים => מתי נקרא עובד ה' בעבודה
xsG6inj3 en: Rabash => Articles => What Are Silver, Gold, Israel, Rest of Nations, in the Work? he: רב"ש => מאמרים => מהו כסף, זהב, ישראל, שאר עמים, בעבודה
bC9gTPcE en: Rabash => Articles => What Is the Reward in the Work of Bestowal? he: רב"ש => מאמרים => מהו השכר בעבודה דלהשפיע
efI9C0NE en: Rabash => Articles => What Does It Mean that the Torah Was Given Out of the Darkness in the Work? he: רב"ש => מאמרים => מהו שהתורה נתנה מתוך החושך בעבודה
cycVe2du en: Rabash => Articles => What Are Merits and Iniquities of a Righteous in the Work? he: רב"ש => מאמרים => מהו זכיות ועוונות אצל צדיק בעבודה
Gv2kfnec en: Rabash => Articles => What Beginning in Lo Lishma Means in the Work he: רב"ש => מאמרים => מהו שמתחילים בשלא לשמה, בעבודה
7gTXI6h3 en: Rabash => Articles => What Is “The Concealed Things Belong to the Lord, and the Revealed Things Belong to Us,” in the Work? he: רב"ש => מאמרים => מהו הנסתרות לה' והנגלות לנו, בעבודה
Zzcey1Vc en: Rabash => Articles => What Is the Preparation on the Eve of Shabbat, in the Work? he: רב"ש => מאמרים => מהי ההכנה בערב שבת, בעבודה
TpCRaC3k en: Rabash => Articles => What Is the Difference between Law and Judgment in the Work? he: רב"ש => מאמרים => מהו ההבדל בין חוק למשפט, בעבודה
btYyM6yY en: Rabash => Articles => What Is, “The Creator Does Not Tolerate the Proud,” in the Work? he: רב"ש => מאמרים => מהו שהמתגאה אין הקב"ה סובלו, בעבודה
fLKVA4IK en: Rabash => Articles => What Is, His Guidance Is Concealed and Revealed? he: רב"ש => מאמרים => מהו השגחתו יתברך היא בהסתר ובנגלה
p1agMOVk en: Rabash => Articles => How to Recognize One Who Serves God from One Who Does Not Serve Him he: רב"ש => מאמרים => מהו ההיכר בין עובד אלקים ללא עבדו
WhVeEvxC en: Rabash => Articles => What to Look for in the Assembly of Friends he: רב"ש => מאמרים => מה לדרוש מאסיפת חברים
uixGVjm4 en: Rabash => Articles => What Is the Work of Man, in the Work that Is Attributed to the Creator? he: רב"ש => מאמרים => מהי הפעולה שבאדם בדרך העבודה, שמיחסים לה'
UgsTAAFY en: Rabash => Articles => What Are the Two Actions During a Descent? he: רב"ש => מאמרים => מה הן הב' פעולות שבזמן ירידה
CghR9wSw en: Rabash => Articles => What Is the Difference between General and Individual in the Work of the Creator? he: רב"ש => מאמרים => מהו ההבדל בעבודת ה' בין כללי לפרטי
fV05K6S3 en: Rabash => Articles => What Are Day and Night in the Work? he: רב"ש => מאמרים => מהו יום ולילה, בעבודה
hYV2hoix en: Rabash => Articles => What Is the Help in the Work that One Should Ask of the Creator? he: רב"ש => מאמרים => מהי העזרה בעבודה, שיבקש מה'
nHgIRfEt en: Rabash => Articles => What Is the Measure of Repentance? he: רב"ש => מאמרים => מהו השיעור של תשובה
ZM6ozzjb en: Rabash => Articles => What Is a Great or a Small Sin in the Work? he: רב"ש => מאמרים => מהו חטא גדול או קטן, בעבודה
apwn7gg2 en: Rabash => Articles => What Is the Difference between the Gate of Tears and the Rest of the Gates? he: רב"ש => מאמרים => מהו השינוי שבשער הדמעות משאר שערים
iGc5BTBe en: Rabash => Articles => What Is a Flood of Water in the Work? he: רב"ש => מאמרים => מהו מבול מים, בעבודה
pISU4FwT en: Rabash => Articles => What Does It Mean that the Creation of the World Was by Largess? he: רב"ש => מאמרים => מהו שבריאת העולם היה בנדבה
9pziecsz en: Rabash => Articles => What Is Above Reason in the Work? he: רב"ש => מאמרים => מהו למעלה מהדעת, בעבודה
lu2uYTZr en: Rabash => Articles => What Is “He Who Did Not Toil on the Eve of Shabbat, What Will He Eat on Shabbat” in the Work? he: רב"ש => מאמרים => מהו "מי שלא טרח בערב שבת, מה יאכל בשבת" בעבודה
p8TsoRvC en: Rabash => Articles => What It Means, in the Work, that If the Good Grows, So Grows the Bad he: רב"ש => מאמרים => מהו שאם הטוב מתגדל, גם הרע מתגדל, בעבודה
0uyGxQLE en: Rabash => Articles => What Is, “Calamity that Comes upon the Wicked Begins with the Righteous,” in the Work? he: רב"ש => מאמרים => מהו פורענות הבאה לרשעים מתחלת מן הצדיקים, בעבודה
r4hAXTYs en: Rabash => Articles => What Does It Mean that the Ladder Is Diagonal, in the Work? he: רב"ש => מאמרים => מהו שהסולם הוא באלכסון, בעבודה
O9k8SmOI en: Rabash => Articles => What Are the Forces Required in the Work? he: רב"ש => מאמרים => מהם הכוחות, שצריכים בעבודה
KETc27ua en: Rabash => Articles => What Is a Groom’s Meal? he: רב"ש => מאמרים => מהי סעודת חתן
hzVuGUg8 en: Rabash => Articles => What Is the “Bread of an Evil-Eyed Man” in the Work? he: רב"ש => מאמרים => מהו "לחם רע עין" בעבודה
krqPaGUQ en: Rabash => Articles => What Is the Meaning of “Reply unto Your Heart”? he: רב"ש => מאמרים => מהו שכתוב "והשבות אל לבבך"
slesA9Ce en: Rabash => Articles => What Is, “The Righteous Become Apparent through the Wicked,” in the Work? he: רב"ש => מאמרים => מהו שהצדיקים ניכרים ע"י הרשעים, בעבודה
hg3XRM9C en: Rabash => Articles => What Is the Prohibition to Bless on an Empty Table, in the Work? he: רב"ש => מאמרים => מה הוא האיסור לברך על שולחן ריק, בעבודה
BSpe81WK en: Rabash => Articles => What Is the Prohibition to Greet Before Blessing the Creator, in the Work? he: רב"ש => מאמרים => מהו האיסור לתת שלום מטרם שמברך לה', בעבודה
6DKu2Kxn en: Rabash => Articles => What Is, “There Is No Blessing in That Which Is Counted,” in the Work? he: רב"ש => מאמרים => מהו שהברכה אינה שורה בדבר שנמנה, בעבודה
94OfHXsv en: Rabash => Articles => Why Is Shabbat Called Shin-Bat in the Work? he: רב"ש => מאמרים => מהו ששבת נקראת "ש-בת", בעבודה
b58HV9VF en: Rabash => Articles => What Does It Mean that the Evil Inclination Ascends and Slanders, in the Work? he: רב"ש => מאמרים => מהו שהיצר הרע עולה ומשטין בעבודה
Bx4x9pIe en: Rabash => Articles => What Is, “A Drunken Man Must Not Pray, in the Work? he: רב"ש => מאמרים => מהו "שכור אל יתפלל" בעבודה
5K0Fa5zc en: Rabash => Articles => Why Are Four Questions Asked Specifically on Passover Night? he: רב"ש => מאמרים => מהו שדוקא בליל פסח, שואלים ד' קשיות
cmfuFmbQ en: Rabash => Articles => What Is, If He Swallows the Bitter Herb, He Will Not Come Out, in the Work? he: רב"ש => מאמרים => מהו אם בלעו את המרור לא יצא, בעבודה
nrsKC3AI en: Rabash => Articles => What Is “Do Not Slight the Blessing of a Layperson” in the Work? he: רב"ש => מאמרים => מהו ברכת הדיוט אל תהי קלה בעיניך, בעבודה
RvAb2tbA en: Rabash => Articles => What Is “He Who Has a Flaw Shall Not Offer [Sacrifice]” in the Work? he: רב"ש => מאמרים => מהו בעבודה, איש אשר בו מום לא יקרב
sKImxYLV en: Rabash => Articles => What Is “He Who Defiles Himself Is Defiled from Above” in the Work? he: רב"ש => מאמרים => מהו מי שמטמא עצמו, מטמאים אותו מלמעלה, בעבודה
EiiLJvUZ en: Rabash => Articles => What Is the Meaning of Suffering in the Work? he: רב"ש => מאמרים => מהו ענין יסורים בעבודה
sYRZ5z5a en: Rabash => Articles => Who Needs to Know that a Person Withstood the Test? he: רב"ש => מאמרים => הידיעה שהאדם עמד בנסיון, לצורך מי
h0aXvOGC en: Rabash => Articles =>  What Is the Preparation to Receive the Torah in the Work?-2 he: רב"ש => מאמרים => מהו הכנה לקבלת התורה, בעבודה - ב
3FgJwsfb en: Rabash => Articles => What Is the Meaning of Lighting the Menorah in the Work? he: רב"ש => מאמרים => מהו ענין הדלקת המנורה, בעבודה
0UYOEq4I en: Rabash => Articles => What Is the Prohibition to Teach Torah to Idol-Worshippers in the Work? he: רב"ש => מאמרים => מהו אסור ללמוד תורה לעכו"ם, בעבודה
ns4uiXxb en: Rabash => Articles => What Does It Mean that Oil Is Called “Good Deeds” in the Work? he: רב"ש => מאמרים => מהו ששמן נקרא מעשים טובים, בעבודה
SDNL13Tm en: Rabash => Articles => What Are Spies in the Work? he: רב"ש => מאמרים => מהו בחינת מרגלים, בעבודה
CSbfhQNs en: Rabash => Articles => What Is Peace in the Work? he: רב"ש => מאמרים => מהו שלום בעבודה
PGOZkUEY en: Rabash => Articles => What Is, “He Who Is Without Sons,” in the Work? he: רב"ש => מאמרים => מהו מי שאין לו בנים, בעבודה
QuLjmsam en: Rabash => Articles => What Is “For It Is Your Wisdom and Understanding in the Eyes of the Nations,” in the Work? he: רב"ש => מאמרים => מהו כי היא חכמתכם ובינתכם לעיני העמים, בעבודה
uWnEICJx en: Rabash => Articles => What Is “A Road Whose Beginning Is Thorns and Its End Is a Plain” in the Work? he: רב"ש => מאמרים => מהו "דרך שתחילתה קוצים וסופה מישור" בעבודה
9krGzKpb en: Rabash => Articles => What Are Judges and Officers in the Work? he: רב"ש => מאמרים => מהו שופטים ושוטרים, בעבודה
zsDapzME en: Rabash => Articles => What Is, “The Torah Speaks Only Against the Evil Inclination,” in the Work? he: רב"ש => מאמרים => מהו לא דברה תורה אלא נגד יצר הרע, בעבודה
OafMyHqt en: Rabash => Articles => What Is, “Every Day They Will Be as New in Your Eyes,” in the Work? he: רב"ש => מאמרים => מהו בכל יום יהיו בעיניך כחדשים, בעבודה
dawsf4Um en: Rabash => Articles => The Daily Schedule he: רב"ש => מאמרים => סדר היום
axZAod4o en: Rabash => Articles => What Does “May We Be the Head and Not the Tail” Mean in the Work? he: רב"ש => מאמרים => מהו ענין שנהיה לראש ולא לזנב בעבודה
JISUC453 en: Rabash => Articles => What Is the Meaning of Failure in the Work? he: רב"ש => מאמרים => מהו ענין כשלון בעבודה
Unt3S41H en: Rabash => Articles => What It Means that the World Was Created for the Torah he: רב"ש => מאמרים => מהו שהעולם נברא בשביל תורה
V3Jbenab en: Rabash => Articles => What It Means that the Generations of the Righteous are Good Deeds, in the Work he: רב"ש => מאמרים => מהו שתולדות הצדיקים הם מעשים טובים, בעבודה
QiHjIAz0 en: Rabash => Articles => What It Means that the Land Did Not Bear Fruit before Man Was Created, in the Work he: רב"ש => מאמרים => מהו שהארץ לא הוציאה פירות מטרם שנברא האדם, בעבודה
ON1025W1 en: Rabash => Articles => When Should One Use Pride in the Work? he: רב"ש => מאמרים => מתי האדם צריך להשתמש עם גאוה, בעבודה
fft2fyey en: Rabash => Articles => What Are the Times of Prayer and Gratitude in the Work? he: רב"ש => מאמרים => מתי הם הזמנים של תפלה והודאה, בעבודה
tFundSFL en: Rabash => Articles => What It Means that Esau Was Called “A Man of the Field,” in the Work he: רב"ש => מאמרים => מהו שעשו נקרא איש שדה, בעבודה
gUG2bIe0 en: Rabash => Articles => What Is, “A Ladder Is Set on the Earth, and Its Top Reaches Heaven,” in the Work? he: רב"ש => מאמרים => מהו סולם מוצב ארצה וראשו מגיע השמימה
txEbWFaA en: Rabash => Articles => What Does It Mean that Our Sages Said, “King David Did Not Have a Life,” in the Work? he: רב"ש => מאמרים => מהו שאמרו חז"ל, שדוד המלך לא היו לו חיים, בעבודה
qQoJhgFB en: Rabash => Articles => What Placing the Hanukkah Candle on the Left Means in the Work he: רב"ש => מאמרים => מהו שנר חנוכה מניחה בשמאל, בעבודה
fRA8LXdh en: Rabash => Articles => Why Is the Torah Called “Middle Line” in the Work? - 1 he: רב"ש => מאמרים => מהו שהתורה נקראת קו אמצעי, בעבודה - א
cqSRUE13 en: Rabash => Articles => What Does It Mean that by the Unification of the Creator and the Shechina, All Iniquities Are Atoned? he: רב"ש => מאמרים => מהו שעל ידי יחוד קוב"ה ושכינתה כל העוונות מתכפרים
RQC14tsC en: Rabash => Articles => What Is True Hesed in the Work? he: רב"ש => מאמרים => מהו בעבודה "חסד של אמת"
3VTIkIJf en: Rabash => Articles => What Does It Mean that Before the Egyptian Minister Fell, Their Outcry Was Not Answered, in the Work? he: רב"ש => מאמרים => מהו שמטרם שנפל השר המצרי, לא נענו בצעקתם, בעבודה
2LhAKGGw en: Rabash => Articles => What Is “For Lack of Spirit and for Hard Work,” in the Work? he: רב"ש => מאמרים => מהו מקוצר רוח ומעבודה קשה, בעבודה
AXMh7Wca en: Rabash => Articles => What Is the Assistance that He who Comes to Purify Receives in the Work? he: רב"ש => מאמרים => מהו הסיוע, שהבא לטהר מקבל, בעבודה
UyiU39Aw en: Rabash => Articles => Why the Speech of Shabbat Must Not Be as the Speech of a Weekday, in the Work he: רב"ש => מאמרים => מהו שדיבור של שבת לא יהיה כדיבור של חול, בעבודה
QYFQxHQL en: Rabash => Articles => Why Is the Torah Called “Middle Line” in the Work?-2 he: רב"ש => מאמרים => מהו שהתורה נקראת קו אמצעי, בעבודה - ב
nCp1rMZY en: Rabash => Articles => What Is Half a Shekel in the Work? - 2 he: רב"ש => מאמרים => מהו מחצית השקל בעבודה - ב
4YYhQG27 en: Rabash => Articles => What Is, “As I Am for Nothing, so You Are for Nothing,” in the Work? he: רב"ש => מאמרים => מהו מה אני בחנם, אף אתם בחנם, בעבודה
xkJeNx7d en: Rabash => Articles => What Is the Order in Blotting Out Amalek? he: רב"ש => מאמרים => מהו הסדר במחית עמלק
XTPkUysb en: Rabash => Articles => What Does It Mean that Moses Was Perplexed about the Birth of the Moon, in the Work? he: רב"ש => מאמרים => מהו שנתקשה משה על מולד הלבנה, בעבודה
hIGwKKjR en: Rabash => Articles => What Does, “Everything that Comes to Be a Burnt Offering Is Male,” Mean in the Work? he: רב"ש => מאמרים => מהו שכל הבא לקרבן עולה הוא זכר, בעבודה
8pwqj3Yg en: Rabash => Articles => What Is, “Praise the Lord, All Nations,” in the Work? he: רב"ש => מאמרים => מהו "הללו את ה' כל גויים" בעבודה
okiLeVeU en: Rabash => Articles => What Is, “There Is None as Holy as the Lord, for There Is None Besides You,” in the Work? he: רב"ש => מאמרים => מהו "אין קדוש כה', כי אין בלתך" בעבודה
DreMbEFQ en: Rabash => Articles => What Is, “Every Blade of Grass Has an Appointee Above, Who Strikes It and Tells It, Grow!” in the Work? he: רב"ש => מאמרים => מהו שכל עשב - יש ממונה למעלה המכה אותו ואומר גדל, בעבודה
1pscTRuT en: Rabash => Articles => What Is, “Warn the Great about the Small,” in the Work? he: רב"ש => מאמרים => מהו "להזהיר גדולים על קטנים" בעבודה
PPylX8BX en: Rabash => Articles => What Is, “The Torah Exhausts a Person’s Strength,” in the Work? he: רב"ש => מאמרים => מהו "התורה מתשת כוחו של אדם" בעבודה
tg0P0421 en: Rabash => Articles => What It Means that “Law and Ordinance” Is the Name of the Creator in the Work he: רב"ש => מאמרים => מהו שחוק ומשפט הוא השם של הקב"ה, בעבודה
DxQcNJE8 en: Rabash => Articles => What “There Is No Blessing in That Which Is Counted” Means in the Work he: רב"ש => מאמרים => מהו שאין הברכה מצויה בדבר שבמנין, בעבודה
gPa1zUoe en: Rabash => Articles => What “Israel Do the Creator’s Will” Means in the Work he: רב"ש => מאמרים => מהו ישראל עושין רצון המקום, בעבודה
KTRc6e1w en: Rabash => Articles => What Is “The Earth Feared and Was Still,” in the Work? he: רב"ש => מאמרים => מהי "ארץ יראה ושקטה", בעבודה
c454mVY0 en: Rabash => Articles => What Are “A Layperson’s Vessels,” in the Work? he: רב"ש => מאמרים => מהו כלים הדיוטות, בעבודה
aUo4OvBi en: Rabash => Articles => What Is “He Who Enjoys at a Groom’s Meal,” in the Work? he: רב"ש => מאמרים => מהו "הנהנה מסעודת חתן" בעבודה
VPgzX4oT en: Rabash => Articles => What Is, “The Children of Esau and Ishmael Did Not Want to Receive the Torah,” in the Work? he: רב"ש => מאמרים => מהו שבני עשו וישמעאל לא רצו לקבל את התורה, בעבודה
FQAQq152 en: Rabash => Articles => What Is, “The Shechina Is a Testimony to Israel,” in the Work? he: רב"ש => מאמרים => מהו "השכינה היא עדות על ישראל" בעבודה
XhgHflei en: Rabash => Articles => What Is, “A Cup of Blessing Must Be Full,” in the Work? he: רב"ש => מאמרים => מהו כוס של ברכה צריך להיות מלא, בעבודה
GSe727DO en: Rabash => Articles => What Is, “Anyone Who Mourns forJerusalem Is Rewarded with Seeing Its Joy,” in the Work? he: רב"ש => מאמרים => מהו "כל המתאבל על ירושלים, זוכה ורואה בשמחתה" בעבודה
wuvVuoNW en: Rabash => Articles => What Is, “For You Are the Least of All the Peoples,” in the Work? he: רב"ש => מאמרים => מהו כי אתם המעט מכל העמים, בעבודה
eCoy5wXp en: Rabash => Articles => What Are the Light Mitzvot that a Person Tramples with His Heels, in the Work? he: רב"ש => מאמרים => מהו המצות קלות שאדם דש בעקביו, בעבודה
QjYh439V en: Rabash => Articles => What Are a Blessing and a Curse, in the Work? he: רב"ש => מאמרים => מהו ברכה וקללה, בעבודה
I0cVIub2 en: Rabash => Articles => What Is, “You Shall Not Plant for Yourself an Asherah by the Altar,” in the Work? he: רב"ש => מאמרים => מהו לא תיטע לך אשרה אצל מזבח, בעבודה
kQ41Qw7p en: Rabash => Articles => What Is an Optional War, in the work? - 2 he: רב"ש => מאמרים => מהי מלחמת הרשות, בעבודה - ב
tvgcs5xh en: Rabash => Articles => What Is, “The Concealed Things Belong to the Lord Our God,” in the work? he: רב"ש => מאמרים => מהו "הנסתרות לה' אלקינו" בעבודה
znQilovN en: Rabash => Articles => The Order of the Work, from Baal HaSulam he: רב"ש => מאמרים => סדר עבודה מבעל הסולם זצ"ל
KTsJOdF8 en: Rabash => Articles => What Is, “We Have No Other King But You,” in the Work? he: רב"ש => מאמרים => מהו אין לנו מלך אלא אתה, בעבודה
sBb6MaqA en: Rabash => Articles => What Is, “Return, O Israel, Unto the Lord Your God,” in the Work? he: רב"ש => מאמרים => מהו שובה ישראל עד ה' אלקיך, בעבודה
SkzRd3n0 en: Rabash => Articles => What Is, “The Wicked Will Prepare and the Righteous Will Wear,” in the Work? he: רב"ש => מאמרים => מהו רשע יכין וצדיק ילבש, בעבודה
r62FAcLO en: Rabash => Articles => What Is, “The Saboteur Was in the Flood, and Was Putting to Death,” in the Work? he: רב"ש => מאמרים => מהו שהמחבל היה נמצא בהמבול, והוא היה ממית, בעבודה
BtKrU4pD en: Rabash => Articles => What Is, “The Good Deeds of the Righteous Are the Generations,” in the Work? he: רב"ש => מאמרים => מהו מעשים טובים של צדיקים הם התולדות, בעבודה
T8DfLPJw en: Rabash => Articles => What Is, “The Herdsmen of Abram’s Cattle and the Herdsmen of Lot’s Cattle,” in the Work? he: רב"ש => מאמרים => מהו רועי מקנה אברם ורועי מקנה לוט, בעבודה
ZrivpTTN en: Rabash => Articles => What Is “Man” and What Is “Beast” in the Work? he: רב"ש => מאמרים => מהו אדם ומהו בהמה, בעבודה
FZGUMJ5Z en: Rabash => Articles => What Is, “And Abraham Was Old, of Many Days,” in the Work? he: רב"ש => מאמרים => מהו ואברהם זקן בא בימים, בעבודה
S0wHeAbt en: Rabash => Articles => What Is, “The Smell of His Garments,” in the Work? he: רב"ש => מאמרים => מהו ריח בגדיו, בעבודה
tsFItmB2 en: Rabash => Articles => What Does “The King Stands on His Field When the Crop Is Ripe” Mean in the Work? he: רב"ש => מאמרים => מהו שהמלך עומד על שדהו, כשהתבואה עומד בכרי, בעבודה
NMoOeizs en: Rabash => Articles => What It Means that the Good Inclination and the Evil Inclination Guard a Person in the Work he: רב"ש => מאמרים => מהו שהיצר טוב ויצר הרע שומרים לאדם, בעבודה
nk2bJKqx en: Rabash => Articles => These Candles Are Sacred he: רב"ש => מאמרים => הנרות הללו קדש הם
9oYWkHzF en: Rabash => Articles => What “You Have Given the Strong to the Hands of the Weak” Means in the Work he: רב"ש => מאמרים => מהו "מסרת גבורים ביד חלשים" בעבודה
zPkb5djw en: Rabash => Articles => What Does It Mean that Man’s Blessing Is the Blessing of the Sons, in the Work? he: רב"ש => מאמרים => מהו שברכת האדם היא ברכת הבנים, בעבודה
tbBAALn8 en: Rabash => Articles => What Is the Blessing, “Who Made a Miracle for Me in This Place,” in the Work? he: רב"ש => מאמרים => מהו הברכה "שעשה לי נס במקום הזה" בעבודה
Xa1ZxDGM en: Rabash => Articles => Why We Need “Reply unto Your Heart,” to Know that the Lord, He Is God, in the Work he: רב"ש => מאמרים => בכדי לדעת שה' הוא אלקים, צריכים ל"השבות אל לבבך" בעבודה
GvEuKR7I en: Rabash => Articles => What Is, “For I Have Hardened His Heart,” in the work? he: רב"ש => מאמרים => מהו כי אני הכבדתי את לבו, בעבודה
Nt2ksxxV en: Rabash => Articles => What It Means that We Should Raise the Right Hand over the Left Hand, in the Work he: רב"ש => מאמרים => מהו שצריך להרים יד ימין על שמאל, בעבודה
vK4btTLv en: Rabash => Articles => What Is, “Rise Up, O Lord, and Let Your Enemies Be Scattered,” in the Work? he: רב"ש => מאמרים => מהו קומה ה' ויפוצו אויביך, בעבודה
Q5sEhtrB en: Rabash => Articles => What Is, “There Is Nothing that Has No Place,” in the Work? he: רב"ש => מאמרים => מהו אין לך דבר שאין לו מקום, בעבודה
vCbYXseJ en: Rabash => Articles => What Does It Mean that We Read the Portion, Zachor [Remember], Before Purim, in the Work? he: רב"ש => מאמרים => מהו שקוראים פרשת זכור לפני פורים, בעבודה
XQFVY57p en: Rabash => Articles => What Is “A Lily Among the Thorns,” in the Work? he: רב"ש => מאמרים => מהי שושנה בין החוחים, בעבודה
GaZtLpMe en: Rabash => Articles => What Is the Meaning of the Purification of a Cow’s Ashes, in the Work? he: רב"ש => מאמרים => מהו ענין טהרת אפר פרה, בעבודה
MzCpjqT4 en: Rabash => Articles => What Does It Mean that One Should Bear a Son and a Daughter, in the Work? he: רב"ש => מאמרים => מהו שהאדם צריך להוליד בן ובת, בעבודה
deXN9qdo en: Rabash => Articles => What Does It Mean that One Who Repents Should Be in Happiness? he: רב"ש => מאמרים => מהו שהאדם שב בתשובה צריך להיות בשמחה
M8KN7NIe en: Rabash => Articles => What Is Revealing a Portion and Covering Two Portions in the Work? he: רב"ש => מאמרים => מהו גילוי טפח וכיסוי טפחיים, בעבודה
rXnBHg90 en: Rabash => Articles => What Is, “If a Woman Inseminates First, She Delivers a Male Child,” in the Work? he: רב"ש => מאמרים => מהו אשה מזרעת תחילה יולדת זכר, בעבודה
4YtLj4Tr en: Rabash => Articles => What Are Holiness and Purity, in the Work? he: רב"ש => מאמרים => מהו קדושה וטהרה בעבודה
UjdK3jN1 en: Rabash => Articles => What Does It Mean that a High Priest Should Take a Virgin Wife, in the Work? he: רב"ש => מאמרים => מהו שכהן גדול יקח לאשה בתולה, בעבודה
pbz1cNf7 en: Rabash => Articles => What Does It Mean that One Who Was On a Far Off Way Is Postponed to a Second Passover, in the Work? he: רב"ש => מאמרים => מהו מי שהיה בדרך רחוקה, הוא נדחה לפסח שני, בעבודה
gxqpWCIK en: Rabash => Articles => What Does It Mean that Charity to the Poor Makes the Holy Name, in the Work? he: רב"ש => מאמרים => מהו שצדקה לעניים עושה השם הקדוש, בעבודה
A0zvWJqS en: Rabash => Articles => What Are Banners in the Work? he: רב"ש => מאמרים => מהו דגלים בעבודה
DKmX5iDi en: Rabash => Articles => What Does It Mean that the Creator Favors Someone, in the Work? he: רב"ש => מאמרים => מהו שהקב"ה נושא פנים, בעבודה
a9nTujnc en: Rabash => Articles => What Is Eating Their Fruits in This World and Keeping the Principal for the Next World, in the Work? he: רב"ש => מאמרים => מהו אוכל פירותיהם בעולם הזה והקרן קיימת לעולם הבא, בעבודה
Wi3Y7c4h en: Rabash => Articles => What Is the Meaning of “Spies,” in the Work? he: רב"ש => מאמרים => מהו ענין מרגלים, בעבודה
Jhp32x0t en: Rabash => Articles => What Is, “Peace, Peace, to the Far and to the Near,” in the Work? he: רב"ש => מאמרים => מהו שלום שלום לרחוק ולקרוב, בעבודה
GbaCghpG en: Rabash => Articles => What Is the “Torah” and What Is “The Statute of the Torah,” in the Work? he: רב"ש => מאמרים => מהו תורה ומהו חקת התורה, בעבודה
vMh5n5NP en: Rabash => Articles => What Is the “Right Line,” in the Work? he: רב"ש => מאמרים => מהו ענין קו ימין בעבודה
y7jL0WVH en: Rabash => Articles => What Does It Mean that the Right Must Be Greater than the Left, in the Work? he: רב"ש => מאמרים => מהו שהימין צריך להיות יותר גדול מהשמאל, בעבודה
Zo9qIKe0 en: Rabash => Articles => What Are Truth and Falsehood in the Work? he: רב"ש => מאמרים => מהם אמת ושקר, בעבודה
de0oATlW en: Rabash => Articles => What Should One Do If He Was Born With Bad Qualities? he: רב"ש => מאמרים => מהו על האדם לעשות, אם נברא במידות לא טובות
v6XiIQP8 en: Rabash => Articles => What Is, “An Ox Knows Its Owner, etc., Israel Does Not Know,” in the Work? he: רב"ש => מאמרים => מהו "ידע שור קנהו וכו' ישראל לא ידע", בעבודה
dyIfc5rT en: Rabash => Articles => What Is, “You Will See My Back, But My Face Shall Not Be Seen,” in the Work? he: רב"ש => מאמרים => מהו "וראית את אחורי ופני לא יראו", בעבודה
aWjz2LMt en: Rabash => Articles => What Is the Reason for which Israel Were Rewarded with Inheritance of the Land, in the Work? he: רב"ש => מאמרים => מהי הסיבה שבגללה זכו ישראל לירושת הארץ, בעבודה
OvixlOop en: Rabash => Articles => What Does It Mean that a Judge Must Judge Absolutely Truthfully, in the Work? he: רב"ש => מאמרים => מהו שהדיין צריך לדון דין אמת לאמיתו, בעבודה
4uLCIg6Q en: Rabash => Articles => What Is the Son of the Beloved and the Son of the Hated in the Work? he: רב"ש => מאמרים => מהו בן האהובה ובן השנואה, בעבודה
kJwTJmIk en: Rabash => Articles => What Does It Mean that the Right and the Left Are in Contrast, in the Work? he: רב"ש => מאמרים => מהו שהימין והשמאל הם בסתירה, בעבודה
CkyOE05Q en: Rabash => Concealment and Revelation he: רב"ש => מהסתר לגילוי
Q8331iRZ en: Rabash => Explanation of the Article, “Preface to the Wisdom of Kabbalah” he: רב"ש => ביאור לפתיחה לחכמת הקבלה
ml en: Michael Laitman he: מיכאל לייטמן
QUBP2DYe en: Michael Laitman => Summaries of articles and letters of Baal HaSulam he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם
o7fhKZaX en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Chronology of the Wisdom of Kabbalah (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. תולדות חכמת הקבלה - קיצור
T9OJwn6h en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Disclosing a Portion, Covering Two (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. גילוי טפח וכיסוי טפחיים - קיצור
tdqztaDr en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Exile and Redemption (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הגלות והגאולה - קיצור (1)
17ge0Z8x en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Exile and Redemption (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הגלות והגאולה - קיצור (2)
ruKvYwRz en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Exile and Redemption (modified) (3) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הגלות והגאולה - קיצור (3)
dPBBjjzT en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Exile and Redemption (modified) (4) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הגלות והגאולה - קיצור (4)
qO7BHtX2 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Foreword to the Book “The Tree of Life” (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הקדמה לספר "פנים מאירות ומסבירות" - קיצור
mA8Cz4iD en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Four Worlds (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. ארבעה עולמות - קיצור (1)
bDDtg5Mp en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Four Worlds (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. ארבעה עולמות - קיצור (2)
K5GQnB5a en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Freedom of Will (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חופש הבחירה - קיצור (1)
0sPJ7FRW en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Freedom of Will (modified)(2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חופש הבחירה - קיצור (2)
7scdAIuf en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. From the mouth of a Wise man (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הקדמה לספר "פי חכם" - קיצור (1)
Q3s8qfeF en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. From the mouth of a Wise man (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הקדמה לספר "פי חכם" - קיצור (2)
SOibuJuk en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. General preface (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. פתיחה כוללת - קיצור
q4owrrU5 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. I will know the creator from within myself (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. למצא את הבורא בתוכי - קיצור
r0vuBvyq en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Influence of the Creator (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מעשה הבורא באחור וקדם - קיצור (1)
CdMXYtao en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Influence of the Creator (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מעשה הבורא באחור וקדם - קיצור (2)
5SWiTsEm en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Inheritance of the Land (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. ירושת הארץ - קיצור
W4nhu4mZ en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Introduction to The Study of the Ten Sefirot (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הקדמה לתלמוד עשר הספירות - קיצור
gOxFzOLb en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Introduction to the Book of Zohar (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מבוא לספר הזוהר - קיצור
QH14QIWF en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Introduction to the Preface to the Wisdom of Kabbalah (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הקדמה לפתיחה לחכמת הקבלה - קיצור
0qZ6AAKc en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Kabbalah as Compared with Other Sciences (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוכמת הקבלה בהשוואה למדעים אחרים - קיצור
A078TvJh en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Kabbalah as a Root of All Sciences (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוכמת הקבלה – שורש כל המדעים - קיצור
LvnSISye en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Kabbalah as a modern teaching (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוכמת הקבלה כמדע מודרני - קיצור
z2BSKgTe en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Letter 13 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אגרת י"ג - קיצור
fGb9J7eW en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Letter 16 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אגרת ט"ז - קיצור
GyHEvVYc en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Letter 21 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אגרת כ"א - קיצור
tL5FeN6p en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Letter 47 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אגרת מ"ז - קיצור
28Rj1TfB en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Letter 49 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אגרת מ"ט - קיצור
BrCeoRlf en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Letter 5 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אגרת ה' - קיצור
Li2CkIC4 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Love for the Creator and Love for the Created Beings (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אהבת הבורא ואהבת הבריות - קיצור (1)
PPFobbbl en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Love for the Creator and Love for the Created Beings (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. אהבת הבורא ואהבת הבריות - קיצור (2)
okIikm9q en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Matan Torah (The Giving of the Torah) (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מתן תורה - קיצור (1)
ZAgkfwAf en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Matan Torah (The Giving of the Torah) (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מתן תורה - קיצור (2)
PbEd1FnL en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Matter and Form in Kabbalah (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. החומר והצורה בחוכמת הקבלה - קיצור (1)
CBsg6kNF en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Matter and Form in Kabbalah (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. החומר והצורה בחוכמת הקבלה - קיצור (2)
2p0Xosd9 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Newspaper "The Nation" (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. עיתון האומה - קיצור (1)
iHziyXMy en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Newspaper "The Nation" (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. עיתון האומה - קיצור (2)
v77bYIcm en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. One law (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוק אחד - קיצור (1)
XZY09Mh3 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. One law (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוק אחד - קיצור (2)
dzNJacTQ en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Peace in the World (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. השלום בעולם - קיצור (1)
TcJm3cCf en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Peace in the World (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. השלום בעולם - קיצור (2)
Cj8l3Gxl en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Peace in the World (modified) (3) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. השלום בעולם - קיצור (3)
5sLqsXjD en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Preface to the Sulam Commentary (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. פתיחה לפירוש הסולם - קיצור
pqsh4HGz en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 115. Still, Vegetative, Animate, and Speaking (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. קטו. ענין דומם, צומח, חי, מדבר - קיצור 
VRCdWdI0 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 164. There Is a Difference between Corporeality and Spirituality (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. קסד. יש הפרש בין גשמיות לרוחניות - קיצור 
cuFXlgfU en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 169. Concerning a Complete Righteous (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. קסט. ענין צדיק גמור - קיצור
3ybHEzdu en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 33. The Lots on Yom Kippurim and with Haman (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. לג. ענין גורלות, שהיה ביום כפורים, ואצל המן - קיצור
WGtjOrPm en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 67. Depart from Evil (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. סז. סור מרע - קיצור
Hlcb9v71 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 68. Man's Connection to the Sefirot (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. סח. קשר האדם אל הספירות - קיצור
ogAG7Q0h en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Shamati 74. World, Year, Soul (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שמעתי. עד. ענין עולם שנה נפש - קיצור
XbIg5x0b en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. TES, Part 1, Histaklut Pnimit (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. תע"ס, חלק א', הסתכלות פנימית - קיצור
6zlJ4KJH en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Arvut (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הערבות - קיצור
5mIViFqA en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Creating Mind (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שכל הפועל - קיצור (1)
Pr3PALvZ en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Creating Mind (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שכל הפועל - קיצור (2)
pfUClIAP en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Creating Mind (modified) (3) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שכל הפועל - קיצור (3)
lqu5lLM3 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Creator’s Concealment and Revelation - 1 (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. ההסתר והגילוי של הבורא - א' , קיצור
pRtEuRFu en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Difference between the Science of Kabbalah and Religion (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. ההבדל בין דת וקבלה - קיצור
xGEqG8l6 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Essence of Religion and Its Purpose (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מהות הדת ומטרתה - קיצור
mkdyfmRE en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Essence of the Science of Kabbalah (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מהות חוכמת הקבלה - קיצור (1)
Pw95dXgi en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Essence of the Science of Kabbalah (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מהות חוכמת הקבלה - קיצור (2)
IILb9nI3 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Giving of the Torah. The Arvut (modified) (3) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מתן תורה (קבלה). ערבות - קיצור (3)
5Naay6Qx en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Last Generation (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. דור האחרון - קיצור (1)
Y9HaAsfl en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Last Generation (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. דור האחרון - קיצור (2)
2l4x9Pfd en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Meaning of the Chaf in Anochi (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. סוד הכף דאנכי - קיצור
epCZmQxJ en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Peace (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. השלום - קיצור (1)
PTdznYAF en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Peace (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. השלום - קיצור (2)
iqfmpEoY en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Peace (modified) (3) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. השלום - קיצור (3)
uoYiTKcl en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Prophecy (modified)  he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. הנבואה - קיצור
kbfL4cw1 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Purpose of Kabbalah (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מטרת חכמת הקבלה - קיצור
2Gt8cOmK en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Quality of the Wisdom of the Hidden in General (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. תכונתה של חכמת הנסתר בכללה - קיצור (2)
aZUc1sjV en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Teaching of the Kabbalah and Its Essence (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. תורת הקבלה ומהותה  -קיצור (1)
UTAhWr5D en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Teaching of the Kabbalah and Its Essence (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. תורת הקבלה ומהותה - קיצור (2)
8EXw2FvT en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The Wisdom of Israel Compared to External Wisdoms (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חכמת ישראל בערך חכמת חיצוניים - קיצור
WktRd9ov en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. The language of Kabbalah (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. שפת הקבלה - קיצור
PAkoVTQ4 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. This Is for Judah (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. וזאת ליהודה - קיצור (1)
4na34Qqr en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. This Is for Judah (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. וזאת ליהודה - קיצור (2)
z78YDEnL en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Time to Act (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. עת לעשות - קיצור (1)
xFDUivIY en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Time to Act (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. עת לעשות - קיצור (2)
8b9iDw8l en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Wisdom of Kabbalah and Philosophy (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוכמת הקבלה והפילוסופיה - קיצור (1)
BE4FPwaw en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Wisdom of Kabbalah and Philosophy (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. חוכמת הקבלה והפילוסופיה - קיצור (2)
D2arfqUC en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Michael Laitman. Attainment of Unity of the Universe he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => מיכאל לייטמן. השגת ייחוד הבריאה
kEx5Zt30 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Michael Laitman. How to achieve happiness ? he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => מיכאל לייטמן. איך להגיע לאושר?
bBhVbpvh en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Michael Laitman. To perceive the greatness of nature he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => מיכאל לייטמן. להשיג גדלות הטבע
XmZdDMdS en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Summaries of Psalms (rus) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => תהילים - קיצור (רוסית)
ar1mpQ2Z en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam.  From My Flesh I Shall See God (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מבשרי אחזה אלוקי - קיצור
ZXc97oMl en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. 600,000 Souls (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. ששים רבוא נשמות - קיצור
kGMPVgxh en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. All who feel the affliction of the public (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. כל המצטער עם הצבור - קיצור
pISxqKIE en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. A Speech for the Completion of The Zohar (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מאמר לסיום הזוהר - קיצור
ceKVN7Lx en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Body and Soul (modified) (2) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. גוף ונשמה - קיצור (2)
tvMN493F en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Body and Soul (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. גוף ונשמה - קיצור (1)
ZFU2faHd en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Characteristics of the Wisdom of Kabbalah (modified) (1) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. תכונותיה של חכמת הקבלה - קיצור (1)
ivMG9L03 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Baal HaSulam. Conditions of Revealing the Kabbalistic Knowledge (modified) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. התנאים לפרסום הידיעות שבחוכמת הקבלה - קיצור
uspRWF11 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => Summaries of Psalms for Twitter (rus) he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => תהילים - קיצור עבור טוויטר (רוסית)
6tr0R6F5 en: Michael Laitman => Summaries of articles and letters of Baal HaSulam => בעל הסולם. מבוא לפתיחה לחכמת הקבלה - קיצור he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => בעל הסולם. מבוא לפתיחה לחכמת הקבלה - קיצור
fTogIfvk en: Michael Laitman => Summaries of articles and letters of Baal HaSulam =>  he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => 
qKHcRRxu en: Michael Laitman => Summaries of articles and letters of Baal HaSulam =>  he: מיכאל לייטמן => קיצורי מאמרים ומכתבים של בעל הסולם => 
8Y0f8Jg9 en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית)
z1UFx4Jr en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Body and Soul he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => גוף ונפש
egMr1434 en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Concealment and Disclosure of the Face of the Creator - 1 he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => הסתר וגילוי פנים של השי"ת - א'
diDYqvQY en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Disclosing a Portion, Covering Two he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => גילוי טפח וכיסוי טפחיים
gShbOWPg en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Foreword to The Book of Zohar he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => מבוא לספר הזוהר
2CDaeeY0 en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Four Worlds he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => ד' עולמות
P8q5PngO en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => From My Flesh I Shall See God he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => מבשרי אחזה אלוקי
ao0JdRrd en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Introduction to “From the Mouth of a Sage” he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => הקדמה לספר פי חכם
pYYjUYph en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Introduction to The Book of Zohar he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => הקדמה לספר הזוהר
m8pIk8KI en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Introduction to the Book Panim Meirot uMasbirot he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => הקדמה לספר פנים מאירות ומסבירות
sPVGAuFW en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Introduction to The Study of the Ten Sefirot he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => הקדמה לתע"ס
WvjHd3j6 en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Matter and Form in the Wisdom of Kabbalah he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => החומר והצורה בחכמת הקבלה
9CWuSu3J en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => One Commandment he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => מצווה אחת
VF8871pP en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Peace in the World he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => השלום בעולם
rwNRf2tI en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Study of the Ten Sefirot, Part 1, Histaklut Pnimit he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => תלמוד עשר הספירות, חלק א', הסתכלות פנימית
i4QIXsjy en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Acting Mind he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => שכל הפועל
a5pgqF2K en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Essence of Religion and Its Purpose he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => מהות הדת ומטרתה
BMqag56q en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Essence of the Wisdom of Kabbalah he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => מהותה של חכמת הקבלה
Vii0BWAp en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Freedom he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => מאמר החירות
G5MMKq2W en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Love of God and the Love of Man he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => אהבת ה' ואהבת הבריות
qEcXTJ41 en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Peace he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => השלום
r5o7oIvy en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Quality of the Wisdom of the Hidden in General he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => תכונתה של חכמת הנסתר בכללה
5PmcvuhK en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Teaching of the Kabbalah and Its Essence he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => תורת הקבלה ומהותה
XcaJIQWv en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Wisdom of Kabbalah and Philosophy he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => חכמת הקבלה והפילוסופיה
TsFwW2Xu en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => The Wisdom of Kabbalah Compared to Other Sciences he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => חכמת ישראל בערך חכמת חיצוניים
aZRQeeqg en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => Time to Act he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => עת לעשות
fSDmF0Cr en: Michael Laitman => The Writings of Baal HaSulam, Campus (rus) => You Have Made Me in Behind and Before he: מיכאל לייטמן => כתבי בעל הסולם, קמפוס (רוסית) => אחור וקדם צרתני
FZhiWkph en: Michael Laitman => Work with Faith Above Reason (rus) he: מיכאל לייטמן => ספר העבודה באמונה למעלה מהדעת (רוסית)
tKJ0rhmH en: Michael Laitman => Work with Faith Above Reason (rus) => Preparation to the Lesson he: מיכאל לייטמן => ספר העבודה באמונה למעלה מהדעת (רוסית) => הכנה לשיעור
1lskO2mH en: Michael Laitman => Work with Faith Above Reason (rus) => Selected excerpts from the sources he: מיכאל לייטמן => ספר העבודה באמונה למעלה מהדעת (רוסית) => קטעים נבחרים מהמקורות
LPk2bK2d en: Michael Laitman => Summaries of Torah chapters (rus) he: מיכאל לייטמן => קיצורי פרקים מהתורה (רוסית)
rsC6qkVs en: Michael Laitman => Summaries of Articles of the Introduction of The Book of Zohar (rus) he: מיכאל לייטמן => קיצורי מאמרים מהקדמת ספר הזוהר (רוסית)
N4FJ3mD4 en: Michael Laitman => Lecture in Arosa - Brochure he: מיכאל לייטמן => מסמך ארוסה
GIaC4QP3 en: Michael Laitman => Lecture in Dusseldorf - Brochure he: מיכאל לייטמן => מסמך דיסלדורף
mVOa4vZh en: Michael Laitman => Summaries of Psalms (rus) he: מיכאל לייטמן => תהילים - קיצור (רוסית)
TdxXtcqf en: Michael Laitman => Summaries of Psalms for Twitter (rus) he: מיכאל לייטמן => תהילים - קיצור עבור טוויטר (רוסית)
AzFpCn7D en: Michael Laitman => Revealing The Creator (rus) he: מיכאל לייטמן => ספר גילוי הבורא (רוסית)
oQAUBJad en: Michael Laitman => Revealing The Creator (rus) => Внутреннее устремление в изучении каббалы he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Внутреннее устремление в изучении каббалы
iUHlpPbN en: Michael Laitman => Revealing The Creator (rus) => Общая картина творения he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Общая картина творения
GRoE6jd6 en: Michael Laitman => Revealing The Creator (rus) => Десять светов he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Десять светов
ZCNpiCOA en: Michael Laitman => Revealing The Creator (rus) => Внутреннее движение he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Внутреннее движение
yiLwkjU1 en: Michael Laitman => Revealing The Creator (rus) => История каббалы he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => История каббалы
bNqBVWyc en: Michael Laitman => Revealing The Creator (rus) => Путешествие в душу человека he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Путешествие в душу человека
ThmZxJ86 en: Michael Laitman => Revealing The Creator (rus) => Каббалистический театр he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Каббалистический театр
S8cqI6aW en: Michael Laitman => Revealing The Creator (rus) => Гость и Хозяин. Хозяин и гость he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Гость и Хозяин. Хозяин и гость
Bk7kdjQ7 en: Michael Laitman => Revealing The Creator (rus) => Рамхаль. 138 ворот мудрости he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Рамхаль. 138 ворот мудрости
fJWVBgNr en: Michael Laitman => Revealing The Creator (rus) => Высший и низший he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Высший и низший
039nkpk4 en: Michael Laitman => Revealing The Creator (rus) => Необходимость альтруистического намерения he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Необходимость альтруистического намерения
tQHObBcN en: Michael Laitman => Revealing The Creator (rus) => Что такое праздник he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Что такое праздник
zoQhsfqs en: Michael Laitman => Revealing The Creator (rus) => Сфират аОмер he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Сфират аОмер
JOC7bqHi en: Michael Laitman => Revealing The Creator (rus) => Вопросы и ответы he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Вопросы и ответы
hWkGcnFm en: Michael Laitman => Revealing The Creator (rus) => Высшее наслаждение he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Высшее наслаждение
SktTX2DM en: Michael Laitman => Revealing The Creator (rus) => О душе и теле he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => О душе и теле
3YZ83PMx en: Michael Laitman => Revealing The Creator (rus) => Отрывки из книги "Йошер диврей эмет" he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Отрывки из книги "Йошер диврей эмет"
SqrTsHYq en: Michael Laitman => Revealing The Creator (rus) => Молитва, предваряющая молитву he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Молитва, предваряющая молитву
oPT4L1Kf en: Michael Laitman => Revealing The Creator (rus) => Передача знаний he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Передача знаний
lBH1A07l en: Michael Laitman => Revealing The Creator (rus) => Первые ощущения в духовном пути he: מיכאל לייטמן => ספר גילוי הבורא (רוסית) => Первые ощущения в духовном пути
mr en: Moses he: משה
bvA8ZB1w en: Moses => Torah he: משה => תורה
74ThCir9 en: Moses => Torah => Bereshit he: משה => תורה => בראשית
mmnCkYgm en: Moses => Torah => Bereshit => Bereshit he: משה => תורה => בראשית => בראשית
EtjFb5Ws en: Moses => Torah => Bereshit => Noach he: משה => תורה => בראשית => נוח
mxAWeilL en: Moses => Torah => Bereshit => Lech Lecha he: משה => תורה => בראשית => לך לך
EY97KSCU en: Moses => Torah => Bereshit => VaYera he: משה => תורה => בראשית => ויירא
qlP0W6JB en: Moses => Torah => Bereshit => Chayei Sarah he: משה => תורה => בראשית => חיי שרה
At3s85iS en: Moses => Torah => Bereshit => Toldot he: משה => תורה => בראשית => תולדות
4Av053ug en: Moses => Torah => Bereshit => VaYetze he: משה => תורה => בראשית => ויצא
k8OGIDxO en: Moses => Torah => Bereshit => VaYishlach he: משה => תורה => בראשית => וישלח
VElajoyj en: Moses => Torah => Bereshit => VaYeshev he: משה => תורה => בראשית => וישב
BYG5EEkY en: Moses => Torah => Bereshit => Miketz he: משה => תורה => בראשית => מקץ
4OoYZYM2 en: Moses => Torah => Bereshit => VaYigash he: משה => תורה => בראשית => ויגש
uc6gTQmE en: Moses => Torah => Bereshit => VaYechi he: משה => תורה => בראשית => ויחי
zk97UPnh en: Moses => Torah => Shmot he: משה => תורה => שמות
grgJx7av en: Moses => Torah => Shmot => Shmot he: משה => תורה => שמות => שמות
SzC09yex en: Moses => Torah => Shmot => VaEra he: משה => תורה => שמות => וארא
e9V6VzYs en: Moses => Torah => Shmot => Bo he: משה => תורה => שמות => בוא
Vm0dtMhH en: Moses => Torah => Shmot => BeShalach he: משה => תורה => שמות => בשלח
vBtZcsIr en: Moses => Torah => Shmot => Itro he: משה => תורה => שמות => יתרו
ejo7uR8r en: Moses => Torah => Shmot => Mishpatim he: משה => תורה => שמות => משפטים
TbCXY76B en: Moses => Torah => Shmot => Truma he: משה => תורה => שמות => תרומה
hBVco9vg en: Moses => Torah => Shmot => Tetzave he: משה => תורה => שמות => תצווה
JBD22SZw en: Moses => Torah => Shmot => Ki Tisa he: משה => תורה => שמות => כי תשא
04musv2K en: Moses => Torah => Shmot => VaYakhel he: משה => תורה => שמות => ויקהל
Uz1D97ZE en: Moses => Torah => Shmot => Pkudei he: משה => תורה => שמות => פקודי
zHPhXQ7a en: Moses => Torah => VaYikra he: משה => תורה => ויקרא
qwpyjfYN en: Moses => Torah => VaYikra => VaYikra he: משה => תורה => ויקרא => ויקרא
fExvW21e en: Moses => Torah => VaYikra => Tzav he: משה => תורה => ויקרא => צו
JtJSYMWy en: Moses => Torah => VaYikra => Shmini he: משה => תורה => ויקרא => שמיני
gZamJsCQ en: Moses => Torah => VaYikra => Tazria he: משה => תורה => ויקרא => תזריע
F0TfZx4v en: Moses => Torah => VaYikra => Metzora he: משה => תורה => ויקרא => מצורע
wkZGcvcr en: Moses => Torah => VaYikra => Acharei Mot he: משה => תורה => ויקרא => אחרי מות
Qj3LypVs en: Moses => Torah => VaYikra => Kdoshim he: משה => תורה => ויקרא => קדושים
6XT37CU8 en: Moses => Torah => VaYikra => Emor he: משה => תורה => ויקרא => אמור
DEOA93sV en: Moses => Torah => VaYikra => BaHar he: משה => תורה => ויקרא => בהר
6p9i2oLI en: Moses => Torah => VaYikra => BeHukotai he: משה => תורה => ויקרא => בחוקתי
w37Ou4LB en: Moses => Torah => Bamidbar he: משה => תורה => במדבר
rfqp5V8I en: Moses => Torah => Bamidbar => Bamidbar he: משה => תורה => במדבר => במדבר
kiLdQe8g en: Moses => Torah => Bamidbar => Naso he: משה => תורה => במדבר => נשוא
CRZFtL9o en: Moses => Torah => Bamidbar => BeHaalotcha he: משה => תורה => במדבר => בהעלותך
7smJ1Uzr en: Moses => Torah => Bamidbar => Shlach Lecha he: משה => תורה => במדבר => שלח לך
Raan2pVq en: Moses => Torah => Bamidbar => Korach he: משה => תורה => במדבר => קורח
FtIGggbC en: Moses => Torah => Bamidbar => Chukat he: משה => תורה => במדבר => חוקת
x7KSdDwM en: Moses => Torah => Bamidbar => Balak he: משה => תורה => במדבר => בלק
R6YnMZZW en: Moses => Torah => Bamidbar => Pinchas he: משה => תורה => במדבר => פנחס
5WhzwqRc en: Moses => Torah => Bamidbar => Matot he: משה => תורה => במדבר => מטות
670UzT2C en: Moses => Torah => Bamidbar => Masei he: משה => תורה => במדבר => מסעי
jxKFqIIl en: Moses => Torah => Dvarim he: משה => תורה => דברים
yjvHgSdc en: Moses => Torah => Dvarim => Dvarim he: משה => תורה => דברים => דברים
WjTITKNI en: Moses => Torah => Dvarim => VaEtchanan he: משה => תורה => דברים => ואתחנן
nfrYkE1S en: Moses => Torah => Dvarim => Ekev he: משה => תורה => דברים => עקב
6Uko897F en: Moses => Torah => Dvarim => Reeh he: משה => תורה => דברים => ראה
DtqWso4L en: Moses => Torah => Dvarim => Shoftim he: משה => תורה => דברים => שופטים
4DLwhTBq en: Moses => Torah => Dvarim => Ki Tetze he: משה => תורה => דברים => כי תצא
MvOwVPAh en: Moses => Torah => Dvarim => Ki Tavo he: משה => תורה => דברים => כי תבוא
RAw4LZ2I en: Moses => Torah => Dvarim => Nitzavim he: משה => תורה => דברים => נצבים
V13YHvcK en: Moses => Torah => Dvarim => VaYelech he: משה => תורה => דברים => וילך
xwbiQixS en: Moses => Torah => Dvarim => Haazinu he: משה => תורה => דברים => האזינו
1y9X7QSk en: Moses => Torah => Dvarim => Ve Zot HaBracha he: משה => תורה => דברים => וזאת הברכה
rh en: Rashbi he: רשב"י
ar en: Ari he: אר"י
UncaTFGA en: Ari => The Tree of Life - a Poem he: אר"י => עץ החיים - שיר
OtAMovN9 en: Ari => Gate of Intentions he: אר"י => שער הכוונות
HujVytxA en: Ari => Etz Chaim (Tree of Life) he: אר"י => עץ חיים
CqvEYMsd en: Ari => Gate of Intentions - for Pesach (heb) he: אר"י => שער הכוונות לפסח - קיצור
mcJIt2Vv en: Ari => Gate of Intentions - for Pesach (heb) => Gate of Intentions - for Pesach he: אר"י => שער הכוונות לפסח - קיצור => דרוש א - בענין פסח ויציאת מצרים
9wq8A7IY en: Ari => Gate of Intentions - for Pesach (heb) => Gate of Intentions - for Pesach he: אר"י => שער הכוונות לפסח - קיצור => דרוש ב - תוקף הגדלת ז"א ביציאת מצרים
KTyag9Dn en: Ari => Gate of Intentions - for Pesach (heb) => Gate of Intentions - for Pesach he: אר"י => שער הכוונות לפסח - קיצור => דרוש ג - יציאת מצרים ופסח
o9ilhNJa en: Ari => Gate of Intentions - for Pesach (heb) => Gate of Intentions - for Pesach he: אר"י => שער הכוונות לפסח - קיצור => דרוש ד - החמץ והשאור
q5ugY2k5 en: Ari => Gate of Intentions - for Pesach (heb) => Gate of Intentions - for Pesach he: אר"י => שער הכוונות לפסח - קיצור => דרוש ה - גלות מצרים ופרעה
ybIqbwK6 en: Ari => Gate of Intentions - for Pesach (heb) => Gate of Intentions - for Pesach he: אר"י => שער הכוונות לפסח - קיצור => דרוש ו - סדר ליל פסח
rl en: Ramchal he: רמח"ל
ag en: Agra he: הגר"א
vk en: Various he: שונות
mmh3yJa9 en: Various => Moshe Butril he: שונות => משה בוטריל
J6U0kWO8 en: Various => Moshe Butril => Preface by Rabbi Moshe Butryl to the Book of Yetzira he: שונות => משה בוטריל => הקדמת הרב משה בוטריל לספר יצירה
nyQ2Vs4N en: Various => Maor VaShemesh he: שונות => מאור ושמש
VQgwvH3M en: Various => Maor VaShemesh => Maor VaShemesh he: שונות => מאור ושמש => מאור ושמש
3i1ektDL en: Various => Degel Machne Ephraim he: שונות => דגל מחנה אפרים
lHdkvAp4 en: Various => Degel Machne Ephraim => Degel Machne Ephraim he: שונות => דגל מחנה אפרים => דגל מחנה אפרים
4yExZMtm en: Various => Yosher Divrei Emet he: שונות => יושר דברי אמת
wWm6fbn4 en: Various => Connecting to the Source he: שונות => מתחברים אל המקור
WsQeihV8 en: Various => Connecting to the Source => All the Wisdoms Are Included in the Wisdom of Kabbalah he: שונות => מתחברים אל המקור => כל החכמות בעולם כלולות בחכמת הקבלה
hXgdf0Je en: Various => Connecting to the Source => Annulment and Submission he: שונות => מתחברים אל המקור => ביטול והכנעה
ICpIlFnV en: Various => Connecting to the Source => A Prayer of Many he: שונות => מתחברים אל המקור => תפילת רבים
YtJx4Tii en: Various => Connecting to the Source => Arvut he: שונות => מתחברים אל המקור => ערבות
UAYrbs62 en: Various => Connecting to the Source => Ascents and Descents he: שונות => מתחברים אל המקור => עליות וירידות
6LXJtigR en: Various => Connecting to the Source => Burdening of the Heart he: שונות => מתחברים אל המקור => הכבדת הלב
PjOAREY1 en: Various => Connecting to the Source => Buy for Yourself a Friend he: שונות => מתחברים אל המקור => קנה לך חבר
afbx7Dym en: Various => Connecting to the Source => By Your Actions, We Know You he: שונות => מתחברים אל המקור => ממעשיך היכרנוך
hw2BkqhM en: Various => Connecting to the Source => Choosing the Environment he: שונות => מתחברים אל המקור => הבחירה בסביבה
H77cTYz5 en: Various => Connecting to the Source => Concealment and Revelation he: שונות => מתחברים אל המקור => הסתר וגילוי
RLmjPXqj en: Various => Connecting to the Source => Despair with One’s Own Strength he: שונות => מתחברים אל המקור => ייאוש מכוחותיו עצמו
hb69fLv8 en: Various => Connecting to the Source => Devotion he: שונות => מתחברים אל המקור => מסירות נפש
lCx8ei5T en: Various => Connecting to the Source => Dvekut he: שונות => מתחברים אל המקור => דביקות
61VA3jOL en: Various => Connecting to the Source => Envy, Lust, and Honor he: שונות => מתחברים אל המקור => קנאה, תאווה וכבוד
2WwnhIpw en: Various => Connecting to the Source => Faith Above Reason he: שונות => מתחברים אל המקור => אמונה למעלה מהדעת
1yKOb8mX en: Various => Connecting to the Source => Fear he: שונות => מתחברים אל המקור => יראה
kKIdfaLe en: Various => Connecting to the Source => From Lo Lishma to Lishma he: שונות => מתחברים אל המקור => משלא לשמה באים לשמה
zuf8BvVW en: Various => Connecting to the Source => From the Love of People to the Love of the Creator he: שונות => מתחברים אל המקור => מאהבת הבריות לאהבת ה'
lZuqpN9P en: Various => Connecting to the Source => Giving Contentment to the Creator he: שונות => מתחברים אל המקור => נחת רוח לבורא
zGxXOfBr en: Various => Connecting to the Source => Gratitude he: שונות => מתחברים אל המקור => הודיה
SkHABzg3 en: Various => Connecting to the Source => Hitkalelut he: שונות => מתחברים אל המקור => התכללות
O4j3aaKW en: Various => Connecting to the Source => Ibur he: שונות => מתחברים אל המקור => עיבור
ZhHPsWqR en: Various => Connecting to the Source => If I am Not for Me, Who Is for Me? he: שונות => מתחברים אל המקור => אם אין אני לי, מי לי
wZpyPHm7 en: Various => Connecting to the Source => Intention he: שונות => מתחברים אל המקור => כוונה
DFHY3DWN en: Various => Connecting to the Source => Israel and the Nations of the World he: שונות => מתחברים אל המקור => ישראל ואומות העולם
9ENbYKeA en: Various => Connecting to the Source => Joy on the Path he: שונות => מתחברים אל המקור => שמחה בדרך
W2tTcBtc en: Various => Connecting to the Source => Judge Every Person to the Side of Merit he: שונות => מתחברים אל המקור => והווי דן את כל האדם לכף זכות
LJx5XRtd en: Various => Connecting to the Source => Kabbalists and the Writings of Kabbalah he: שונות => מתחברים אל המקור => המקובלים וכתבי הקבלה
rU6ft4ZW en: Various => Connecting to the Source => Labor he: שונות => מתחברים אל המקור => יגיעה
Dd2YHReP en: Various => Connecting to the Source => Love of Friends he: שונות => מתחברים אל המקור => אהבת חברים
WhU3ZrbE en: Various => Connecting to the Source => Love Will Cover All Crimes he: שונות => מתחברים אל המקור => על כל פשעים תכסה אהבה
PXaqejvJ en: Various => Connecting to the Source => Make for Yourself a Rav he: שונות => מתחברים אל המקור => עשה לך רב
nQfo1MNS en: Various => Connecting to the Source => Our Generation – The Last Generation he: שונות => מתחברים אל המקור => דורנו - הדור האחרון
GMQGu4PH en: Various => Connecting to the Source => Overcoming he: שונות => מתחברים אל המקור => התגברות
2r1rN4kX en: Various => Connecting to the Source => Prayer he: שונות => מתחברים אל המקור => תפילה
O1QnO4re en: Various => Connecting to the Source => Preparation for Learning he: שונות => מתחברים אל המקור => הכנה ללימוד
9WtY6ARU en: Various => Connecting to the Source => Recognition of Evil he: שונות => מתחברים אל המקור => הכרת הרע
yCOxSMTS en: Various => Connecting to the Source => Seeing One’s Friend’s Merits he: שונות => מתחברים אל המקור => לראות מעלת חבירו
C2VfTz9Q en: Various => Connecting to the Source => Shame he: שונות => מתחברים אל המקור => בושה
vMcdZ1OR en: Various => Connecting to the Source => The Agenda of the Assembly he: שונות => מתחברים אל המקור => סדר ישיבת החברים
tgvxxbEW en: Various => Connecting to the Source => The Center of the Group he: שונות => מתחברים אל המקור => מרכז הקבוצה
r74bLFNj en: Various => Connecting to the Source => The Covenant he: שונות => מתחברים אל המקור => ברית
tdV3SZnY en: Various => Connecting to the Source => The Essence of Man he: שונות => מתחברים אל המקור => מהות האדם
7YipXjZe en: Various => Connecting to the Source => The Greatness of the Creator he: שונות => מתחברים אל המקור => גדלות הבורא
1NxJ60lo en: Various => Connecting to the Source => The Importance of Disseminating the Wisdom of Kabbalah he: שונות => מתחברים אל המקור => חשיבות הפצת חכמת הקבלה
weo1JV2k en: Various => Connecting to the Source => The Importance of the Goal he: שונות => מתחברים אל המקור => חשיבות המטרה
oQiMx1rl en: Various => Connecting to the Source => The Influence of the Environment on a Person he: שונות => מתחברים אל המקור => השפעת הסביבה על האדם
D94AnHso en: Various => Connecting to the Source => The Language of Kabbalists Is a Language of Branches he: שונות => מתחברים אל המקור => שפת המקובלים היא שפה של ענפים
ot3nZxVL en: Various => Connecting to the Source => The Merit of Learning – The Reforming Light he: שונות => מתחברים אל המקור => הסגולה בלימוד - המאור המחזיר למוטב
Hx7RqSma en: Various => Connecting to the Source => The Necessity to Learn the Wisdom of Kabbalah he: שונות => מתחברים אל המקור => החיוב בלימוד חכמת הקבלה
VTQ5T6fo en: Various => Connecting to the Source => The Origin of Resistance to the Wisdom of Kabbalah he: שונות => מתחברים אל המקור => מקור ההתנגדות לחכמת הקבלה
NnVqeax7 en: Various => Connecting to the Source => The Path of Torah and the Path of Suffering he: שונות => מתחברים אל המקור => דרך תורה ודרך ייסורים
pmC1K8IF en: Various => Connecting to the Source => The Perception of Reality he: שונות => מתחברים אל המקור => תפיסת המציאות
GJAVjA98 en: Various => Connecting to the Source =>  The Point in the Heart he: שונות => מתחברים אל המקור => התעוררות הנקודה שבלב
qaSNDUNt en: Various => Connecting to the Source => The Power in Connection he: שונות => מתחברים אל המקור => הכוח שבחיבור
l7PfQvJg en: Various => Connecting to the Source => The Preparation Period he: שונות => מתחברים אל המקור => זמן ההכנה
aYoGbLmm en: Various => Connecting to the Source =>  The Purpose of Creation he: שונות => מתחברים אל המקור => מטרת הבריאה
RK4EN1hf en: Various => Connecting to the Source => The Purpose of Society he: שונות => מתחברים אל המקור => מטרת החברה
oDKmefGv en: Various => Connecting to the Source => There Is None Else Besides Him he: שונות => מתחברים אל המקור => אין עוד מלבדו
Fp0LVa87 en: Various => Connecting to the Source => The Role of Israel he: שונות => מתחברים אל המקור => תפקיד ישראל
UVg2W09d en: Various => Connecting to the Source => The Society of the Last Generation he: שונות => מתחברים אל המקור => החברה בדור האחרון
aHWC4QRK en: Various => Connecting to the Source => The Soul of Adam HaRishon he: שונות => מתחברים אל המקור => נשמת אדם הראשון
Thn8lUqg en: Various => Connecting to the Source => The Ten he: שונות => מתחברים אל המקור => עשירייה
yCirB3y6 en: Various => Connecting to the Source => The Thought of Creation he: שונות => מתחברים אל המקור => מחשבת הבריאה
Rv5x5cnH en: Various => Connecting to the Source => They Helped Every Man His Friend he: שונות => מתחברים אל המקור => איש את רעהו יעזורו
4XHIHixV en: Various => Connecting to the Source => Two Opposites in One Subject he: שונות => מתחברים אל המקור => שני הפכים בנושא אחד
qWVYZKXG en: Various => Connecting to the Source => What Is the Wisdom of Kabbalah About? he: שונות => מתחברים אל המקור => במה עוסקת חכמת הקבלה
N1aDLNHV en: Various => Connecting to the Source => Who Is a Kabbalist he: שונות => מתחברים אל המקור => מיהו המקובל?
qB339E21 en: Various => Connecting to the Source => Yearning he: שונות => מתחברים אל המקור => השתוקקות
8IXkEYPv en: Various => A Prayer before a Prayer he: שונות => תפילה קודם תפילה
NlESBGzL en: Various => Psalms - Tehilim he: שונות => תהילים
ZUAiL2wT en: Various => Psalms - Tehilim => תהילים פרק א he: שונות => תהילים => תהילים פרק א
VJD4OYFj en: Various => Psalms - Tehilim => תהילים פרק ב he: שונות => תהילים => תהילים פרק ב
HKq9xh5v en: Various => Psalms - Tehilim => תהילים פרק ג he: שונות => תהילים => תהילים פרק ג
15ast2yi en: Various => Psalms - Tehilim => תהילים פרק ד he: שונות => תהילים => תהילים פרק ד
wkNqybJw en: Various => Psalms - Tehilim => תהילים פרק ה he: שונות => תהילים => תהילים פרק ה
TSzRIzHH en: Various => Psalms - Tehilim => תהילים פרק ו he: שונות => תהילים => תהילים פרק ו
52UfIOL9 en: Various => Psalms - Tehilim => תהילים פרק ז he: שונות => תהילים => תהילים פרק ז
T41IvzBx en: Various => Psalms - Tehilim => תהילים פרק ח he: שונות => תהילים => תהילים פרק ח
gEfJ1mvW en: Various => Psalms - Tehilim => תהילים פרק ט he: שונות => תהילים => תהילים פרק ט
4Hcwume4 en: Various => Psalms - Tehilim => תהילים פרק י he: שונות => תהילים => תהילים פרק י
rv2zvmi4 en: Various => Psalms - Tehilim => תהילים פרק יא he: שונות => תהילים => תהילים פרק יא
l8GWBDHI en: Various => Psalms - Tehilim => תהילים פרק יב he: שונות => תהילים => תהילים פרק יב
Tr3G49Ip en: Various => Psalms - Tehilim => תהילים פרק יג he: שונות => תהילים => תהילים פרק יג
YJmBenb9 en: Various => Psalms - Tehilim => תהילים פרק יד he: שונות => תהילים => תהילים פרק יד
IQtp599J en: Various => Psalms - Tehilim => תהילים פרק טו he: שונות => תהילים => תהילים פרק טו
Pl1rGpw9 en: Various => Psalms - Tehilim => תהילים פרק טז he: שונות => תהילים => תהילים פרק טז
vcljfNa2 en: Various => Psalms - Tehilim => תהילים פרק יז he: שונות => תהילים => תהילים פרק יז
bx56fVB5 en: Various => Psalms - Tehilim => תהילים פרק יח he: שונות => תהילים => תהילים פרק יח
a3mkiLL1 en: Various => Psalms - Tehilim => תהילים פרק יט he: שונות => תהילים => תהילים פרק יט
tkqkMVVN en: Various => Psalms - Tehilim => תהילים פרק כ he: שונות => תהילים => תהילים פרק כ
qX73FLcR en: Various => Psalms - Tehilim => תהילים פרק כא he: שונות => תהילים => תהילים פרק כא
KQNNUSgW en: Various => Psalms - Tehilim => תהילים פרק כב he: שונות => תהילים => תהילים פרק כב
hFlw0IEY en: Various => Psalms - Tehilim => תהילים פרק כג he: שונות => תהילים => תהילים פרק כג
4VxmOGEX en: Various => Psalms - Tehilim => תהילים פרק כד he: שונות => תהילים => תהילים פרק כד
EEYs7MKJ en: Various => Psalms - Tehilim => תהילים פרק כה he: שונות => תהילים => תהילים פרק כה
knweWfuj en: Various => Psalms - Tehilim => תהילים פרק כו he: שונות => תהילים => תהילים פרק כו
8nvLc82H en: Various => Psalms - Tehilim => תהילים פרק כז he: שונות => תהילים => תהילים פרק כז
aH3x3e1a en: Various => Psalms - Tehilim => תהילים פרק כח he: שונות => תהילים => תהילים פרק כח
uZ4HGdGZ en: Various => Psalms - Tehilim => תהילים פרק כט he: שונות => תהילים => תהילים פרק כט
Q6b1RUDc en: Various => Psalms - Tehilim => תהילים פרק ל he: שונות => תהילים => תהילים פרק ל
BMlBFtJh en: Various => Psalms - Tehilim => תהילים פרק לא he: שונות => תהילים => תהילים פרק לא
0APKQfX1 en: Various => Psalms - Tehilim => תהילים פרק לב he: שונות => תהילים => תהילים פרק לב
r2luLPI8 en: Various => Psalms - Tehilim => תהילים פרק לג he: שונות => תהילים => תהילים פרק לג
V55txi19 en: Various => Psalms - Tehilim => תהילים פרק לד he: שונות => תהילים => תהילים פרק לד
BfsD20nz en: Various => Psalms - Tehilim => תהילים פרק לה he: שונות => תהילים => תהילים פרק לה
tV5QO6EW en: Various => Psalms - Tehilim => תהילים פרק לו he: שונות => תהילים => תהילים פרק לו
IXrY4x4F en: Various => Psalms - Tehilim => תהילים פרק לז he: שונות => תהילים => תהילים פרק לז
Oowjfc00 en: Various => Psalms - Tehilim => תהילים פרק לח he: שונות => תהילים => תהילים פרק לח
aeULvg0x en: Various => Psalms - Tehilim => תהילים פרק לט he: שונות => תהילים => תהילים פרק לט
IGuJkWwc en: Various => Psalms - Tehilim => תהילים פרק מ he: שונות => תהילים => תהילים פרק מ
xHqxvO6e en: Various => Psalms - Tehilim => תהילים פרק מא he: שונות => תהילים => תהילים פרק מא
XgNnJq9K en: Various => Psalms - Tehilim => תהילים פרק מב he: שונות => תהילים => תהילים פרק מב
uxkhyxuD en: Various => Psalms - Tehilim => תהילים פרק מג he: שונות => תהילים => תהילים פרק מג
dyS2pHH4 en: Various => Psalms - Tehilim => תהילים פרק מד he: שונות => תהילים => תהילים פרק מד
8ZMmtD29 en: Various => Psalms - Tehilim => תהילים פרק מה he: שונות => תהילים => תהילים פרק מה
V45KgZU1 en: Various => Psalms - Tehilim => תהילים פרק מו he: שונות => תהילים => תהילים פרק מו
qxaqeNdK en: Various => Psalms - Tehilim => תהילים פרק מז he: שונות => תהילים => תהילים פרק מז
uCgnh59Y en: Various => Psalms - Tehilim => תהילים פרק מח he: שונות => תהילים => תהילים פרק מח
6lJqCrLT en: Various => Psalms - Tehilim => תהילים פרק מט he: שונות => תהילים => תהילים פרק מט
L7v41u32 en: Various => Psalms - Tehilim => תהילים פרק נ he: שונות => תהילים => תהילים פרק נ
4A5mguQl en: Various => Psalms - Tehilim => תהילים פרק נא he: שונות => תהילים => תהילים פרק נא
0L8ww2no en: Various => Psalms - Tehilim => תהילים פרק נב he: שונות => תהילים => תהילים פרק נב
CQ3onige en: Various => Psalms - Tehilim => תהילים פרק נג he: שונות => תהילים => תהילים פרק נג
NPP5armi en: Various => Psalms - Tehilim => תהילים פרק נד he: שונות => תהילים => תהילים פרק נד
fmP7mKje en: Various => Psalms - Tehilim => תהילים פרק נה he: שונות => תהילים => תהילים פרק נה
msSWEutj en: Various => Psalms - Tehilim => תהילים פרק נו he: שונות => תהילים => תהילים פרק נו
gLfTsqvG en: Various => Psalms - Tehilim => תהילים פרק נז he: שונות => תהילים => תהילים פרק נז
mO3uctlU en: Various => Psalms - Tehilim => תהילים פרק נח he: שונות => תהילים => תהילים פרק נח
ASTerc0R en: Various => Psalms - Tehilim => תהילים פרק נט he: שונות => תהילים => תהילים פרק נט
5cA0diwd en: Various => Psalms - Tehilim => תהילים פרק ס he: שונות => תהילים => תהילים פרק ס
q9sn6emk en: Various => Psalms - Tehilim => תהילים פרק סא he: שונות => תהילים => תהילים פרק סא
gaYnykmU en: Various => Psalms - Tehilim => תהילים פרק סב he: שונות => תהילים => תהילים פרק סב
w6XYWV3q en: Various => Psalms - Tehilim => תהילים פרק סג he: שונות => תהילים => תהילים פרק סג
uL4Nqa0N en: Various => Psalms - Tehilim => תהילים פרק סד he: שונות => תהילים => תהילים פרק סד
Nh86mmKc en: Various => Psalms - Tehilim => תהילים פרק סה he: שונות => תהילים => תהילים פרק סה
Rwa3wpeY en: Various => Psalms - Tehilim => תהילים פרק סו he: שונות => תהילים => תהילים פרק סו
Y7n5rjDn en: Various => Psalms - Tehilim => תהילים פרק סז he: שונות => תהילים => תהילים פרק סז
ZUbRuoTA en: Various => Psalms - Tehilim => תהילים פרק סח he: שונות => תהילים => תהילים פרק סח
6p2prpu8 en: Various => Psalms - Tehilim => תהילים פרק סט he: שונות => תהילים => תהילים פרק סט
gUG7Dg9I en: Various => Psalms - Tehilim => תהילים פרק ע he: שונות => תהילים => תהילים פרק ע
rJqrA9OZ en: Various => Psalms - Tehilim => תהילים פרק עא he: שונות => תהילים => תהילים פרק עא
C4eKUYfc en: Various => Psalms - Tehilim => תהילים פרק עב he: שונות => תהילים => תהילים פרק עב
BpOb4s3a en: Various => Psalms - Tehilim => תהילים פרק עג he: שונות => תהילים => תהילים פרק עג
yHexiQ5i en: Various => Psalms - Tehilim => תהילים פרק עד he: שונות => תהילים => תהילים פרק עד
qypXn6rg en: Various => Psalms - Tehilim => תהילים פרק עה he: שונות => תהילים => תהילים פרק עה
wRK1Q7rn en: Various => Psalms - Tehilim => תהילים פרק עו he: שונות => תהילים => תהילים פרק עו
avQrsu7z en: Various => Psalms - Tehilim => תהילים פרק עז he: שונות => תהילים => תהילים פרק עז
6b6VRCql en: Various => Psalms - Tehilim => תהילים פרק עח he: שונות => תהילים => תהילים פרק עח
3QW9jzBR en: Various => Psalms - Tehilim => תהילים פרק עט he: שונות => תהילים => תהילים פרק עט
BdjrvWRE en: Various => Psalms - Tehilim => תהילים פרק פ he: שונות => תהילים => תהילים פרק פ
bX3hZuxY en: Various => Psalms - Tehilim => תהילים פרק פא he: שונות => תהילים => תהילים פרק פא
Nob6fPMb en: Various => Psalms - Tehilim => תהילים פרק פב he: שונות => תהילים => תהילים פרק פב
1dF57pPR en: Various => Psalms - Tehilim => תהילים פרק פג he: שונות => תהילים => תהילים פרק פג
4T0R0lJk en: Various => Psalms - Tehilim => תהילים פרק פד he: שונות => תהילים => תהילים פרק פד
a1eKd4Yt en: Various => Psalms - Tehilim => תהילים פרק פה he: שונות => תהילים => תהילים פרק פה
PiZ2w4jk en: Various => Psalms - Tehilim => תהילים פרק פו he: שונות => תהילים => תהילים פרק פו
Fd4eTGPq en: Various => Psalms - Tehilim => תהילים פרק פז he: שונות => תהילים => תהילים פרק פז
4umm4SY5 en: Various => Psalms - Tehilim => תהילים פרק פח he: שונות => תהילים => תהילים פרק פח
ZPh9DWjR en: Various => Psalms - Tehilim => תהילים פרק פט he: שונות => תהילים => תהילים פרק פט
KrgtH5UM en: Various => Psalms - Tehilim => תהילים פרק צ he: שונות => תהילים => תהילים פרק צ
DmG0eaBP en: Various => Psalms - Tehilim => תהילים פרק צא he: שונות => תהילים => תהילים פרק צא
TSltznq5 en: Various => Psalms - Tehilim => תהילים פרק צב he: שונות => תהילים => תהילים פרק צב
kdaLEHNv en: Various => Psalms - Tehilim => תהילים פרק צג he: שונות => תהילים => תהילים פרק צג
BYnTWeL5 en: Various => Psalms - Tehilim => תהילים פרק צד he: שונות => תהילים => תהילים פרק צד
WgQrBbSZ en: Various => Psalms - Tehilim => תהילים פרק צה he: שונות => תהילים => תהילים פרק צה
TMxAqwhW en: Various => Psalms - Tehilim => תהילים פרק צו he: שונות => תהילים => תהילים פרק צו
TXE0ijYH en: Various => Psalms - Tehilim => תהילים פרק צז he: שונות => תהילים => תהילים פרק צז
biyB5JLK en: Various => Psalms - Tehilim => תהילים פרק צח he: שונות => תהילים => תהילים פרק צח
8HNJgYnS en: Various => Psalms - Tehilim => תהילים פרק צט he: שונות => תהילים => תהילים פרק צט
KsAw8cTi en: Various => Psalms - Tehilim => תהילים פרק ק he: שונות => תהילים => תהילים פרק ק
sYZh9zcs en: Various => Psalms - Tehilim => תהילים פרק קא he: שונות => תהילים => תהילים פרק קא
XzsqtzAe en: Various => Psalms - Tehilim => תהילים פרק קב he: שונות => תהילים => תהילים פרק קב
wDGGEMGq en: Various => Psalms - Tehilim => תהילים פרק קג he: שונות => תהילים => תהילים פרק קג
QauaMH57 en: Various => Psalms - Tehilim => תהילים פרק קד he: שונות => תהילים => תהילים פרק קד
HxbedPV5 en: Various => Psalms - Tehilim => תהילים פרק קה he: שונות => תהילים => תהילים פרק קה
NmTw2BjW en: Various => Psalms - Tehilim => תהילים פרק קו he: שונות => תהילים => תהילים פרק קו
7gi0JDbl en: Various => Psalms - Tehilim => תהילים פרק קז he: שונות => תהילים => תהילים פרק קז
Hukd82ky en: Various => Psalms - Tehilim => תהילים פרק קח he: שונות => תהילים => תהילים פרק קח
90VFznRI en: Various => Psalms - Tehilim => תהילים פרק קט he: שונות => תהילים => תהילים פרק קט
ZEwnDi28 en: Various => Psalms - Tehilim => תהילים פרק קי he: שונות => תהילים => תהילים פרק קי
2rBIsGNJ en: Various => Psalms - Tehilim => תהילים פרק קיא he: שונות => תהילים => תהילים פרק קיא
2zPnfxAK en: Various => Psalms - Tehilim => תהילים פרק קיב he: שונות => תהילים => תהילים פרק קיב
i3Bizo6i en: Various => Psalms - Tehilim => תהילים פרק קיג he: שונות => תהילים => תהילים פרק קיג
HxuTQipo en: Various => Psalms - Tehilim => תהילים פרק קיד he: שונות => תהילים => תהילים פרק קיד
Xeasz2Kw en: Various => Psalms - Tehilim => תהילים פרק קטו he: שונות => תהילים => תהילים פרק קטו
5ftyE3Ib en: Various => Psalms - Tehilim => תהילים פרק קטז he: שונות => תהילים => תהילים פרק קטז
9IMyMVlq en: Various => Psalms - Tehilim => תהילים פרק קיז he: שונות => תהילים => תהילים פרק קיז
gNEZlhay en: Various => Psalms - Tehilim => תהילים פרק קיח he: שונות => תהילים => תהילים פרק קיח
JY2VX03E en: Various => Psalms - Tehilim => תהילים פרק קיט he: שונות => תהילים => תהילים פרק קיט
tJGxqNy4 en: Various => Psalms - Tehilim => תהילים פרק קכ he: שונות => תהילים => תהילים פרק קכ
Uw1NuJuS en: Various => Psalms - Tehilim => תהילים פרק קכא he: שונות => תהילים => תהילים פרק קכא
c0YWP6cT en: Various => Psalms - Tehilim => תהילים פרק קכב he: שונות => תהילים => תהילים פרק קכב
JTvvuRUU en: Various => Psalms - Tehilim => תהילים פרק קכג he: שונות => תהילים => תהילים פרק קכג
YIhHWDIg en: Various => Psalms - Tehilim => תהילים פרק קכד he: שונות => תהילים => תהילים פרק קכד
hmKNzJLD en: Various => Psalms - Tehilim => תהילים פרק קכה he: שונות => תהילים => תהילים פרק קכה
T7odUDKc en: Various => Psalms - Tehilim => תהילים פרק קכו he: שונות => תהילים => תהילים פרק קכו
rC2H3LH2 en: Various => Psalms - Tehilim => תהילים פרק קכז he: שונות => תהילים => תהילים פרק קכז
0rZ7jD24 en: Various => Psalms - Tehilim => תהילים פרק קכח he: שונות => תהילים => תהילים פרק קכח
VJXxpX6O en: Various => Psalms - Tehilim => תהילים פרק קכט he: שונות => תהילים => תהילים פרק קכט
k4zTmRAm en: Various => Psalms - Tehilim => תהילים פרק קל he: שונות => תהילים => תהילים פרק קל
qa5IJ753 en: Various => Psalms - Tehilim => תהילים פרק קלא he: שונות => תהילים => תהילים פרק קלא
4Jw2J5m1 en: Various => Psalms - Tehilim => תהילים פרק קלב he: שונות => תהילים => תהילים פרק קלב
IoVIMq8h en: Various => Psalms - Tehilim => תהילים פרק קלג he: שונות => תהילים => תהילים פרק קלג
RIxusUpY en: Various => Psalms - Tehilim => תהילים פרק קלד he: שונות => תהילים => תהילים פרק קלד
gg9nplhx en: Various => Psalms - Tehilim => תהילים פרק קלה he: שונות => תהילים => תהילים פרק קלה
nIne9mNT en: Various => Psalms - Tehilim => תהילים פרק קלו he: שונות => תהילים => תהילים פרק קלו
OqhVEsnh en: Various => Psalms - Tehilim => תהילים פרק קלז he: שונות => תהילים => תהילים פרק קלז
vN5HnmJN en: Various => Psalms - Tehilim => תהילים פרק קלח he: שונות => תהילים => תהילים פרק קלח
rGqwthBI en: Various => Psalms - Tehilim => תהילים פרק קלט he: שונות => תהילים => תהילים פרק קלט
1ZibEtN8 en: Various => Psalms - Tehilim => תהילים פרק קמ he: שונות => תהילים => תהילים פרק קמ
E8YDds46 en: Various => Psalms - Tehilim => תהילים פרק קמא he: שונות => תהילים => תהילים פרק קמא
BxJzssYr en: Various => Psalms - Tehilim => תהילים פרק קמב he: שונות => תהילים => תהילים פרק קמב
hDVmJwW8 en: Various => Psalms - Tehilim => תהילים פרק קמג he: שונות => תהילים => תהילים פרק קמג
idVXkg5s en: Various => Psalms - Tehilim => תהילים פרק קמד he: שונות => תהילים => תהילים פרק קמד
14ZjxxHc en: Various => Psalms - Tehilim => תהילים פרק קמה he: שונות => תהילים => תהילים פרק קמה
XFyJF6vY en: Various => Psalms - Tehilim => תהילים פרק קמו he: שונות => תהילים => תהילים פרק קמו
glqHYtu4 en: Various => Psalms - Tehilim => תהילים פרק קמז he: שונות => תהילים => תהילים פרק קמז
GnGhY71I en: Various => Psalms - Tehilim => תהילים פרק קמח he: שונות => תהילים => תהילים פרק קמח
ArAPMSPT en: Various => Psalms - Tehilim => תהילים פרק קמט he: שונות => תהילים => תהילים פרק קמט
tLKlD28X en: Various => Psalms - Tehilim => תהילים פרק קנ he: שונות => תהילים => תהילים פרק קנ
mQunD2ac en: Various => Derech HaShem he: שונות => דרך ה'
WiyCqS2Y en: Various => Pirkei Avot (heb) he: שונות => פרקי אבות
H4mUYM0r en: Various => Pirkei Avot (heb) => פרק א' he: שונות => פרקי אבות => פרק א'
4xACARgU en: Various => Pirkei Avot (heb) => פרק ב' he: שונות => פרקי אבות => פרק ב'
yccNY73g en: Various => Pirkei Avot (heb) => פרק ג' he: שונות => פרקי אבות => פרק ג'
b3yytOO9 en: Various => Pirkei Avot (heb) => פרק ד' he: שונות => פרקי אבות => פרק ד'
7wWxyjZN en: Various => Pirkei Avot (heb) => פרק ה' he: שונות => פרקי אבות => פרק ה'
GVYi1jAY en: Various => Pirkei Avot (heb) => פרק ו' he: שונות => פרקי אבות => פרק ו'
3ZEBiLLs en: Various => מגילות he: שונות => מגילות
xxSe5qbb en: Various => מגילות => Ecclesiastes he: שונות => מגילות => מגילת קהלת
uluPiuOK en: Various => מגילות => Esther he: שונות => מגילות => מגילת אסתר
a2cE3dbJ en: Various => מגילות => Lamentations he: שונות => מגילות => מגילת איכה
jYCKYWRF en: Various => מגילות => Ruth he: שונות => מגילות => מגילת רות
8iCV8Sa0 en: Various => מגילות => Song of Songs he: שונות => מגילות => מגילת שיר השירים
bb en: Bnei Baruch he: בני ברוך


###`

const schema = `{
  "name": "search_queries",
  "strict": true,
  "schema": {
    "type": "object",
    "required": [
      "queries"
    ],
    "properties": {
      "queries": {
        "type": "array",
        "items": {
          "type": "object",
          "required": [
            "text_query",
            "filters",
            "start_date",
            "end_date"
          ],
          "properties": {
            "filters": {
              "type": "array",
              "items": {
                "type": "object",
                "required": [
                  "type",
                  "value"
                ],
                "properties": {
                  "type": {
                    "enum": [
                      "content_type",
                      "source",
                      "author",
                      "original-language",
                      "media-language"
                    ],
                    "type": "string",
                    "description": "The type of filter to apply."
                  },
                  "value": {
                    "type": "string",
                    "description": "The value associated with the filter type."
                  }
                },
                "additionalProperties": false
              },
              "description": "An array of filters to apply to the search query."
            },
            "end_date": {
              "type": "string",
              "nullable": true,
              "description": "The ending date in yyyy-MM-dd format to filter results."
            },
            "start_date": {
              "type": "string",
              "nullable": true,
              "description": "The starting date in yyyy-MM-dd format to filter results."
            },
            "text_query": {
              "type": "string",
              "description": "The search query string to be used in the search engine."
            }
          },
          "additionalProperties": false
        },
        "description": "An array of search query objects."
      }
    },
    "additionalProperties": false
  }
}`

func GenerateSearchQueries(query string) ([]Query, error) {

	if query == "" {
		return []Query{}, nil
	}

	token := viper.GetString("openai.token")
	if token == "" {
		return nil, errors.New("OpenAI token is not configured")
	}
	openaiService := NewOpenAIService(token)
	sysMsg := fmt.Sprintf(sysMsgMask, time.Now().Format("Monday, January 2, 2006"))
	messages := []LLMBotMessage{
		{
			Role:    "developer",
			Content: sysMsg,
		},
		{
			Role:    "user",
			Content: query,
		},
	}
	reasoningEffort := "low"
	var queriesResult QueriesResult
	err := openaiService.GetStructuredOutput(schema, "o3", nil, messages, nil, &reasoningEffort, &queriesResult)
	if err != nil {
		return nil, err
	}
	return queriesResult.Queries, nil
}
