package buff

import (
	"net/http"
	"strings"

	"github.com/valyala/bytebufferpool"
)

func writeHeaderLines(buf *bytebufferpool.ByteBuffer, hdr http.Header) {
	for k, vv := range hdr {
		for _, v := range vv {
			buf.WriteString(k)
			buf.WriteString(": ")
			buf.WriteString(v)
			buf.WriteString(crlf)
		}
	}
}

func clearHeader(h http.Header) {
	for k := range h {
		delete(h, k)
	}
}

func connectionCloseRequested(hdr http.Header) bool {
	return hasConnectionToken(hdr, "close")
}

func hasConnectionToken(hdr http.Header, token string) bool {
	for _, v := range hdr.Values("Connection") {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			if strings.EqualFold(strings.TrimSpace(p), token) {
				return true
			}
		}
	}
	return false
}
