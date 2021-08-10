package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/pkg/errors"
)

func FormatDateWithMonthNames(d time.Time, format string, monthNames [][]string, lang string) ([]string, error) {
	const monthNameRep = "MMMM"
	if monthNames == nil {
		return nil, errors.New("monthNames is nil.")
	}
	hasMonthNameRep := strings.Contains(format, monthNameRep)
	values := FormatDate(d, format, hasMonthNameRep && lang == consts.LANG_ENGLISH, !hasMonthNameRep)
	if !hasMonthNameRep {
		return values, nil
	}
	valuesWithMonthNames := []string{}
	for _, val := range values {
		for i := range monthNames {
			if len(monthNames[i]) != 12 {
				return nil, errors.New("Month names length is not 12.")
			}
			monthName := monthNames[i][d.Month()-1]
			valuesWithMonthNames = append(valuesWithMonthNames, strings.Replace(val, monthNameRep, monthName, 1))
		}
	}
	return valuesWithMonthNames, nil
}

func FormatDate(d time.Time, format string, addOrdinal bool, addDayZeroPrefix bool) []string {
	values := []string{}
	fd := func() string {
		val := strings.Replace(format, "yyyy", strconv.Itoa(d.Year()), 1)
		val = strings.Replace(val, "yy", strconv.Itoa(d.Year()%100), 1)
		return val
	}
	val := fd()
	val = strings.Replace(val, "dd", strconv.Itoa(d.Day()), 1)
	val = strings.Replace(val, "mm", strconv.Itoa(int(d.Month())), 1)
	values = append(values, val)
	if addOrdinal {
		val := fd()
		val = strings.Replace(val, "dd", Ordinal(d.Day()), 1)
		val = strings.Replace(val, "mm", fmt.Sprintf("%d", d.Month()), 1)
		values = append(values, val)
	}
	if d.Month() < 10 && d.Day() < 10 && addDayZeroPrefix {
		val := fd()
		val = strings.Replace(val, "dd", fmt.Sprintf("0%d", d.Day()), 1)
		val = strings.Replace(val, "mm", fmt.Sprintf("0%d", d.Month()), 1)
		values = append(values, val)
	} else if d.Month() < 10 {
		val := fd()
		val = strings.Replace(val, "dd", fmt.Sprintf("%d", d.Day()), 1)
		val = strings.Replace(val, "mm", fmt.Sprintf("0%d", d.Month()), 1)
		values = append(values, val)
	} else if d.Day() < 10 && addDayZeroPrefix {
		val := fd()
		val = strings.Replace(val, "dd", fmt.Sprintf("0%d", d.Day()), 1)
		val = strings.Replace(val, "mm", fmt.Sprintf("%d", d.Month()), 1)
		values = append(values, val)
	}
	return values
}

func Ordinal(x int) string {
	suffix := "th"
	switch x % 10 {
	case 1:
		if x%100 != 11 {
			suffix = "st"
		}
	case 2:
		if x%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if x%100 != 13 {
			suffix = "rd"
		}
	}
	return strconv.Itoa(x) + suffix
}
