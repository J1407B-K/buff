package buff

import (
	"bytes"
	"fmt"
	"io"
	"math"
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

	chunked, err := hasChunkedEncoding(header)
	if err != nil {
		return nil, 0, false, err
	}

	if chunked && header.Get("Content-Length") != "" {
		return nil, 0, false, fmt.Errorf("chunked request must not include Content-Length")
	}

	sepLen := len(headerSeparatorBuf)
	bodyStart := headerEnd + sepLen
	total := bodyStart
	var body io.ReadCloser = http.NoBody
	contentLength := int64(0)

	if chunked {
		data, consumed, trailers, err := parseChunkedBody(buf, bodyStart)
		if err != nil {
			return nil, 0, false, err
		}
		for k, vv := range trailers {
			for _, v := range vv {
				header.Add(k, v)
			}
		}
		total = consumed
		if len(data) > 0 {
			body = io.NopCloser(bytes.NewReader(data))
		}
		contentLength = -1
	} else {
		length := 0
		if cl := header.Get("Content-Length"); cl != "" {
			clVal, err := strconv.ParseInt(cl, 10, 64)
			if err != nil || clVal < 0 {
				return nil, 0, false, fmt.Errorf("invalid Content-Length: %q", cl)
			}
			if clVal > int64(len(buf)) {
				return nil, 0, false, errNeedMoreData
			}
			length = int(clVal)
		}

		total = bodyStart + length
		if len(buf) < total {
			return nil, 0, false, errNeedMoreData
		}

		if length > 0 {
			body = io.NopCloser(bytes.NewReader(buf[bodyStart:total]))
		}
		contentLength = int64(length)
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
	if chunked {
		req.TransferEncoding = []string{"chunked"}
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

func hasChunkedEncoding(hdr http.Header) (bool, error) {
	values := hdr.Values("Transfer-Encoding")
	if len(values) == 0 {
		return false, nil
	}
	var encodings []string
	for _, v := range values {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			token := strings.TrimSpace(strings.ToLower(p))
			if token == "" {
				continue
			}
			encodings = append(encodings, token)
		}
	}
	if len(encodings) == 0 {
		return false, fmt.Errorf("invalid Transfer-Encoding header")
	}
	chunked := false
	for i, enc := range encodings {
		switch enc {
		case "identity":
			continue
		case "chunked":
			if i != len(encodings)-1 {
				return false, fmt.Errorf("transfer-encoding %q not supported", enc)
			}
			chunked = true
		default:
			return false, fmt.Errorf("transfer-encoding %q not supported", enc)
		}
	}
	return chunked, nil
}

func parseChunkedBody(buf []byte, start int) ([]byte, int, http.Header, error) {
	i := start
	if i > len(buf) {
		return nil, 0, nil, errNeedMoreData
	}

	var body bytes.Buffer
	var trailers http.Header

	for {
		lineOffset := bytes.Index(buf[i:], crlfBytes)
		if lineOffset == -1 {
			return nil, 0, nil, errNeedMoreData
		}
		lineEnd := i + lineOffset
		sizeLine := string(buf[i:lineEnd])
		semi := strings.Index(sizeLine, ";")
		if semi >= 0 {
			sizeLine = sizeLine[:semi]
		}
		if sizeLine == "" {
			return nil, 0, nil, fmt.Errorf("invalid chunk size line")
		}
		chunkSize, err := strconv.ParseInt(sizeLine, 16, 64)
		if err != nil || chunkSize < 0 {
			return nil, 0, nil, fmt.Errorf("invalid chunk size: %q", sizeLine)
		}
		i = lineEnd + len(crlfBytes)

		if int64(len(buf)-i) < chunkSize+int64(len(crlfBytes)) {
			return nil, 0, nil, errNeedMoreData
		}

		if chunkSize > 0 {
			if chunkSize > math.MaxInt || i > len(buf)-int(chunkSize) {
				return nil, 0, nil, fmt.Errorf("invalid chunk size: %q", sizeLine)
			}
			body.Write(buf[i : i+int(chunkSize)])
			i += int(chunkSize)
		}

		if len(buf) < i+len(crlfBytes) {
			return nil, 0, nil, errNeedMoreData
		}
		if !bytes.Equal(buf[i:i+len(crlfBytes)], crlfBytes) {
			return nil, 0, nil, fmt.Errorf("invalid chunk terminator")
		}
		i += len(crlfBytes)

		if chunkSize == 0 {
			for {
				if i >= len(buf) {
					return nil, 0, nil, errNeedMoreData
				}
				lineOffset := bytes.Index(buf[i:], crlfBytes)
				if lineOffset == -1 {
					return nil, 0, nil, errNeedMoreData
				}
				if lineOffset == 0 {
					i += len(crlfBytes)
					break
				}
				line := buf[i : i+lineOffset]
				colon := bytes.IndexByte(line, ':')
				if colon <= 0 {
					return nil, 0, nil, fmt.Errorf("malformed trailer line: %q", string(line))
				}
				key := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(string(line[:colon])))
				val := strings.TrimSpace(string(line[colon+1:]))
				if trailers == nil {
					trailers = http.Header{}
				}
				trailers.Add(key, val)
				i += lineOffset + len(crlfBytes)
			}
			break
		}
	}

	return body.Bytes(), i, trailers, nil
}
