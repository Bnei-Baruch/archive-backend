package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

func CMSBanners(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		err := fmt.Errorf("id must be supplied")
		concludeRequestFile(c, "", NewBadRequestError(err))
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	filePattern := fmt.Sprintf("%sactive/banners/%%s-*", assets)
	fileNames, err := handleItemsRequest(filePattern, id)
	concludeRequestFiles(c, fileNames, err)
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
	filePattern := fmt.Sprintf("%sactive/sources/%s-%%s-%s/%s", assets, r.Uid, r.Uid, id)
	fileName, err := handleItemRequest(filePattern, r.Language)
	concludeRequestFile(c, fileName, err)
}

func CMSSourceIndex(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		err := fmt.Errorf("id must be supplied")
		concludeRequestFile(c, "", NewBadRequestError(err))
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	fileName := fmt.Sprintf("%sactive/sources/%s-en-%s/index.json", assets, id, id)
	j, err := ioutil.ReadFile(fileName)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	var m map[string]map[string]string
	//j = j[1:len(j)-1]
	j = []byte(strings.Replace(string(j), "\\n", "", -1))
	j = []byte(strings.Replace(string(j), "\\", "", -1))
	err = json.Unmarshal(j[1:len(j)-1], &m)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	c.JSON(http.StatusOK, m)
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

func handleItemsRequest(filePattern string, language string) ([]string, *HttpError) {
	for _, lang := range consts.I18N_LANG_ORDER[language] {
		pattern := fmt.Sprintf(filePattern, lang)
		if files, err := filepath.Glob(pattern); err == nil {
			return files, nil
		}
	}

	return nil, NewNotFoundError()
}

// responds with File(s) content or aborts the request with the given error.
func concludeRequestFiles(c *gin.Context, fileNames []string, err *HttpError) {
	if err != nil || len(fileNames) == 0 {
		err.Abort(c)
	}

	content := make([]string, len(fileNames))
	var (
		data []byte
		errr error
	)
	for _, file := range fileNames {
		if data, errr = os.ReadFile(file); errr != nil {
			c.String(http.StatusOK, "[]")
		}
		content = append(content, string(data))
	}
	c.String(http.StatusOK, "[%s]", strings.Join(content, ","))
}
