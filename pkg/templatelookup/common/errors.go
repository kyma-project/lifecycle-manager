package common

import "errors"

var (
	ErrNoTemplatesInListResult = errors.New("no templates were found")
	ErrTemplateNotIdentified   = errors.New("no unique template could be identified")
)
