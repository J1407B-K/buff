package buff

import (
	"encoding/json"
	"io"
	"net/http"
)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
	params  []paramKV
	pbuf    [8]paramKV
	sw      statusWriter
	store   map[string]any

	Route string
}

type paramKV struct{ key, val string }

func (c *Context) Param(k string) string {
	for i := len(c.params) - 1; i >= 0; i-- {
		if c.params[i].key == k {
			return c.params[i].val
		}
	}
	return ""
}
func (c *Context) Set(k string, v any) {
	if c.store == nil {
		c.store = map[string]any{}
	}
	c.store[k] = v
}

func (c *Context) Get(k string) (any, bool) { v, ok := c.store[k]; return v, ok }

func (c *Context) Header(k, v string) *Context { c.Writer.Header().Set(k, v); return c }

func (c *Context) Text(code int, s string) error {
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
	}
	c.Writer.WriteHeader(code)
	_, err := io.WriteString(c.Writer, s)
	return err
}

func (c *Context) JSON(code int, v any) error {
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "application/json; charset=utf-8")
	}
	c.Writer.WriteHeader(code)
	enc := json.NewEncoder(c.Writer)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func (c *Context) Bind(v any) error { return json.NewDecoder(c.Request.Body).Decode(v) }
func (c *Context) Redirect(code int, url string) error {
	http.Redirect(c.Writer, c.Request, url, code)
	return nil
}
