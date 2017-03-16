package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/olivere/elastic.v5"
)

func SearchHandler(c *gin.Context) {
	text := c.Query("text")
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Can't search for an empty text",
		})
		return
	}

	page := 0
	pageQ := c.Query("page")
	if pageQ != "" {
		var err error
		page, err = strconv.Atoi(pageQ)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Illegal value provided for 'page' parameter: %s", pageQ),
			})
			return
		}
	}

	res, err := handleSearch(c.MustGet("ES_CLIENT").(*elastic.Client), "mdb_collections", text, page)
	if err != nil {
		c.Error(err).SetType(gin.ErrorTypePrivate)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": err.Error()})
	}

	c.JSON(http.StatusOK, res)
}

func handleSearch(esc *elastic.Client, index string, text string, from int) (*elastic.SearchResult, error) {
	q := elastic.NewNestedQuery("content_units",
		elastic.NewMultiMatchQuery(text, "content_units.names.*", "content_units.descriptions.*"))

	h := elastic.NewHighlight().HighlighQuery(q)

	return esc.Search().
		Index(index).
		Query(q).
		Highlight(h).
		From(from).
		Do(context.TODO())
}
