package main

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// staticFiles holds the frontend, compiled into the binary so the service stays
// a single self-contained artifact and does not depend on a working directory.
//
//go:embed static
var staticFiles embed.FS

// serveIndex serves the single-page UI at GET /.
func serveIndex(c *gin.Context) {
	page, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the page"})
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", page)
}

// serveAsset serves one embedded file with the given content type.
func serveAsset(name, contentType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		content, err := staticFiles.ReadFile("static/" + name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, contentType, content)
	}
}
