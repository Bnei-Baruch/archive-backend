package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func FormatDateWithMonthNames(d time.Time, format string, monthNames [][]string) ([]string, error) {
	if monthNames == nil {
		return nil, errors.New("monthNames is nil.")
	}
	valuesWithMonthNames := []string{}
	values := FormatDate(d, format)
	for _, val := range values {
		for i := range monthNames {
			if len(monthNames[i]) != 12 {
				return nil, errors.New("Month names length is not 12.")
			}
			monthName := monthNames[i][d.Month()-1]
			valuesWithMonthNames = append(valuesWithMonthNames, strings.Replace(val, "MMMM", monthName, 1))
		}
	}
	return valuesWithMonthNames, nil
}

func FormatDate(d time.Time, format string) []string {
	values := []string{}
	fd := func() string {
		val := strings.Replace(format, "yyyy", strconv.Itoa(d.Year()), 1)
		val = strings.Replace(val, "yy", strconv.Itoa(d.Year()%100), 1)
		return val
	}
	val := fd()
	val = strings.Replace(format, "dd", strconv.Itoa(d.Day()), 1)
	val = strings.Replace(val, "mm", strconv.Itoa(int(d.Month())), 1)
	values = append(values, val)
	if d.Month() < 10 && d.Day() < 10 {
		val := fd()
		val = strings.Replace(format, "dd", fmt.Sprintf("0%d", d.Day()), 1)
		val = strings.Replace(format, "mm", fmt.Sprintf("0%d", d.Month()), 1)
		values = append(values, val)
	} else if d.Month() < 10 {
		val := fd()
		val = strings.Replace(format, "dd", fmt.Sprintf("%d", d.Day()), 1)
		val = strings.Replace(format, "mm", fmt.Sprintf("0%d", d.Month()), 1)
		values = append(values, val)
	} else if d.Day() < 10 {
		val := fd()
		val = strings.Replace(format, "dd", fmt.Sprintf("0%d", d.Day()), 1)
		val = strings.Replace(format, "mm", fmt.Sprintf("%d", d.Month()), 1)
		values = append(values, val)
	}
	return values
}
