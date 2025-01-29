package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sonichigo/letterpress/db"
	"github.com/sonichigo/letterpress/models"
)

func (h *Handler) helloWorld(c *gin.Context) {
	fmt.Printf("HelloWorld")
}

func (h *Handler) DeletePost(c *gin.Context) {
	var id int
	var err error
	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}
	err = h.DB.DeletePost(id)
	switch err {
	case db.ErrNoRecord:
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("could not find post with id: %d", id)})
		break
	case nil:
		c.JSON(http.StatusOK, gin.H{"data": map[string]string{"message": "post deleted"}})
		break
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		break
	}
}

func (h *Handler) GetPosts(c *gin.Context) {
	posts, err := h.DB.GetPosts()
	if err != nil {
		h.Logger.Err(err).Msg("Could not fetch posts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"data": posts})
	}
}

func (h *Handler) CreatePost(c *gin.Context) {
	var post models.Post
	if err := c.ShouldBindJSON(&post); err != nil {
		h.Logger.Err(err).Msg("could not parse request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %s", err.Error())})
		return
	}
	err := h.DB.SavePost(&post)
	if err != nil {
		h.Logger.Err(err).Msg("could not save post")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not save post: %s", err.Error())})
	} else {
		c.JSON(http.StatusCreated, gin.H{"post": post})
	}
}

func (h *Handler) UpdatePost(c *gin.Context) {
	var id int
	var post models.Post
	var err error
	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}
	if err = c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("could not parse request: %s", err.Error())})
		return
	}

	err = h.DB.UpdatePost(id, post)
	if err != nil {
		switch err {
		case db.ErrNoRecord:
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("could not find post with id: %d", id)})
		default:
			h.Logger.Err(err).Msg("could not update post")
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not update post: %s", err.Error())})
		}
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"post": post})
	}
}

func (h *Handler) GetPost(c *gin.Context) {
	var id int
	var err error
	var post models.Post
	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
	} else {
		post, err = h.DB.GetPostById(id)
		switch err {
		case db.ErrNoRecord:
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("could not find post with id: %d", id)})
			break
		case nil:
			c.JSON(http.StatusOK, gin.H{"data": post})
			break
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		}
	}
}

func (h *Handler) SearchPosts(c *gin.Context) {
	var query string
	if query, _ = c.GetQuery("q"); query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no search query present"})
		return
	}

	//body := `{"query" : { "match_all" : {}" }}`
	body := fmt.Sprintf(
		`{"query": {"multi_match": {"query": "%s", "fields": ["title", "body"]}}}`,
		query)
	res, err := h.ESClient.Search(
		h.ESClient.Search.WithContext(context.Background()),
		h.ESClient.Search.WithIndex("posts"),
		h.ESClient.Search.WithBody(strings.NewReader(body)),
		h.ESClient.Search.WithPretty(),
	)
	if err != nil {
		h.Logger.Err(err).Msg("elasticsearch error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer res.Body.Close()
	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			h.Logger.Err(err).Msg("error parsing the response body")
		} else {
			// Print the response status and error information.
			h.Logger.Err(fmt.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)).Msg("failed to search query")
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": e["error"].(map[string]interface{})["reason"]})
		return
	}

	h.Logger.Info().Interface("res", res.Status())

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		h.Logger.Err(err).Msg("elasticsearch error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": r["hits"]})
}
