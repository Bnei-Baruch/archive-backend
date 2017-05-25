package utils

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
	for i, a := range args {
		c[i] = a
	}
	return c
}

func ConvertArgsString(args []string) []interface{} {
	c := make([]interface{}, len(args))
	for i, a := range args {
		c[i] = a
	}
	return c
}
