package main

import (
	"fmt"
	"github.com/xiusin/router/core"
	_ "github.com/xiusin/router/core/components/cache/adapters/redis"
	"github.com/xiusin/router/core/components/di"
	"github.com/xiusin/router/core/components/helper"
)

type Field struct {
	Name string
}

type MyController struct {
	//todo 从DI中反射出来类型
	core.Controller
	F1 Field
	F2 *Field
}


// 优先执行此函数执行映射
func (m *MyController) UrlMapping(r *core.RouteGroup) {
	r.GET("/get/hello", m.GetHello)
}

func (m *MyController) GetHello(c *core.Context) {
	_, _ = c.Writer().Write([]byte("Hello world Get"))
}

func (m *MyController) PostHello(c *core.Context) {
	_, _ = c.Writer().Write([]byte("Hello world Post"))
}



func main() {

	di.Set(helper.GetTypeName(&Field{}), func(builder di.BuilderInf) (i interface{}, e error) {
		return &Field{Name: "ref"}, nil
	}, true)

	di.Set(helper.GetTypeName(Field{}), func(builder di.BuilderInf) (i interface{}, e error) {
		return Field{Name: "no ref"}, nil
	}, true)

	handler := core.NewRouter(nil)
	g := handler.Group("/api")
	a := new(MyController)
	g.Handle(a)
	fmt.Println(a.F2)
	fmt.Println(a.F1)
	handler.Serve()
}