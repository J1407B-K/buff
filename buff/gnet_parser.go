package buff

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

var (
	crlfBytes          = []byte(crlf)
	headerSeparatorBuf = []byte(headerBodySeparator)
)

func parseHTTPRequest(buf []byte, maxHeaderBytes int) (*http.Request, int, bool, error) {
	if len(buf) == 0 {
		return nil, 0, false, errNeedMoreData
	}
	if maxHeaderBytes <= 0 {
		maxHeaderBytes = defaultMaxHeaderBytes
	}
	headerEnd := bytes.Index(buf, headerSeparatorBuf)
	if headerEnd == -1 {
		if len(buf) > maxHeaderBytes {
			return nil, 0, false, errHeaderTooLarge
		}
		return nil, 0, false, errNeedMoreData
	}
	if headerEnd > maxHeaderBytes {
		return nil, 0, false, errHeaderTooLarge
	}

	lines := bytes.Split(buf[:headerEnd], crlfBytes)
	if len(lines) == 0 {
		return nil, 0, false, fmt.Errorf("empty request line")
	}
	requestLine := string(lines[0])
	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) != 3 {
		return nil, 0, false, fmt.Errorf("invalid request line: %q", requestLine)
	}
	method, target, proto := parts[0], parts[1], parts[2]
	if method == "" {
		return nil, 0, false, fmt.Errorf("missing method")
	}
	if !strings.HasPrefix(strings.ToUpper(proto), "HTTP/") {
		return nil, 0, false, fmt.Errorf("invalid proto: %s", proto)
	}

	maj, min, err := parseHTTPVersion(proto)
	if err != nil {
		return nil, 0, false, err
	}

	header := make(http.Header, len(lines)-1)
	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		colon := bytes.IndexByte(line, ':')
		if colon <= 0 {
			return nil, 0, false, fmt.Errorf("malformed header line: %q", string(line))
		}
		key := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(string(line[:colon])))
		val := strings.TrimSpace(string(line[colon+1:]))
		header.Add(key, val)
	}

	if te := header.Get("Transfer-Encoding"); te != "" && !strings.EqualFold(te, "identity") {
		return nil, 0, false, fmt.Errorf("transfer-encoding %q not supported", te)
	}

	contentLength := 0
	if cl := header.Get("Content-Length"); cl != "" {
		clVal, err := strconv.ParseInt(cl, 10, 64)
		if err != nil || clVal < 0 {
			return nil, 0, false, fmt.Errorf("invalid Content-Length: %q", cl)
		}
		if clVal > int64(len(buf)) {
			return nil, 0, false, errNeedMoreData
		}
		contentLength = int(clVal)
	}

	sepLen := len(headerSeparatorBuf)
	total := headerEnd + sepLen + contentLength
	if len(buf) < total {
		return nil, 0, false, errNeedMoreData
	}

	var body io.ReadCloser = http.NoBody
	if contentLength > 0 {
		body = io.NopCloser(bytes.NewReader(buf[headerEnd+sepLen : total]))
	}

	requestURI := target
	if requestURI == "" {
		requestURI = "/"
	}
	parsedURL, err := parseRequestURL(requestURI, header.Get("Host"))
	if err != nil {
		return nil, 0, false, err
	}

	req := &http.Request{
		Method:        method,
		Proto:         fmt.Sprintf("HTTP/%d.%d", maj, min),
		ProtoMajor:    maj,
		ProtoMinor:    min,
		Header:        header,
		Body:          body,
		ContentLength: int64(contentLength),
		Host:          header.Get("Host"),
		RequestURI:    requestURI,
	}
	req.URL = parsedURL
	req.Close = shouldCloseConnection(req, header)
	return req, total, req.Close, nil
}

func parseHTTPVersion(proto string) (int, int, error) {
	trimmed := strings.TrimSpace(proto)
	if !strings.HasPrefix(strings.ToUpper(trimmed), "HTTP/") {
		return 0, 0, fmt.Errorf("invalid proto: %s", proto)
	}
	version := strings.TrimPrefix(strings.ToUpper(trimmed), "HTTP/")
	segments := strings.SplitN(version, ".", 2)
	if len(segments) != 2 {
		return 0, 0, fmt.Errorf("invalid proto: %s", proto)
	}
	maj, err := strconv.Atoi(segments[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid proto: %s", proto)
	}
	min, err := strconv.Atoi(segments[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid proto: %s", proto)
	}
	if maj != 1 {
		return 0, 0, fmt.Errorf("unsupported http version: %s", proto)
	}
	return maj, min, nil
}

func parseRequestURL(target, host string) (*url.URL, error) {
	if target == "*" {
		return &url.URL{Path: "*"}, nil
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "*") {
		return url.ParseRequestURI(target)
	}
	if host == "" {
		return nil, fmt.Errorf("missing Host header")
	}
	return url.ParseRequestURI(target)
}

func shouldCloseConnection(req *http.Request, hdr http.Header) bool {
	if hasConnectionToken(hdr, "close") {
		return true
	}
	if req.ProtoMajor == 1 && req.ProtoMinor == 0 {
		return !hasConnectionToken(hdr, "keep-alive")
	}
	return false
}
