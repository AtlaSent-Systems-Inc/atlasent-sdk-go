package atlasentecho_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasenttest"
	atlasentecho "github.com/atlasent-systems-inc/atlasent-sdk-go/middleware/echo"
	"github.com/labstack/echo/v4"
)

func resolve(echo.Context) (string, atlasent.Resource, map[string]any, error) {
	return "read", atlasent.Resource{ID: "r", Type: "doc"}, nil, nil
}

func newApp(t *testing.T, decision func(*atlasenttest.Server)) (*echo.Echo, *atlasenttest.Server) {
	fake := atlasenttest.NewServer(t)
	decision(fake)
	client, _ := atlasent.New("k", atlasent.WithBaseURL(fake.URL))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(atlasent.WithPrincipal(c.Request().Context(), atlasent.Principal{ID: "u"})))
			return next(c)
		}
	})
	e.Use(atlasentecho.Middleware(client, resolve))
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	return e, fake
}

func TestEchoAllow(t *testing.T) {
	e, _ := newApp(t, func(s *atlasenttest.Server) { s.On("read").Allow() })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEchoDeny(t *testing.T) {
	e, _ := newApp(t, func(s *atlasenttest.Server) { s.On("read").Deny("nope") })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
