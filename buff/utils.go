package buff

import (
	"net/http"
	"strings"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
	bytes  int
}

func (sw *statusWriter) WriteHeader(code int) {
	if sw.wrote {
		return
	}
	sw.status = code
	sw.wrote = true
	sw.ResponseWriter.WriteHeader(code)
}
func (sw *statusWriter) Write(p []byte) (int, error) {
	if !sw.wrote {
		sw.WriteHeader(http.StatusOK)
	}
	n, err := sw.ResponseWriter.Write(p)
	sw.bytes += n
	return n, err
}
func (sw *statusWriter) Status() int {
	if sw.wrote {
		return sw.status
	}
	return http.StatusOK
}
func (sw *statusWriter) BytesWritten() int { return sw.bytes }

func splitPath(p string) []string {
	if p == "/" || p == "" {
		return nil
	}
	i, j := 0, len(p)
	if p[0] == '/' {
		i++
	}
	for j > i && p[j-1] == '/' {
		j--
	}
	if i >= j {
		return nil
	}
	segs := make([]string, 0, 8)
	start := i
	for k := i; k < j; k++ {
		if p[k] == '/' {
			if k > start {
				segs = append(segs, p[start:k])
			}
			start = k + 1
		}
	}
	if start < j {
		segs = append(segs, p[start:j])
	}
	return segs
}

func normalize(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	if !strings.Contains(p, "//") && (len(p) == 1 || p[len(p)-1] != '/') {
		return p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if p != "/" && p[len(p)-1] == '/' {
		p = strings.TrimRight(p, "/")
	}
	return p
}
