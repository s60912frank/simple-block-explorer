package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func WrapperErr(f func(c *gin.Context) error) func(c *gin.Context) {
	return func(c *gin.Context) {
		if err := f(c); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			c.Abort()
		}
	}
}
