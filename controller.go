package router

import (
	"github.com/xiusin/router/components/di"
	"reflect"
	"sync"
	"unsafe"

	"github.com/xiusin/router/components/di/interfaces"
)

type (
	Controller struct {
		ctx  *Context
		sess interfaces.SessionInf
		once sync.Once
	}

	// 控制器接口定义
	ControllerInf interface {
		Ctx() *Context
		Render() *Render
		Logger() interfaces.LoggerInf
		Session() interfaces.SessionInf
	}

	// 控制器路由映射注册接口
	ControllerRouteMappingInf interface {
		GET(path string, handle string, mws ...Handler)
		POST(path string, handle string, mws ...Handler)
		PUT(path string, handle string, mws ...Handler)
		HEAD(path string, handle string, mws ...Handler)
		DELETE(path string, handle string, mws ...Handler)
		ANY(path string, handle string, mws ...Handler)
	}

	// 控制器映射路由
	controllerMappingRoute struct {
		r *RouteCollection
		c ControllerInf
	}
)

func (c *Controller) Ctx() *Context {
	return c.ctx
}

func (c *Controller) Session() interfaces.SessionInf {
	var err error
	c.once.Do(func() {
		c.sess, err = c.ctx.SessionManger().Session(c.ctx.Request(), c.ctx.Writer())
		if err != nil {
			panic(err)
		}
	})
	return c.sess
}

func (c *Controller) Render() *Render {
	return c.ctx.Render()
}

func (c *Controller) Logger() interfaces.LoggerInf {
	return c.ctx.Logger()
}

func (c *Controller) AfterAction() {
	if c.sess != nil {
		if err := c.sess.Save(); err != nil {
			c.Logger().Error("save session is error", err)
		}
	}
}

func newUrlMappingRoute(r *RouteCollection, c ControllerInf) *controllerMappingRoute {
	return &controllerMappingRoute{r: r, c: c}
}

func (u *controllerMappingRoute) warpControllerHandler(method string, c ControllerInf) Handler {
	refValCtrl := reflect.ValueOf(c)
	return func(context *Context) {
		c := reflect.New(refValCtrl.Elem().Type()) // 利用反射构建变量得到value值
		rs := reflect.Indirect(c)
		rf := rs.FieldByName("ctx") // 利用unsafe设置ctx的值
		ptr := unsafe.Pointer(rf.UnsafeAddr())
		*(**Context)(ptr) = context
		u.autoRegisterService(&c)
		// 判断是否存在BeforeAction， 执行前置操作
		if c.MethodByName("BeforeAction").IsValid() {
			c.MethodByName("BeforeAction").Call([]reflect.Value{})
		}
		c.MethodByName(method).Call([]reflect.Value{})
		// 判断是否存在AfterAction， 执行后置操作
		if c.MethodByName("AfterAction").IsValid() {
			c.MethodByName("AfterAction").Call([]reflect.Value{})
		}
	}
}

func (u *controllerMappingRoute) autoRegisterService(val *reflect.Value) {
	e := val.Type().Elem()
	fieldNum := e.NumField()
	for i := 0; i < fieldNum; i++ {
		serviceName := e.Field(i).Tag.Get("service")
		fieldName := e.Field(i).Name
		if serviceName == "" || fieldName == "Controller" /**忽略内嵌控制器字段的tag内容**/ {
			continue
		}
		service, err := di.Get(serviceName)
		if err != nil {
			panic("自动解析服务：" + serviceName + "失败")
		}
		val.Elem().FieldByName(fieldName).Set(reflect.ValueOf(service))
	}
}

func (u *controllerMappingRoute) GET(path, method string, mws ...Handler) {
	u.r.GET(path, u.warpControllerHandler(method, u.c), mws...)
}

func (u *controllerMappingRoute) POST(path, method string, mws ...Handler) {
	u.r.POST(path, u.warpControllerHandler(method, u.c), mws...)
}

func (u *controllerMappingRoute) PUT(path, method string, mws ...Handler) {
	u.r.PUT(path, u.warpControllerHandler(method, u.c), mws...)
}

func (u *controllerMappingRoute) HEAD(path, method string, mws ...Handler) {
	u.r.HEAD(path, u.warpControllerHandler(method, u.c), mws...)
}

func (u *controllerMappingRoute) DELETE(path, method string, mws ...Handler) {
	u.r.DELETE(path, u.warpControllerHandler(method, u.c), mws...)
}

func (u *controllerMappingRoute) ANY(path, method string, mws ...Handler) {
	u.r.ANY(path, u.warpControllerHandler(method, u.c), mws...)
}
