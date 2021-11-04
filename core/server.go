package core

import (
	"github.com/beego/beego/v2/adapter/logs"
	"github.com/gin-gonic/gin"
)

var Server *gin.Engine

func init() {
	Server = gin.New()
}

var Tail = "--来自快乐星球"

func RunServer() {
	if sillyGirl.GetBool("enable_http_server", false) == false {
		return
	}
	Server.GET("/", func(c *gin.Context) {
		c.String(200, Tail)
	})
	gin.SetMode(gin.ReleaseMode)
	logs.Info("开启httpserver----0.0.0.0:" + sillyGirl.Get("port", "8080"))
	Server.Run("0.0.0.0:" + sillyGirl.Get("port", "8080"))
}
