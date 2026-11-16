package common

import (
	"encoding/json"
	"errors"
)

var ErrRelinked = errors.New("the chat moved to a new page, by a reload or in another tab, which ended what ran on the old page")

type ErrorWithOutput struct {
	Err    error
	Output json.RawMessage
}

func (e ErrorWithOutput) Error() string {
	return e.Err.Error()
}

func (e ErrorWithOutput) Unwrap() error {
	return e.Err
}
