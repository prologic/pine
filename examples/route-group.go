package main

import (
	"fmt"
	"router/core"
	"runtime"
	"time"
)

func main() {
	handler := core.NewRouter()
	handler.GET("/", func(context *core.Context) {
		time.Sleep(5 * time.Second)
		_, _ = context.Writer().Write([]byte("hello world"))
	})

	handler.GET("/:name/*action", func(context *core.Context) {
		_, _ = context.Writer().Write(
			[]byte(fmt.Sprintf("%s %s",
				context.GetParamDefault("name", "xiusin"),
				context.GetParamDefault("action", "coding")),
			))
	})

	g := handler.Group("/api/:version")
	{
		g.GET("/user/login", func(context *core.Context) {
			_, _ = context.Writer().Write([]byte(context.Request().URL.Path))
		})
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	handler.Serve("0.0.0.0:9999")
}