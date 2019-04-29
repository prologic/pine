package core

import (
	"context"
	"fmt"
	"github.com/xiusin/router/core/components/di"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/mholt/binding"
	"github.com/unrolled/render"
)

type Context struct {
	req             *http.Request       // 请求对象
	params          map[string]string   // 路由参数
	res             http.ResponseWriter // 响应对象
	stopped         bool                // 是否停止传播中间件
	route           *Route              // 当前context匹配到的路由
	di              di.BuilderInf
	middlewareIndex int            // 中间件起始索引
	render          *render.Render // 模板渲染
	session         sessions.Store
	app             *Router
	status          int
}

// 开放几个API 获取 app 的只读行为

//func (c *Context) GetApp() *Router {
//	return c.app
//}

func (c *Context) SetDI(builder di.BuilderInf) {
	c.di = builder
}

func (c *Context) GetDI() di.BuilderInf {
	return c.di
}

// 重置Context对象
func (c *Context) Reset(res http.ResponseWriter, req *http.Request) {
	c.req = req
	c.res = res
	c.middlewareIndex = -1
	c.route = nil
	c.stopped = false
	c.status = http.StatusOK
	c.params = map[string]string{}
}

// 设置模板渲染 (后期改为interface)
func (c *Context) setRenderer(r *render.Render) {
	c.render = r
}

// 获取请求
func (c *Context) Request() *http.Request {
	return c.req
}

// 设置路由参数
func (c *Context) SetParam(key, value string) {
	c.params[key] = value
}

// 获取路由参数
func (c *Context) GetParam(key string) string {
	value, _ := c.params[key]
	return value
}

// 获取路由参数,如果为空字符串则返回 defaultVal
func (c *Context) GetParamDefault(key, defaultVal string) string {
	val := c.GetParam(key)
	if val != "" {
		return val
	}
	return defaultVal
}

// 获取响应
func (c *Context) Writer() http.ResponseWriter {
	return c.res
}

// 重定向
func (c *Context) Redirect(url string, statusHeader ...int) {
	if len(statusHeader) == 0 {
		statusHeader[0] = http.StatusFound
	}
	http.Redirect(c.res, c.req, url, statusHeader[0])
}

// 获取命名参数内容
func (c *Context) GetRoute(name string) *Route {
	r, _ := namedRoutes[name]
	return r
}

// 记录中间件索引位置
func (c *Context) handlerIndex() {
	c.middlewareIndex++
}

// 执行下个中间件
func (c *Context) Next() {
	if c.IsStopped() == true {
		return
	}
	c.middlewareIndex++
	middlewares := c.route.ExtendsMiddleWare
	middlewares = append(middlewares, c.route.Middleware...)
	length := len(middlewares)
	if length > c.middlewareIndex {
		idx := c.middlewareIndex
		middlewares[c.middlewareIndex](c)
		if length == idx {
			c.route.Handle(c)
			return
		}
	} else {
		c.route.Handle(c)
	}
}

// 设置当前处理路由对象
func (c *Context) setRoute(route *Route) {
	c.route = route
}

// 判断中间件是否停止
func (c *Context) IsStopped() bool {
	return c.stopped
}

// 停止中间件执行 即接下来的中间件以及handler会被忽略.
func (c *Context) Stop() {
	c.stopped = true
}

// 获取当前路由对象
func (c *Context) getRoute() *Route {
	return c.route
}

// 附加数据的context (可以装载附加组件)
func (c *Context) Set(key string, value interface{}) {
	c.req.WithContext(context.WithValue(c.req.Context(), key, value))
}

// 获取附带数据
func (c *Context) Get(key string) interface{} {
	return c.req.Context().Value(key)
}

// 获取模板渲染对象
func (c *Context) GetRenderer() *render.Render {
	return c.render
}

// 渲染data
func (c *Context) Data(v string) error {
	return c.render.Data(c.Writer(), http.StatusOK, []byte(v))
}

// 渲染html
func (c *Context) HTML(name string, binding interface{}, htmlOpt ...render.HTMLOptions) error {
	return c.render.HTML(c.Writer(), http.StatusOK, name, binding)
}

// 渲染json
func (c *Context) JSON(v interface{}) error {
	return c.render.JSON(c.Writer(), http.StatusOK, v)
}

// 渲染jsonp
func (c *Context) JSONP(callback string, v interface{}) error {
	return c.render.JSONP(c.Writer(), http.StatusOK, callback, v)
}

// 渲染text
func (c *Context) Text(v string) error {
	return c.render.Text(c.Writer(), http.StatusOK, v)
}

// 渲染xml
func (c *Context) XML(v interface{}) error {
	return c.render.XML(c.Writer(), http.StatusOK, v)
}

// 发送file
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer(), c.Request(), filepath)
}

// 获取cookie
func (c *Context) GetCookie(name string) (cookie string, err error) {
	cok, err := c.req.Cookie(name)
	if err == nil {
		cookie = cok.Value
	}
	return
}

// 设置cookie
func (c *Context) SetCookie(name, value string, maxAge int) {
	cookie := &http.Cookie{
		Name:   name,
		Value:  value,
		MaxAge: maxAge,
	}
	c.req.AddCookie(cookie)
}

// 绑定表单数据
func (c *Context) Bind(req *http.Request, formData binding.FieldMapper) error {
	return binding.Bind(req, formData)
}

// 判断是不是ajax请求
func (c *Context) IsAjax() bool {
	return c.req.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// 判断是不是Get请求
func (c *Context) IsGet() bool {
	return c.req.Method == http.MethodGet
}

// 判断是不是Post请求
func (c *Context) IsPost() bool {
	return c.req.Method == http.MethodPost
}

func (c *Context) Abort(statusCode int, msg string) {
	c.SetStatus(statusCode)
	if c.app.option.ErrorHandler != nil {
		if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError {
			c.app.option.ErrorHandler.Error40x(c, msg)
		} else if statusCode >= http.StatusInternalServerError {
			c.app.option.ErrorHandler.Error50x(c, msg)
			panic(msg)
		}
	}
}

func (c *Context) GetToken() string {
	r := rand.Int()
	t := time.Now().UnixNano()
	token := fmt.Sprintf("%d%d", r, t)
	c.SetCookie("csrf_token", token, 2*60)
	c.Set("csrf_token", token)
	return token
}

// 设置状态码
func (c *Context) SetStatus(statusCode int) {
	c.status = statusCode
	c.res.WriteHeader(statusCode)
}

func (c *Context) Status() int {
	return c.status
}
