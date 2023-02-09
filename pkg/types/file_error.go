package types

import (
	"os"
)

// ParsedFile contains the parsed content and any error encountered during processing of a file.
//
//nolint:errname
type ParsedFile struct {
	content string
	err     error
}

func NewParsedFile(content string, err error) *ParsedFile {
	return &ParsedFile{
		content: content,
		err:     err,
	}
}

// Error returns the error message during parsing of the file.
func (fe *ParsedFile) Error() string {
	if fe.err != nil {
		return fe.err.Error()
	}
	return ""
}

// IsResultConclusive indicates if caching results are processed.
func (fe *ParsedFile) IsResultConclusive() bool {
	return fe.content != "" || fe.getFilteredError() != nil
}

// FilterOsErrors filters out os.IsPermission and os.IsNotExist errors.
func (fe *ParsedFile) FilterOsErrors() *ParsedFile {
	fe.err = fe.getFilteredError()
	return fe
}

// GetContent returns the raw content of the parsed file.
func (fe *ParsedFile) GetContent() string {
	return fe.content
}

// GetRawError returns the raw error during parsing of the file.
func (fe *ParsedFile) GetRawError() error {
	return fe.err
}

func (fe *ParsedFile) isPermissionError() bool {
	return os.IsPermission(fe.err)
}

func (fe *ParsedFile) isExistError() bool {
	return os.IsNotExist(fe.err)
}

func (fe *ParsedFile) getFilteredError() error {
	if fe.err != nil && !fe.isPermissionError() && !fe.isExistError() {
		return fe.err
	}
	return nil
}
