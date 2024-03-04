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
	fileName, err := handleItemRequest(filePattern, BaseRequestToContentLanguages(r))
	concludeRequestFile(c, fileName, err)
}

func CMSBanner(c *gin.Context) {
	var r BaseRequest
	if c.Bind(&r) != nil {
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	filePattern := fmt.Sprintf("%sactive/banners/%%s", assets)
	fileName, err := handleItemRequest(filePattern, BaseRequestToContentLanguages(r))
	concludeRequestFile(c, fileName, err)
}

func CMSBanners(c *gin.Context) {
	var r BaseRequest
	if c.Bind(&r) != nil {
		return
	}

	assets := c.MustGet("CMS").(*CMSParams).Assets
	filePattern := fmt.Sprintf("%sactive/banners/%%s-*", assets)
	fileNames, err := handleItemsRequest(filePattern, BaseRequestToContentLanguages(r))
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
	fileName, err := handleItemRequest(filePattern, BaseRequestToContentLanguages(r.BaseRequest))
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

func handleItemRequest(filePattern string, contentLanguages []string) (string, *HttpError) {
	for _, lang := range contentLanguages {
		file := fmt.Sprintf(filePattern, lang)
		if _, err := os.Stat(file); err == nil {
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

func handleItemsRequest(filePattern string, contentLanguages []string) ([]string, *HttpError) {
	for _, lang := range contentLanguages {
		pattern := fmt.Sprintf(filePattern, lang)
		if files, err := filepath.Glob(pattern); err == nil && len(files) > 0 {
			return files, nil
		}
	}

	return nil, NewNotFoundError()
}

// responds with File(s) content or aborts the request with the given error.
func concludeRequestFiles(c *gin.Context, fileNames []string, err *HttpError) {
	if err != nil {
		err.Abort(c)
		return
	}

	if len(fileNames) == 0 {
		NewNotFoundError().Abort(c)
		return
	}

	content := make([]string, len(fileNames))
	var (
		data []byte
		errr error
	)
	for idx, file := range fileNames {
		if data, errr = os.ReadFile(file); errr != nil {
			NewHttpError(http.StatusNotFound, errr, gin.ErrorTypePublic).Abort(c)
			return
		}
		content[idx] = string(data)
	}
	c.String(http.StatusOK, "[%s]", strings.Join(content, ","))
}
