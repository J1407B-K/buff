package buff

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterNormalizesDynamicPaths(t *testing.T) {
	r := NewRouter()
	err := r.Handle(http.MethodGet, "/foo/:id", func(c *Context) {
		if got := c.Param("id"); got != "123" {
			t.Fatalf("expected param id=123, got %q", got)
		}
		if c.Route != "/foo/:id" {
			t.Fatalf("expected route template /foo/:id, got %q", c.Route)
		}
		_ = c.Text(http.StatusOK, "ok")
	})
	if err != nil {
		t.Fatalf("register route: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/foo//123/", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestRouterContextRouteReset(t *testing.T) {
	r := NewRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	ctx := r.getCtx(rec, req)
	ctx.Route = "stale"
	r.putCtx(ctx)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)

	ctx2 := r.getCtx(rec2, req2)
	if ctx2.Route != "" {
		t.Fatalf("expected empty route on pooled context, got %q", ctx2.Route)
	}
	r.putCtx(ctx2)
}
