package router

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xiusin/router/components/option"
)

type (
	Router struct {
		RouteCollection
		groups         map[string]*RouteCollection // 分组路由保存
		pool           *sync.Pool
		option         *option.Option
		recoverHandler Handler
	}

	// 定义路由处理函数类型
	Handler func(*Context)
)

var (
	shutdownBeforeHandler []func()
	errCodeCallHandler    = make(map[int]Handler)
)

const (
	Version = "dev 0.0.5"
	logo    = `
____  __.__            .__      __________               __                
\   \/  |__|__ __ _____|__| ____\______   \ ____  __ ___/  |_  ___________ 
 \     /|  |  |  /  ___|  |/    \|       _//  _ \|  |  \   ___/ __ \_  __ \
 /     \|  |  |  \___ \|  |   |  |    |   (  <_> |  |  /|  | \  ___/|  | \/
/___/\  |__|____/____  |__|___|  |____|_  /\____/|____/ |__|  \___  |__|   
      \_/            \/        \/       \/                        \/   	  Version: ` + Version
)

func RegisterOnInterrupt(handler func()) {
	shutdownBeforeHandler = append(shutdownBeforeHandler, handler)
}

// 注册
func RegisterErrorCodeHandler(code int, handler Handler) {
	if code != http.StatusOK {
		errCodeCallHandler[code] = handler
	}
}

// 实例化路由
// 如果传入nil 则使用默认配置
func NewRouter(opt *option.Option) *Router {
	r := &Router{
		option: opt,
		groups: map[string]*RouteCollection{},
		pool: &sync.Pool{
			New: func() interface{} {
				ctx := &Context{
					params:          NewParams(map[string]string{}), //保存路由参数
					middlewareIndex: -1,                             // 初始化中间件索引. 默认从0开始索引.
				}
				return ctx
			},
		},
		RouteCollection: RouteCollection{
			methodRoutes: initRouteMap(),
		},
		recoverHandler: Recover,
	}
	if r.option == nil {
		r.option = option.Default()
	}
	return r
}

func (r *Router) SetRecoverHandler(handler Handler) {
	if handler != nil {
		r.recoverHandler = handler
	}
}

// 匹配路由
func (r *Router) matchRoute(ctx *Context, urlParsed *url.URL) *RouteEntry {
	pathInfos := strings.Split(urlParsed.Path, urlSeparator)
	l := len(pathInfos)
	for i := 1; i <= l; i++ {
		p := strings.Join(pathInfos[:i], urlSeparator)
		route, ok := r.methodRoutes[ctx.Request().Method][p]
		if ok { // 直接匹配到路由
			if route.Method != ctx.Request().Method {
				continue
			}
			return route
		}
		// 在路由分组内查找
		group, ok := r.groups[p]
		if ok {
			path := urlSeparator + strings.Join(pathInfos[i:], urlSeparator)
			for routePath, route := range group.methodRoutes[ctx.Request().Method] {
				if routePath != path || route.Method != ctx.Request().Method {
					continue
				}
				route.ExtendsMiddleWare = group.middleWares
				return route
			}
		}
	}
	// 匹配正则规则
	for pattern, routes := range patternRoutes {
		reg := regexp.MustCompile(pattern)
		matched := reg.FindAllStringSubmatch(urlParsed.Path, -1)
		for _, route := range routes {
			if len(matched) == 0 || len(matched[0]) == 0 || route.Method != ctx.Request().Method {
				continue
			}
			subMatched := matched[0][1:]
			for k, param := range route.Param {
				ctx.Params().Set(param, subMatched[k])
			}
			route.ExtendsMiddleWare = r.middleWares
			return route
		}
	}
	return nil
}

// 处理静态文件夹 todo 这里会不会出现重复注册的问题
func (r *Router) Static(path, dir string) {
	r.GET(path, func(i *Context) {
		http.StripPrefix(
			strings.TrimSuffix(path, "*file"), http.FileServer(http.Dir(dir)),
		).ServeHTTP(i.Writer(), i.req)
	})
}

// 处理静态文件
func (r *Router) StaticFile(path, file string) {
	r.GET(path, func(c *Context) {
		http.ServeFile(c.Writer(), c.Request(), file)
	})
}

// 路由分组
func (r *Router) Group(prefix string, middleWares ...Handler) *RouteCollection {
	g := &RouteCollection{Prefix: prefix}
	g.methodRoutes = initRouteMap()
	g.middleWares = append(g.middleWares, middleWares...)
	r.groups[prefix] = g
	return g
}

// 针对全局的router引入中间件
func (r *Router) Use(middleWares ...Handler) *Router {
	r.middleWares = append(r.middleWares, middleWares...)
	return r
}

func (_ *Router) gracefulShutdown(srv *http.Server, quit <-chan os.Signal, done chan<- bool) {
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.SetKeepAlivesEnabled(false)
	if err := srv.Shutdown(ctx); err != nil {
		_ = fmt.Errorf("could not gracefully shutdown the server: %v\n", err)
	}
	for _, beforeHandler := range shutdownBeforeHandler {
		beforeHandler()
	}
	close(done)
}

// 启动服务
func (r *Router) Serve() {
	done, quit := make(chan bool, 1), make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	addr := r.option.Host + ":" + strconv.Itoa(r.option.Port)
	srv := &http.Server{
		ReadHeaderTimeout: r.option.TimeOut,
		WriteTimeout:      r.option.TimeOut,
		ReadTimeout:       r.option.TimeOut,
		IdleTimeout:       r.option.TimeOut,
		Addr:              addr,
		Handler:           http.TimeoutHandler(r, r.option.TimeOut, "Server Timeout"), // 超时函数, 但是无法阻止服务器端停止,内部耗时部分可以自行使用context.context控制
	}
	if r.option.Env == option.DevMode {
		fmt.Println(logo)
	}
	go r.gracefulShutdown(srv, quit, done)
	fmt.Println("server run on: http://" + addr)
	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		_ = fmt.Errorf("server was error: %s", err.Error())
	}
	<-done
}

// 处理总线
func (r *Router) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	c := r.pool.Get().(*Context)
	defer r.pool.Put(c)
	c.reset(res, req)
	c.app = r
	res.Header().Set("Server", r.option.ServerName)
	defer r.recoverHandler(c)
	r.dispatch(c, res, req)
}

// 有可处理函数
func (r *Router) handle(c *Context, urlParsed *url.URL) {
	route := r.matchRoute(c, urlParsed)
	if route != nil {
		//todo 是否可以设置自己设置解析表单
		if r.option.MaxMultipartMemory > 0 {
			if err := c.ParseForm(); err != nil {
				panic(err)
			}
		}
		c.setRoute(route)
		c.Next()
	} else {
		c.SetStatus(http.StatusNotFound)
		errCodeCallHandler[http.StatusNotFound](c)
	}
}

// 调度路由
func (r *Router) dispatch(c *Context, res http.ResponseWriter, req *http.Request) {
	urlParsed, _ := url.ParseRequestURI(req.RequestURI) // 解析地址参数
	r.handle(c, urlParsed)
}

// 初始化RouteMap
func initRouteMap() map[string]map[string]*RouteEntry {
	return map[string]map[string]*RouteEntry{
		http.MethodGet:     {},
		http.MethodPost:    {},
		http.MethodPut:     {},
		http.MethodHead:    {},
		http.MethodDelete:  {},
		http.MethodTrace:   {},
		http.MethodConnect: {},
		http.MethodPatch:   {},
	}
}
