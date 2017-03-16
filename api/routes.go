package api

import (
	"gopkg.in/gin-gonic/gin.v1"
)

func SetupRoutes(router *gin.Engine) {
	router.GET("/search", SearchHandler)

	router.GET("/_recover", func(c *gin.Context) {
		panic("test recover")
	})
}
