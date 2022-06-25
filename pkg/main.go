package main

import (
	"portto-explorer/pkg/routers"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/ping", routers.StubHandler)
	r.GET("/blocks", routers.StubHandler)
	r.GET("/blocks/:id", routers.StubHandler)
	r.GET("/transaction/:txHash", routers.StubHandler)
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
