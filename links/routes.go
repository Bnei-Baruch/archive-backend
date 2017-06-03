package links

import (
	"gopkg.in/gin-gonic/gin.v1"
)

func SetupRoutes(router *gin.Engine) {
	router.GET("/:uid", FilesHandler)

}
