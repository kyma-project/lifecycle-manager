package types

import (
	"bytes"
	"fmt"
)

func NewMultiError(errs []error) *MultiError {
	return &MultiError{Errs: errs}
}

type MultiError struct {
	Errs []error
}

func (m MultiError) Error() string {
	buf := &bytes.Buffer{}
	for _, err := range m.Errs {
		_, _ = fmt.Fprintf(buf, "%v\n", err.Error())
	}
	return buf.String()
}
