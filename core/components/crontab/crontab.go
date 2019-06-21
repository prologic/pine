package crontab

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// https://github.com/robfig/cron
// 表示cron表的Crontab结构

var alias = make(map[string]Entry)

type (
	Crontab struct {
		ticker *time.Ticker // 任务打点器
		jobs   []Entry      //任务列表
	}

	// job数据结构
	Entry struct {
		// 根据表达式 解析到每个点的时间结构
		sec       map[int]struct{}
		min       map[int]struct{}
		hour      map[int]struct{}
		day       map[int]struct{}
		month     map[int]struct{}
		dayOfWeek map[int]struct{}

		fn   interface{}
		args []interface{}
	}

	// 每次打点的时间结构
	tick struct {
		sec       int
		min       int
		hour      int
		day       int
		month     int
		dayOfWeek int
	}
)

func init() {
	AddAlias("@everysec", "* * * * * *")
}

// 初始化并且返回一个任务表
func New() *Crontab {
	c := &Crontab{
		ticker: time.NewTicker(time.Second), // 每分钟打点
	}
	return c
}

// 添加任务到任务表
// 如果发生以下信息则返回错误：
// * Cron语法无法解析
// * fn 不是一个函数
// * 提供的参数不能匹配数量或者无法匹配参数类型
func (c *Crontab) AddJob(schedule string, fn interface{}, args ...interface{}) error {
	var err error
	entry, ok := alias[schedule]
	if !ok {
		entry, err = parseSchedule(schedule)
		if err != nil {
			return err
		}
	}

	if fn == nil || reflect.ValueOf(fn).Kind() != reflect.Func {
		return fmt.Errorf("Cron job must be func()")
	}

	fnType := reflect.TypeOf(fn)
	if len(args) != fnType.NumIn() {
		return fmt.Errorf("Number of func() params and number of provided params doesn't match")
	}

	for i := 0; i < fnType.NumIn(); i++ {
		a := args[i]
		t1 := fnType.In(i)
		t2 := reflect.TypeOf(a)

		if t1 != t2 {
			if t1.Kind() != reflect.Interface {
				return fmt.Errorf("Param with index %d shold be `%s` not `%s`", i, t1, t2)
			}
			if !t2.Implements(t1) {
				return fmt.Errorf("Param with index %d of type `%s` doesn't implement interface `%s`", i, t2, t1)
			}
		}
	}

	// 检查完成， 添加到任务表内
	entry.fn = fn
	entry.args = args
	c.jobs = append(c.jobs, entry)
	return nil
}

//对AddJob添加panic
func (c *Crontab) MustAddJob(schedule string, fn interface{}, args ...interface{}) {
	if err := c.AddJob(schedule, fn, args...); err != nil {
		panic(err)
	}
}

func (c *Crontab) Start() {
	for t := range c.ticker.C {
		c.startScheduled(t)
	}
}

// 关闭任务表的调度
func (c *Crontab) Shutdown() {
	c.ticker.Stop()
}

// 清除cron表中的所有作业
func (c *Crontab) Clear() {
	c.jobs = []Entry{}
}

// 运行调度
func (c *Crontab) startScheduled(t time.Time) {
	tick := getTick(t)
	for _, entry := range c.jobs {
		if entry.tick(tick) {
			go entry.run()
		}
	}
}

// 使用反射执行任务有些异常是无法预知的，需要添加检查函数
func (entry Entry) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Crontab error", r)
		}
	}()
	v := reflect.ValueOf(entry.fn)
	rargs := make([]reflect.Value, len(entry.args))
	for i, a := range entry.args {
		rargs[i] = reflect.ValueOf(a)
	}
	v.Call(rargs)
}

// 判断当前时间点是否符合执行条件
func (entry Entry) tick(t tick) bool {
	fmt.Println(entry, t)
	if _, ok := entry.sec[t.sec]; !ok {
		return false
	}

	if _, ok := entry.min[t.min]; !ok {
		return false
	}

	if _, ok := entry.hour[t.hour]; !ok {
		return false
	}

	// 每天或每周
	_, day := entry.day[t.day]
	_, dayOfWeek := entry.dayOfWeek[t.dayOfWeek]
	if !day && !dayOfWeek {
		return false
	}

	if _, ok := entry.month[t.month]; !ok {
		return false
	}

	return true
}

// 用于解析schedyle字符串的regexp
var (
	matchSpaces = regexp.MustCompile("\\s+")
	matchN      = regexp.MustCompile("(.*)/(\\d+)")
	matchRange  = regexp.MustCompile("^(\\d+)-(\\d+)$")
)

// 从字符串创建填充时间的任务结构数据并且启动， 或者语法错误失败
func parseSchedule(s string) (entry Entry, err error) {
	s = matchSpaces.ReplaceAllLiteralString(s, " ")
	parts := strings.Split(s, " ")
	if len(parts) != 6 {
		return Entry{}, errors.New("Schedule string must have five components like * * * * * *")
	}

	entry.sec, err = parsePart(parts[0], 0, 59)
	if err != nil {
		return entry, err
	}

	entry.min, err = parsePart(parts[1], 0, 59)
	if err != nil {
		return entry, err
	}

	entry.hour, err = parsePart(parts[2], 0, 23)
	if err != nil {
		return entry, err
	}

	entry.day, err = parsePart(parts[3], 1, 31)
	if err != nil {
		return entry, err
	}

	entry.month, err = parsePart(parts[4], 1, 12)
	if err != nil {
		return entry, err
	}

	entry.dayOfWeek, err = parsePart(parts[5], 0, 6)
	if err != nil {
		return entry, err
	}

	//  day/dayOfWeek 组合
	switch {
	case len(entry.day) < 31 && len(entry.dayOfWeek) == 7: // day set, but not dayOfWeek, clear dayOfWeek
		entry.dayOfWeek = make(map[int]struct{})
	case len(entry.dayOfWeek) < 7 && len(entry.day) == 31: // dayOfWeek set, but not day, clear day
		entry.day = make(map[int]struct{})
	default:
		// both day and dayOfWeek are * or both are set, use combined
		// i.e. don't do anything here
	}

	return entry, nil
}

// parsePart parse individual schedule part from schedule string
func parsePart(s string, min, max int) (map[int]struct{}, error) {

	r := make(map[int]struct{}, 0)

	// wildcard pattern
	if s == "*" {
		for i := min; i <= max; i++ {
			r[i] = struct{}{}
		}
		return r, nil
	}

	// */2 1-59/5 pattern
	if matches := matchN.FindStringSubmatch(s); matches != nil {
		localMin := min
		localMax := max
		if matches[1] != "" && matches[1] != "*" {
			if rng := matchRange.FindStringSubmatch(matches[1]); rng != nil {
				localMin, _ = strconv.Atoi(rng[1])
				localMax, _ = strconv.Atoi(rng[2])
				if localMin < min || localMax > max {
					return nil, fmt.Errorf("Out of range for %s in %s. %s must be in range %d-%d", rng[1], s, rng[1], min, max)
				}
			} else {
				return nil, fmt.Errorf("Unable to parse %s part in %s", matches[1], s)
			}
		}
		n, _ := strconv.Atoi(matches[2])
		for i := localMin; i <= localMax; i += n {
			r[i] = struct{}{}
		}
		return r, nil
	}

	// 1,2,4  or 1,2,10-15,20,30-45 pattern
	parts := strings.Split(s, ",")
	for _, x := range parts {
		if rng := matchRange.FindStringSubmatch(x); rng != nil {
			localMin, _ := strconv.Atoi(rng[1])
			localMax, _ := strconv.Atoi(rng[2])
			if localMin < min || localMax > max {
				return nil, fmt.Errorf("Out of range for %s in %s. %s must be in range %d-%d", x, s, x, min, max)
			}
			for i := localMin; i <= localMax; i++ {
				r[i] = struct{}{}
			}
		} else if i, err := strconv.Atoi(x); err == nil {
			if i < min || i > max {
				return nil, fmt.Errorf("Out of range for %d in %s. %d must be in range %d-%d", i, s, i, min, max)
			}
			r[i] = struct{}{}
		} else {
			return nil, fmt.Errorf("Unable to parse %s part in %s", x, s)
		}
	}

	if len(r) == 0 {
		return nil, fmt.Errorf("Unable to parse %s", s)
	}

	return r, nil
}

// getTick 结构化时间
func getTick(t time.Time) tick {
	return tick{
		sec:       t.Second(),
		min:       t.Minute(),
		hour:      t.Hour(),
		day:       t.Day(),
		month:     int(t.Month()),
		dayOfWeek: int(t.Weekday()),
	}
}

// 打印成列表
func (c *Crontab) PrintList() {
}

// 字符串别名

func AddAlias(aliasName string, pattern string) {
	entry, err := parseSchedule(pattern)
	if err == nil {
		alias[aliasName] = entry
	}
}

//打印文档
func PrintDoc() {
	fmt.Println(`符合crontab表达式: 
-----------------------------------------------------------------------------
			*	 *     *     *     *     *
			^	 ^     ^     ^     ^     ^
			|	 |     |     |     |     |
			|	 |     |     |     |     +----- day of week (0-6) (Sunday=0)
			|	 |     |     |     +------- month (1-12)
			|	 |     |     +--------- day of month (1-31)
			|	 |     +----------- hour (0-23)
			|	 +------------- min (0-59)
			+------------- sec (0-59)
	`)
}