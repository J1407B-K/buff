package buff

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/valyala/bytebufferpool"
)

func TestParseHTTPRequestBasic(t *testing.T) {
	raw := "GET /hello HTTP/1.1\r\nHost: example.com\r\nUser-Agent: test\r\n\r\n"
	req, consumed, closeAfter, err := parseHTTPRequest([]byte(raw), 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed != len(raw) {
		t.Fatalf("expected consumed %d, got %d", len(raw), consumed)
	}
	if req.Method != http.MethodGet {
		t.Fatalf("expected method GET, got %s", req.Method)
	}
	if req.URL.Path != "/hello" {
		t.Fatalf("expected path /hello, got %s", req.URL.Path)
	}
	if closeAfter {
		t.Fatalf("expected keep-alive connection")
	}
}

func TestParseHTTPRequestContentLength(t *testing.T) {
	raw := "POST /data HTTP/1.1\r\nHost: example.com\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhelloextra"
	req, consumed, closeAfter, err := parseHTTPRequest([]byte(raw), 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	headLen := strings.Index(raw, "\r\n\r\n")
	expected := headLen + len("\r\n\r\n") + 5
	if consumed != expected {
		t.Fatalf("unexpected consumed size %d want %d", consumed, expected)
	}
	if b, err := io.ReadAll(req.Body); err != nil || string(b) != "hello" {
		t.Fatalf("unexpected body: %q err=%v", string(b), err)
	}
	_ = req.Body.Close()
	if !closeAfter {
		t.Fatalf("expected close connection")
	}
}

func TestGNetResponseWriterFinalize(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	pool := &bytebufferpool.Pool{}
	w := acquireGNetResponseWriter(pool)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatalf("write body: %v", err)
	}

	respBuf := pool.Get()
	respBuf.Reset()
	respBuf, closeAfter := w.finalize(req, false, respBuf)
	if closeAfter {
		t.Fatalf("expected connection kept alive")
	}
	resp := respBuf.String()
	if !strings.Contains(resp, "HTTP/1.1 201 Created") {
		t.Fatalf("unexpected status line: %s", resp)
	}
	if !strings.Contains(resp, "Content-Length: 2") {
		t.Fatalf("expected content length header, got: %s", resp)
	}
	if !strings.Contains(resp, "\r\n\r\nok") {
		t.Fatalf("expected body, got: %s", resp)
	}
	pool.Put(respBuf)
	releaseGNetResponseWriter(pool, w)
}

func TestParseHTTPRequestChunked(t *testing.T) {
	raw := "" +
		"POST /chunk HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"4\r\nWiki\r\n" +
		"5\r\npedia\r\n" +
		"0\r\n" +
		"\r\n"
	req, consumed, closeAfter, err := parseHTTPRequest([]byte(raw), 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed != len(raw) {
		t.Fatalf("expected consumed %d, got %d", len(raw), consumed)
	}
	if closeAfter {
		t.Fatalf("expected keep-alive connection")
	}
	if req.TransferEncoding == nil || len(req.TransferEncoding) != 1 || req.TransferEncoding[0] != "chunked" {
		t.Fatalf("expected chunked transfer encoding, got %#v", req.TransferEncoding)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "Wikipedia" {
		t.Fatalf("unexpected body %q", string(body))
	}
	if req.ContentLength != -1 {
		t.Fatalf("expected chunked content length -1, got %d", req.ContentLength)
	}
	_ = req.Body.Close()
}

func TestParseHTTPRequestChunkedWithTrailer(t *testing.T) {
	raw := "" +
		"POST /chunk HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"4\r\nWiki\r\n" +
		"0\r\n" +
		"\r\n" +
		"X-Custom: value\r\n" +
		"\r\n"
	req, consumed, _, err := parseHTTPRequest([]byte(raw), 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed != len(raw) {
		t.Fatalf("expected consumed %d, got %d", len(raw), consumed)
	}
	if req.Header.Get("X-Custom") != "value" {
		t.Fatalf("expected trailer promoted into header, got %q", req.Header.Get("X-Custom"))
	}
}
