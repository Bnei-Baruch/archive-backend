package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"gopkg.in/gin-gonic/gin.v1"
)

const reasoningSearchLimitMessage = "Reasoning search limit reached. Please try again later."

// Trust only local/Docker proxies, not forwarded headers sent by public peers.
// If a CDN/load balancer is added, review the forwarding chain and trust only its
// verified proxy ranges; otherwise per-IP limits may group visitors by proxy IP.
var reasoningSearchTrustedProxyRanges = []net.IPNet{
	{IP: net.IP{127, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
	{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
	{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
}

func reasoningSearchTrustedProxy(ip net.IP) bool {
	for _, network := range reasoningSearchTrustedProxyRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func reasoningSearchClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return "unknown"
	}
	if reasoningSearchTrustedProxy(peer) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			chain := strings.Split(forwarded, ",")
			for i := len(chain) - 1; i >= 0; i-- {
				ip := net.ParseIP(strings.TrimSpace(chain[i]))
				if ip == nil {
					return peer.String()
				}
				if !reasoningSearchTrustedProxy(ip) || i == 0 {
					return ip.String()
				}
			}
		} else if ip := net.ParseIP(r.Header.Get("X-Real-IP")); ip != nil {
			return ip.String()
		}
	}
	return peer.String()
}

func admitReasoningSearchRun(c *gin.Context, runtime *llm.Runtime, parent context.Context) *llm.ReasoningRun {
	if runtime == nil || runtime.RunGuard == nil {
		NewInternalError(errors.New("reasoning run guard is not initialized")).Abort(c)
		return nil
	}
	run, retryAfter := runtime.RunGuard.TryStart(parent, reasoningSearchClientIP(c.Request))
	if run == nil {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		c.Abort()
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": reasoningSearchLimitMessage, "code": "reasoning_search_limit_reached",
		})
	}
	return run
}
