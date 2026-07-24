package wiki

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	KindNotFound     ErrorKind = "not_found"
	KindBlocked      ErrorKind = "blocked"
	KindRateLimited  ErrorKind = "rate_limited"
	KindUpstream     ErrorKind = "upstream"
	KindHTTPStatus   ErrorKind = "http_status"
	KindNetwork      ErrorKind = "network"
	KindBodyTooLarge ErrorKind = "body_too_large"
	KindParse        ErrorKind = "parse"
)

// Error 包含 Wiki 请求或解析失败的结构化上下文。
type Error struct {
	Kind       ErrorKind
	Operation  string
	URL        string
	StatusCode int
	Missing    []string
	Err        error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Wiki %s 失败（%s）: %v", e.Operation, e.Kind, e.Err)
	}
	return fmt.Sprintf("Wiki %s 失败（%s）", e.Operation, e.Kind)
}

func (e *Error) Unwrap() error { return e.Err }

func IsKind(err error, kind ErrorKind) bool {
	var wikiErr *Error
	return errors.As(err, &wikiErr) && wikiErr.Kind == kind
}
