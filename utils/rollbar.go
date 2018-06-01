package utils

import (
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/stvp/rollbar"
	"net/http"
	"fmt"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func errorsToRollbarStack(st stackTracer) rollbar.Stack {
	t := st.StackTrace()
	rs := make(rollbar.Stack, len(t))
	for i, f := range t {
		// Program counter as it's computed internally in errors.Frame
		pc := uintptr(f) - 1
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			rs[i] = rollbar.Frame{
				Filename: "unknown",
				Method:   "?",
				Line:     0,
			}
			continue
		}

		// symtab info
		file, line := fn.FileLine(pc)
		name := fn.Name()

		// trim compile time GOPATH from file name
		fileWImportPath := trimGOPATH(name, file)

		// Strip only method name from FQN
		idx := strings.LastIndex(name, "/")
		name = name[idx+1:]
		idx = strings.Index(name, ".")
		name = name[idx+1:]

		rs[i] = rollbar.Frame{
			Filename: fileWImportPath,
			Method:   name,
			Line:     line,
		}
	}

	return rs
}

// Taken AS IS from errors pkg since it's not exported there.
// Check out the source code with good comments on https://github.com/pkg/errors/blob/master/stack.go
func trimGOPATH(name, file string) string {
	const sep = "/"
	goal := strings.Count(name, sep) + 2
	i := len(file)
	for n := 0; n < goal; n++ {
		i = strings.LastIndex(file[:i], sep)
		if i == -1 {
			i = -len(sep)
			break
		}
	}
	file = file[i+len(sep):]
	return file
}

func LogRequestError(r *http.Request, err error) {
	st, ok := err.(stackTracer)
	if ok {
		fmt.Printf("%s: %+v\n", st, st.StackTrace())
	}

	// Log if we have a token setup
	if len(rollbar.Token) != 0 {
		if ok {
			rollbar.RequestErrorWithStack(rollbar.ERR, r, err, errorsToRollbarStack(st))
		} else {
			rollbar.RequestError(rollbar.ERR, r, err)
		}
	}
}

func LogError(err error) {
	st, ok := err.(stackTracer)
	if ok {
		fmt.Printf("%s: %+v\n", st, st.StackTrace())
	}

	// Log if we have a token setup
	if len(rollbar.Token) != 0 {
		if ok {
			rollbar.ErrorWithStack(rollbar.ERR, err, errorsToRollbarStack(st))
		} else {
			rollbar.Error(rollbar.ERR, err)
		}
	}
}