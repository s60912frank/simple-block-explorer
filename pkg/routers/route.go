package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func StubHandler (c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"params": c.Params,
	})
}