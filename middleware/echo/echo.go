// Package atlasentecho is an Echo middleware that gates requests with an
// AtlaSent authorization check.
//
//	e := echo.New()
//	e.Use(jwtAuth)
//	e.Use(atlasentecho.Middleware(client, resolve))
package atlasentecho

import (
	"net/http"

	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/labstack/echo/v4"
)

// ResourceResolver maps an Echo context to an (action, resource, reqCtx) triple.
type ResourceResolver func(echo.Context) (string, atlasent.Resource, map[string]any, error)

// Middleware gates the downstream handler with a Check call. The Principal
// must be set on the request context upstream via atlasent.WithPrincipal.
func Middleware(client *atlasent.Client, resolve ResourceResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			principal, ok := atlasent.PrincipalFrom(c.Request().Context())
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthenticated")
			}
			action, resource, reqCtx, err := resolve(c)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			}
			decision, checkErr := client.Check(c.Request().Context(), atlasent.CheckRequest{
				Principal: principal,
				Action:    action,
				Resource:  resource,
				Context:   reqCtx,
			})
			if checkErr != nil && client.FailClosed {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "authorization service unavailable")
			}
			if !decision.Allowed {
				return c.JSON(http.StatusForbidden, map[string]string{
					"reason":    decision.Reason,
					"policy_id": decision.PolicyID,
				})
			}
			return next(c)
		}
	}
}
