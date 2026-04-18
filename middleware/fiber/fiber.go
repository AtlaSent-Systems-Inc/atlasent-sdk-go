// Package atlasentfiber is a Fiber middleware that gates requests with an
// AtlaSent authorization check.
//
//	app := fiber.New()
//	app.Use(jwtAuth)
//	app.Use(atlasentfiber.Middleware(client, resolve))
//
// Fiber stores the Principal on the *fiber.Ctx user context, not the
// request context, so the resolver and upstream auth layers must use
// c.UserContext() / c.SetUserContext() when attaching / extracting it.
package atlasentfiber

import (
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/gofiber/fiber/v2"
)

// ResourceResolver maps a Fiber context to an (action, resource, reqCtx) triple.
type ResourceResolver func(*fiber.Ctx) (string, atlasent.Resource, map[string]any, error)

// Middleware gates the downstream handler with a Check call.
func Middleware(client *atlasent.Client, resolve ResourceResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		principal, ok := atlasent.PrincipalFrom(c.UserContext())
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
		}
		action, resource, reqCtx, err := resolve(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		decision, checkErr := client.Check(c.UserContext(), atlasent.CheckRequest{
			Principal: principal,
			Action:    action,
			Resource:  resource,
			Context:   reqCtx,
		})
		if checkErr != nil && client.FailClosed {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "authorization service unavailable"})
		}
		if !decision.Allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"reason":    decision.Reason,
				"policy_id": decision.PolicyID,
			})
		}
		return c.Next()
	}
}
