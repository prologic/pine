package router

import (
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/xiusin/router/components/event"

	"github.com/julienschmidt/httprouter"
	"github.com/xiusin/router/components/option"
)

// 兼容 httprouter
type Httprouter struct {
	*base
	router        *httprouter.Router
	globalMws     []Handler            // 全局中间件
	mws           map[string][]Handler //路由中间件
	innerGroupMws map[string][]Handler // 分组中间件
	l             sync.RWMutex
	routes        map[string]*RouteEntry // 记录path与route的关系
}

var tempGroupPrefix = ""

func NewHttpRouter(opt *option.Option) *Httprouter {
	r := &Httprouter{
		router: httprouter.New(),
		base: &base{
			notFound:       func(ctx *Context) { ctx.Writer().Write([]byte(tpl404)) },
			pool:           &sync.Pool{New: func() interface{} { return NewContext(opt) }},
			option:         opt,
			recoverHandler: DefaultRecoverHandler,
		},
		innerGroupMws: make(map[string][]Handler),
		mws:           make(map[string][]Handler),
		routes:        make(map[string]*RouteEntry),
	}
	r.handler = r
	if r.option == nil {
		r.option = option.Default()
	}
	r.warpNotFoundHandler()
	r.warpRecoverHandler()
	return r
}

var _ IRouter = (*Httprouter)(nil)

//todo 为什么这里不能直接使用base的函数？？？？？？？？？？？？？
func (r *Httprouter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(res, req)
}

func (r *Httprouter) warpRecoverHandler() {
	if r.recoverHandler == nil {
		r.router.PanicHandler = nil
	} else {
		r.router.PanicHandler = func(writer http.ResponseWriter, request *http.Request, _ interface{}) {
			c, _ := r.resolveContext(writer, request, nil)
			r.recoverHandler(c)
		}
	}
}

func (r *Httprouter) SetRecoverHandler(handler Handler) {
	r.base.SetRecoverHandler(handler)
	r.warpRecoverHandler()
}

func (r *Httprouter) warpNotFoundHandler() {
	if r.notFound == nil {
		r.router.NotFound = nil
	} else {
		r.router.NotFound = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			c, _ := r.resolveContext(writer, request, nil)
			r.notFound(c)
		})
	}
	r.router.MethodNotAllowed = r.router.NotFound //框架自实现不允许出现MethodNotAllowed
}
func (r *Httprouter) SetNotFound(handler Handler) {
	r.base.SetNotFound(handler)
	r.warpNotFoundHandler() //设置默认notFound
}

// 针对全局的router引入中间件
func (r *Httprouter) Use(middleWares ...Handler) {
	r.globalMws = append(r.globalMws, middleWares...)
}

//不支持嵌套
func (r *Httprouter) Group(prefix string, callback func(router *Httprouter), middleWares ...Handler) {
	r.l.Lock()
	defer r.l.Unlock()
	tempGroupPrefix = prefix //赋值
	r.innerGroupMws[prefix] = middleWares
	callback(r)
	tempGroupPrefix = "" //置空
}


func (r *Httprouter) registerMwsToRoutePath(path string, mws []Handler) {
	r.mws[path] = mws
}

func (r *Httprouter) Static(path, dir string) {
	r.GET(path, func(i *Context) {
		http.StripPrefix(
			strings.TrimSuffix(path, "*filepath"), http.FileServer(http.Dir(dir)),
		).ServeHTTP(i.Writer(), i.Request())
	})
}

// 处理静态文件
func (r *Httprouter) StaticFile(path, file string) {
	r.GET(path, func(c *Context) { http.ServeFile(c.Writer(), c.Request(), file) })
}

func (r *Httprouter) resolveContext(writer http.ResponseWriter, request *http.Request, params httprouter.Params) (*Context, []string) {
	ctx := r.pool.Get().(*Context)
	ctx.Reset(writer, request)
	var pk []string
	for i := range params {
		pk = append(pk, params[i].Key)
		ctx.Params().Set(params[i].Key, params[i].Value)
	}
	writer.Header().Set("Server", r.option.GetServerName())
	return ctx, pk
}

func (r *Httprouter) warpHandle(path string, handle Handler, mws []Handler) httprouter.Handle {
	r.registerMwsToRoutePath(path, mws)
	r.l.Lock()

	route := &RouteEntry{
		ExtendsMiddleWare: r.globalMws, IsPattern: false,
		Pattern: "", OriginStr: path, Handle: handle,
	}
	for k := range r.innerGroupMws { // 追加分组中间件
		if strings.Contains(path, k) {
			route.ExtendsMiddleWare = append(route.ExtendsMiddleWare, r.innerGroupMws[k]...)
			break
		}
	}
	r.routes[path] = route
	r.l.Unlock()
	return func(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
		ctx, pk := r.resolveContext(writer, request, nil)
		defer r.recoverHandler(ctx)
		_, _ = event.Trigger(event.EventRequestBefore, ctx)
		route := r.routes[path]
		route.Middleware, route.Param = r.mws[path], pk
		ctx.setRoute(route)
		ctx.Next()
		_, _ = event.Trigger(event.EventRequestAfter, ctx)
	}
}

func (r *Httprouter) AddRoute(method, path string, handle Handler, mws ...Handler) *RouteEntry {
	r.router.Handle(method, tempGroupPrefix+path, r.warpHandle(tempGroupPrefix+path, handle, mws))
	return nil
}

func (r *Httprouter) GetPrefix() string {
	return tempGroupPrefix
}

// 处理控制器注册的方式
func (r *Httprouter) Handle(c IController) {
	refVal, refType := reflect.ValueOf(c), reflect.TypeOf(c)
	r.autoRegisterControllerRoute(r, refVal, refType, c)
}

func (r *Httprouter) GET(path string, handle Handler, mws ...Handler) *RouteEntry {
	r.AddRoute(http.MethodGet, path, handle, mws...)
	return nil
}

func (r *Httprouter) POST(path string, handle Handler, mws ...Handler) *RouteEntry {
	r.AddRoute(http.MethodPost, path, handle, mws...)
	return nil
}

func (r *Httprouter) OPTIONS(path string, handle Handler, mws ...Handler) *RouteEntry {
	r.AddRoute(http.MethodOptions, path, handle, mws...)
	return nil
}

func (r *Httprouter) HEAD(path string, handle Handler, mws ...Handler) *RouteEntry {
	r.AddRoute(http.MethodHead, path, handle, mws...)
	return nil
}

func (r *Httprouter) PUT(path string, handle Handler, mws ...Handler) *RouteEntry {
	r.AddRoute(http.MethodPut, path, handle, mws...)
	return nil
}

func (r *Httprouter) DELETE(path string, handle Handler, mws ...Handler) *RouteEntry {
	r.AddRoute(http.MethodDelete, path, handle, mws...)
	return nil
}
