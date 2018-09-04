package utils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

// panic if err != nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// Like math.Min for int
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// Like math.Min for int
func MaxInt(x, y int) int {
	if x > y {
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
