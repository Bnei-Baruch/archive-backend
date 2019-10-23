package api

import (
	"fmt"
	"os"

	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type CMSParams struct {
	Assets string
	Mode   string
}

func CMSPerson(c *gin.Context) {
	var r BaseRequest
	if c.Bind(&r) != nil {
		return
	}
	id := c.Param("id")
	if id == "" {
		err := fmt.Errorf("id must be supplied")
		concludeRequestFile(c, "", NewBadRequestError(err))
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	filePattern := fmt.Sprintf("%sactive/persons/%s-%%s", assets, id)
	fileName, err := handleItemRequest(filePattern, r.Language)
	concludeRequestFile(c, fileName, err)
}

func CMSBanner(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		err := fmt.Errorf("id must be supplied")
		concludeRequestFile(c, "", NewBadRequestError(err))
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	filePattern := fmt.Sprintf("%sactive/banners/%%s", assets)
	fileName, err := handleItemRequest(filePattern, id)
	concludeRequestFile(c, fileName, err)
}

func CMSSource(c *gin.Context) {
	type SourceRequest struct {
		BaseRequest
		Uid string `json:"uid" form:"uid"`
		//Uid string `json:"uid" form:"uid" binding:"len=8"`
	}

	var r SourceRequest
	if c.Bind(&r) != nil {
		return
	}
	id := c.Param("id")
	if id == "" {
		err := fmt.Errorf("id must be supplied")
		concludeRequestFile(c, "", NewBadRequestError(err))
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	filePattern := fmt.Sprintf("%sactive/sources/%s/%s-%%s-%s", assets, r.Uid, r.Uid, id)
	fileName, err := handleItemRequest(filePattern, r.Language)
	concludeRequestFile(c, fileName, err)
}

func CMSImage(c *gin.Context) {
	path := c.Param("path")

	assets := c.MustGet("CMS").(*CMSParams).Assets
	fileName, err := handleImageRequest(path, assets)
	concludeRequestFile(c, fileName, err)
}

func CMSTopics(c *gin.Context) {
}

func handleImageRequest(path string, assets string) (string, *HttpError) {
	var err error

	fileName := fmt.Sprintf("%sactive/images%s", assets, path)
	if _, err = os.Stat(fileName); err != nil {
		return "", NewNotFoundError()
	}

	return fileName, nil
}

func handleItemRequest(filePattern string, language string) (string, *HttpError) {
	var err error

	for _, lang := range consts.I18N_LANG_ORDER[language] {
		file := fmt.Sprintf(filePattern, lang)
		if _, err = os.Stat(file); err == nil {
			return file, nil
		}
	}

	return "", NewNotFoundError()
}

// responds with File content or aborts the request with the given error.
func concludeRequestFile(c *gin.Context, fileName string, err *HttpError) {
	mode := c.MustGet("CMS").(*CMSParams).Mode

	if err == nil {
		if mode == "release" {
			c.Header("X-Accel-Redirect", fileName)
		} else {
			c.File(fileName)
		}
	} else {
		err.Abort(c)
	}
}
