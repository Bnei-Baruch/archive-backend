package utils

import (
	"strings"

	"gopkg.in/gin-gonic/gin.v1"
)

func ResolveScheme(c *gin.Context) string {
	r := c.Request
	switch {
	case r.Header.Get("X-Forwarded-Proto") == "https":
		return "https"
	case r.URL.Scheme == "https":
		return "https"
	case r.TLS != nil:
		return "https"
	case strings.HasPrefix(r.Proto, "HTTPS"):
		return "https"
	default:
		return "http"
	}
}

func ResolveHost(c *gin.Context) (host string) {
	r := c.Request
	switch {
	case r.Header.Get("X-Forwarded-Host") != "":
		return r.Header.Get("X-Forwarded-Host")
	case r.Header.Get("X-Host") != "":
		return r.Header.Get("X-Host")
	case r.Host != "":
		return r.Host
	case r.URL.Host != "":
		return r.URL.Host
	default:
		return ""
	}
}
