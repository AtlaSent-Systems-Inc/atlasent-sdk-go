package atlasentgin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasenttest"
	atlasentgin "github.com/atlasent-systems-inc/atlasent-sdk-go/middleware/gin"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func resolve(*gin.Context) (string, atlasent.Resource, map[string]any, error) {
	return "read", atlasent.Resource{ID: "r", Type: "doc"}, nil, nil
}

func TestGinMiddleware(t *testing.T) {
	fake := atlasenttest.NewServer(t)
	fake.On("read").Allow()
	client, _ := atlasent.New("k", atlasent.WithBaseURL(fake.URL))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(atlasent.WithPrincipal(c.Request.Context(), atlasent.Principal{ID: "u"}))
		c.Next()
	})
	r.Use(atlasentgin.Middleware(client, resolve))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGinMiddlewareDeny(t *testing.T) {
	fake := atlasenttest.NewServer(t)
	fake.On("read").Deny("nope")
	client, _ := atlasent.New("k", atlasent.WithBaseURL(fake.URL))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(atlasent.WithPrincipal(c.Request.Context(), atlasent.Principal{ID: "u"}))
		c.Next()
	})
	r.Use(atlasentgin.Middleware(client, resolve))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestGinMiddlewareUnauth(t *testing.T) {
	client, _ := atlasent.New("k")
	r := gin.New()
	r.Use(atlasentgin.Middleware(client, resolve))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}
