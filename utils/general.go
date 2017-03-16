package utils

// panic if err != nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}
