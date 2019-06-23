package utils

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"time"

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

func is(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}
	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}
	return ret
}

func Pprint(l interface{}) string {
	var s []string
	for _, i := range is(l) {
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
			values = append(values, fmt.Sprintf("%+v:[%s]", k, Join(is(v), ",")))
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
