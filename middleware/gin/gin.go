// Package atlasentgin is a Gin middleware that gates requests with an
// AtlaSent authorization check.
//
//	r := gin.Default()
//	r.Use(jwtAuth)                      // set Principal on ctx
//	r.Use(atlasentgin.Middleware(client, resolve))
//
// The Principal must be attached to the request context upstream with
// atlasent.WithPrincipal.
package atlasentgin

import (
	"github.com/atlasent-systems-inc/atlasent-sdk-go/atlasent"
	"github.com/gin-gonic/gin"
)

// ResourceResolver maps a Gin context to an (action, resource, reqCtx) triple.
type ResourceResolver func(*gin.Context) (string, atlasent.Resource, map[string]any, error)

// Middleware returns a gin.HandlerFunc that calls client.Check and aborts
// with 401/400/403/503 as appropriate. On allow, it forwards to the next
// handler.
func Middleware(client *atlasent.Client, resolve ResourceResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := atlasent.PrincipalFrom(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthenticated"})
			return
		}
		action, resource, reqCtx, err := resolve(c)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
			return
		}
		decision, checkErr := client.Check(c.Request.Context(), atlasent.CheckRequest{
			Principal: principal,
			Action:    action,
			Resource:  resource,
			Context:   reqCtx,
		})
		if checkErr != nil && client.FailClosed {
			c.AbortWithStatusJSON(503, gin.H{"error": "authorization service unavailable"})
			return
		}
		if !decision.Allowed {
			c.AbortWithStatusJSON(403, gin.H{
				"reason":    decision.Reason,
				"policy_id": decision.PolicyID,
			})
			return
		}
		c.Next()
	}
}
