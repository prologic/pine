// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package pine

import "strconv"

type Params struct {
	data map[string]string
}

func newParams() *Params {
	return &Params{
		data: map[string]string{},
	}
}

func (c *Params) reset() {
	c.data = make(map[string]string)
}

func (c *Params) Set(key, value string) {
	c.data[key] = value
}

func (c *Params) Get(key string) string {
	value, _ := c.data[key]
	return value
}

func (c *Params) GetDefault(key, defaultVal string) string {
	val := c.Get(key)
	if val != "" {
		return val
	}
	return defaultVal
}

func (c *Params) GetInt(key string, defaultVal ...int) (val int, err error) {
	val, err = strconv.Atoi(c.Get(key))
	if err != nil && len(defaultVal) > 0 {
		val, err = defaultVal[0], nil
	}
	return
}

func (c *Params) GetInt64(key string, defaultVal ...int64) (val int64, err error) {
	val, err = strconv.ParseInt(c.Get(key), 10, 64)
	if err != nil && len(defaultVal) > 0 {
		val, err = defaultVal[0], nil
	}
	return
}

func (c *Params) GetFloat64(key string, defaultVal ...float64) (val float64, err error) {
	val, err = strconv.ParseFloat(c.Get(key), 64)
	if err != nil && len(defaultVal) > 0 {
		val, err = defaultVal[0], nil
	}
	return
}
