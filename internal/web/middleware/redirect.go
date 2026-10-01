package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RedirectMiddleware(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Keep the most specific legacy path first so /xui/API never
		// takes the shorter /xui redirect.
		redirects := [][2]string{
			{"panel/API", "panel/api"},
			{"xui/API", "panel/api"},
			{"xui", "panel"},
		}

		requestPath := c.Request.URL.Path
		for _, redirect := range redirects {
			from, to := redirect[0], redirect[1]
			from, to = basePath+from, basePath+to
			if len(requestPath) < len(from) || !strings.HasPrefix(requestPath, from) {
				continue
			}
			if len(requestPath) > len(from) && requestPath[len(from)] != '/' {
				continue
			}
			newPath := to + requestPath[len(from):]
			if c.Request.URL.RawQuery != "" {
				newPath += "?" + c.Request.URL.RawQuery
			}
			c.Redirect(http.StatusMovedPermanently, newPath)
			c.Abort()
			return
		}

		c.Next()
	}
}
