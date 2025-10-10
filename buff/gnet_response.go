package buff

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/valyala/bytebufferpool"
)

var gnetRespPool = sync.Pool{
	New: func() any {
		return &gnetResponseWriter{
			header: http.Header{},
		}
	},
}

type gnetResponseWriter struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        *bytebufferpool.ByteBuffer
	serverHdr   string
}

func acquireGNetResponseWriter(pool *bytebufferpool.Pool) *gnetResponseWriter {
	w := gnetRespPool.Get().(*gnetResponseWriter)
	if w.header == nil {
		w.header = http.Header{}
	}
	clearHeader(w.header)
	w.status = 0
	w.wroteHeader = false
	if w.body == nil {
		w.body = pool.Get()
	} else {
		w.body.Reset()
	}
	return w
}

func releaseGNetResponseWriter(pool *bytebufferpool.Pool, w *gnetResponseWriter) {
	if w.body != nil {
		w.body.Reset()
		pool.Put(w.body)
		w.body = nil
	}
	w.serverHdr = ""
	gnetRespPool.Put(w)
}

func (w *gnetResponseWriter) Header() http.Header { return w.header }

func (w *gnetResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(b)
}

func (w *gnetResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
}

func (w *gnetResponseWriter) finalize(req *http.Request, reqClose bool, out *bytebufferpool.ByteBuffer) (*bytebufferpool.ByteBuffer, bool) {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	body := w.body.Bytes()
	hdr := w.header.Clone()

	if _, ok := hdr["Server"]; !ok && w.serverHdr != "" {
		hdr.Set("Server", w.serverHdr)
	}
	if _, ok := hdr["Date"]; !ok {
		hdr.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}
	if _, ok := hdr["Content-Length"]; !ok {
		hdr.Set("Content-Length", strconv.Itoa(len(body)))
	}

	shouldClose := connectionCloseRequested(hdr)
	if !shouldClose {
		shouldClose = reqClose
	}
	if shouldClose {
		hdr.Set("Connection", "close")
	} else if req.ProtoMajor == 1 && req.ProtoMinor == 0 {
		if !hasConnectionToken(hdr, "keep-alive") {
			hdr.Set("Connection", "keep-alive")
		}
	}

	out.Reset()
	out.WriteString("HTTP/1.1 ")
	out.WriteString(strconv.Itoa(status))
	out.WriteByte(' ')
	out.WriteString(http.StatusText(status))
	out.WriteString(crlf)
	writeHeaderLines(out, hdr)
	out.WriteString(crlf)
	out.Write(body)
	return out, shouldClose
}
