package buff

import "errors"

const (
	crlf                = "\r\n"
	headerBodySeparator = "\r\n\r\n"
)

var (
	errNeedMoreData   = errors.New("incomplete http request")
	errHeaderTooLarge = errors.New("request header too large")
)
