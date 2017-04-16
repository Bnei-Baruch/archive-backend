package api

import (
	"gopkg.in/gin-gonic/gin.v1"
)

func SetupRoutes(router *gin.Engine) {
	router.GET("/collections", CollectionsHandler)
	router.POST("/collections", CollectionsHandler)
	router.GET("/content_units/:uid", ContentUnitsHandler)
	router.GET("/sources", SourcesHierarchyHandler)
	router.GET("/tags", TagsHierarchyHandler)
	router.GET("/search", SearchHandler)

	router.GET("/_recover", func(c *gin.Context) {
		panic("test recover")
	})
}
