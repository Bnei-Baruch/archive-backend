package api

import (
	"errors"
	"github.com/spf13/viper"
	"gopkg.in/gin-gonic/gin.v1"
	"os"
)

func CMSPerson(c *gin.Context) {
	id := c.Query("id")
	fbLang := c.DefaultQuery("fbLang", "en")
	lang := c.DefaultQuery("lang", fbLang)

	assets := viper.GetString("cms.assets")

	file := assets + "persons/persons-" + id + "-" + lang + "-html"
	var err error
	if _, err = os.Stat(file); err == nil {
		c.File(file)
		return
	}
	file = assets + "persons/" + id + "-" + fbLang + "-html"
	if _, err = os.Stat(file); err == nil {
		c.File(file)
		return
	}
	_ = c.AbortWithError(404, err)
}

func CMSBanner(c *gin.Context) {
	fbLang := c.DefaultQuery("fbLang", "en")
	lang := c.DefaultQuery("lang", fbLang)

	assets := viper.GetString("cms.assets")

	file := assets + "banners/banner-" + lang
	var err error
	if _, err = os.Stat(file); err == nil {
		c.File(file)
		return
	}
	file = assets + "banners/banner-" + fbLang
	if _, err = os.Stat(file); err == nil {
		c.File(file)
		return
	}
	_ = c.AbortWithError(404, err)
}

func CMSAsset(c *gin.Context) {
	path := c.Param("path")
	//log.Infof("%s\n", path)
	assets := viper.GetString("cms.assets")
	file := assets + "images" + path
	var err error
	if _, err = os.Stat(file); err != nil {
		_ = c.AbortWithError(404, errors.New("No such file"))
		return
	}
	mode := viper.GetString("server.mode")
	if mode == "release" {
		c.Header("X-Accel-Redirect", file)
	} else {
		c.File(file)
	}

	return
}

func CMSTopic(c *gin.Context) {
}
