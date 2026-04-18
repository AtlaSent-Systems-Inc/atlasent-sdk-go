package atlasentfiber_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasenttest"
	atlasentfiber "github.com/atlasent-systems-inc/atlasent-sdk-go/middleware/fiber"
	"github.com/gofiber/fiber/v2"
)

func resolve(*fiber.Ctx) (string, atlasent.Resource, map[string]any, error) {
	return "read", atlasent.Resource{ID: "r", Type: "doc"}, nil, nil
}

func newApp(t *testing.T, setup func(*atlasenttest.Server)) *fiber.App {
	fake := atlasenttest.NewServer(t)
	setup(fake)
	client, _ := atlasent.New("k", atlasent.WithBaseURL(fake.URL))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.SetUserContext(atlasent.WithPrincipal(c.UserContext(), atlasent.Principal{ID: "u"}))
		return c.Next()
	})
	app.Use(atlasentfiber.Middleware(client, resolve))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func do(t *testing.T, app *fiber.App) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

func TestFiberAllow(t *testing.T) {
	app := newApp(t, func(s *atlasenttest.Server) { s.On("read").Allow() })
	if resp := do(t, app); resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestFiberDeny(t *testing.T) {
	app := newApp(t, func(s *atlasenttest.Server) { s.On("read").Deny("nope") })
	if resp := do(t, app); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}
