package utils

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/pkg/errors"
)

const uidBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const lettersBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func GenerateUID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = uidBytes[rand.Intn(len(uidBytes))]
	}
	return string(b)
}

func GenerateName(n int) string {
	b := make([]byte, n)
	b[0] = lettersBytes[rand.Intn(len(lettersBytes))]
	for i := range b[1:] {
		b[i+1] = uidBytes[rand.Intn(len(uidBytes))]
	}
	return string(b)
}

// panic if err != nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// Joins two errors to one.
func JoinErrors(one error, two error) error {
	if one == nil && two == nil {
		return nil
	}
	if one != nil && two != nil {
		return errors.Wrapf(two, "%s\nPrev Error", one.Error())
	}
	if one != nil {
		return one
	}
	return two
}

func JoinErrorsWrap(one error, two error, twoErrorMessage string) error {
	if two != nil {
		if twoErrorMessage == "" {
			return JoinErrors(one, two)
		} else {
			return JoinErrors(one, errors.Wrap(two, twoErrorMessage))
		}
	}
	return one
}

// Like math.Min for int
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// Like math.Max for int
func MaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Like math.Min for int
func MinInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func MinMax(x, y int) (int, int) {
	if x < y {
		return x, y
	}
	return y, x
}

// true if every string in given slice is empty
func IsEmpty(s []string) bool {
	for _, x := range s {
		if x != "" {
			return false
		}
	}
	return true
}

func ConvertArgsInt64(args []int64) []interface{} {
	c := make([]interface{}, len(args))
	for i := range args {
		c[i] = args[i]
	}
	return c
}

func ConvertArgsString(args []string) []interface{} {
	c := make([]interface{}, len(args))
	for i := range args {
		c[i] = args[i]
	}
	return c
}

func Int64InSlice(i int64, s []int64) bool {
	for _, v := range s {
		if v == i {
			return true
		}
	}
	return false
}

func StringInSlice(str string, s []string) bool {
	for i := range s {
		if str == s[i] {
			return true
		}
	}
	return false
}

func Is(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice && s.Kind() != reflect.Array {
		panic(fmt.Sprintf("InterfaceSlice() given a non-slice type: %s", s.Kind()))
	}
	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}
	return ret
}

func Pprint(l interface{}) string {
	var s []string
	for _, i := range Is(l) {
		s = append(s, fmt.Sprintf("%+v", i))
	}
	return strings.Join(s, "\n\t")
}

func Join(l []interface{}, separator string) string {
	var ret []string
	for _, v := range l {
		ret = append(ret, fmt.Sprintf("%+v", v))
	}
	return strings.Join(ret, separator)
}

func JoinInt64(l []int64, separator string) string {
	var ret []string
	for _, v := range l {
		ret = append(ret, fmt.Sprintf("%d", v))
	}
	return strings.Join(ret, separator)
}

func PrintMap(m interface{}) (string, error) {
	mValue := reflect.ValueOf(m)
	if mValue.Kind() != reflect.Map {
		return "", errors.New("Input is not map.")
	}
	var values []string
	for _, k := range mValue.MapKeys() {
		v := mValue.MapIndex(k)
		vValue := reflect.ValueOf(v)
		if vValue.Kind() == reflect.Slice {
			values = append(values, fmt.Sprintf("%+v:[%s]", k, Join(Is(v), ",")))
		} else {
			values = append(values, fmt.Sprintf("%+v:%+v", k, v))
		}
	}
	return strings.Join(values, ","), nil
}

func IntersectSortedStringSlices(first []string, second []string) []string {
	ret := []string{}
	i := 0
	j := 0
	for i < len(first) && j < len(second) {
		cmp := strings.Compare(first[i], second[j])
		if cmp == 0 {
			ret = append(ret, first[i])
			i++
		} else if cmp < 0 {
			i++
		} else {
			j++
		}
	}
	return ret
}

func StringMapOrderedKeys(m interface{}) []string {
	mValue := reflect.ValueOf(m)
	if mValue.Kind() != reflect.Map {
		panic("m is not map")
	}
	if mValue.Type().Key().Kind() != reflect.String {
		panic("m key is not string")
	}
	keys := make([]string, 0, len(mValue.MapKeys()))
	for _, k := range mValue.MapKeys() {
		keys = append(keys, k.Interface().(string))
	}
	sort.Strings(keys)
	return keys
}

func SumAndMax(values []int) (int, int) {
	var sum int = 0
	var max int = 0
	for _, val := range values {
		if val > max {
			max = val
		}
		sum += val
	}
	return sum, max
}

func Contains(list []interface{}, elem interface{}) bool {
	for _, t := range list {
		if t == elem {
			return true
		}
	}
	return false
}

// Return values: 1. Whole term is numeric. 2. At least part of the term is numeric.
func HasNumeric(term string) (bool, bool) {
	allIsDigit := true
	hasDigit := false
	for _, r := range term {
		if unicode.IsDigit(r) {
			hasDigit = true
		} else {
			allIsDigit = false
		}
	}
	return allIsDigit, hasDigit
}

func FilterStringSlice(list []string, test func(string) bool) []string {
	filtered := []string(nil)
	for _, item := range list {
		if test(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func Filter(list []interface{}, test func(interface{}) bool) ([]interface{}, []interface{}) {
	passed := []interface{}{}
	rest := []interface{}{}
	for _, item := range list {
		if test(item) {
			passed = append(passed, item)
		} else {
			rest = append(rest, item)
		}
	}
	return passed, rest
}

func Select(list []interface{}, newValue func(interface{}) interface{}) []interface{} {
	ret := []interface{}{}
	for _, item := range list {
		ret = append(ret, newValue(item))
	}
	return ret
}

func MaxByValue(list []interface{}, value func(interface{}) float64) interface{} {
	if len(list) == 0 {
		return nil
	}
	sort.SliceStable(list, func(i, j int) bool {
		return value(list[i]) > value(list[j])
	})
	return list[0]
}

func First(list []interface{}, test func(interface{}) bool) interface{} {
	filtered, _ := Filter(list, test)
	if len(filtered) > 0 {
		return filtered[0]
	}
	return nil
}

func GroupBy(list []interface{}, value func(interface{}) interface{}) map[interface{}][]interface{} {
	ret := map[interface{}][]interface{}{}
	for _, item := range list {
		key := value(item)
		if _, ok := ret[key]; !ok {
			ret[key] = []interface{}{}
		}
		ret[key] = append(ret[key], item)
	}
	return ret
}

func ClearDuplicateString(list []string) []string {
	m := make(map[string]bool, len(list))
	ret := make([]string, 0)
	for _, x := range list {
		if _, ok := m[x]; !ok {
			ret = append(ret, x)
			m[x] = true
		}
	}
	return ret
}

func ClearDuplicateInt64(list []int64) []int64 {
	m := make(map[int64]bool, len(list))
	ret := make([]int64, 0)
	for _, x := range list {
		if _, ok := m[x]; !ok {
			ret = append(ret, x)
			m[x] = true
		}
	}
	return ret
}
