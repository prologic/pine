// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package redis

import (
	"github.com/xiusin/pine/cache"

	redisgo "github.com/gomodule/redigo/redis"
)


type PineRedis struct {
	ttl    int
	pool   *redisgo.Pool
}

func New(pool *redisgo.Pool, ttl int) *PineRedis {
	b := PineRedis{
		ttl:    ttl,
		pool: pool,
	}
	return &b
}

func (r *PineRedis) Pool() *redisgo.Pool {
	return r.pool
}

func (r *PineRedis) Get(key string) ([]byte, error) {
	client := r.pool.Get()
	s, err := redisgo.Bytes(client.Do("GET", key))
	_ = client.Close()
	return s, err
}

func (r *PineRedis) GetWithUnmarshal(key string, receiver interface{}) error {
	data, err := r.Get(key)
	if err != nil {
		return err
	}
	err = cache.DefaultTranscoder.UnMarshal(data, receiver)
	return err
}


func (r *PineRedis) Set(key string, val []byte, ttl ...int) error {
	params := []interface{}{key, val}
	if len(ttl) == 0 {
		ttl = []int{r.ttl}
	}
	if ttl[0] > 0 {
		params = append(params, "EX", ttl[0])
	}
	client := r.pool.Get()
	_, err := client.Do("SET", params...)
	_ = client.Close()
	return err
}

func (r *PineRedis) SetWithMarshal(key string, structData interface{}, ttl ...int) error {
	data, err := cache.DefaultTranscoder.Marshal(structData)
	if err != nil {
		return  err
	}
	return r.Set(key, data, ttl...)
}


func (r *PineRedis) Delete(key string) error {
	client := r.pool.Get()
	_, err := client.Do("DEL", key)
	_ = client.Close()
	return err
}

func (r *PineRedis) Exists(key string) bool {
	client := r.pool.Get()
	isKeyExit, _ := redisgo.Bool(client.Do("EXISTS", key))
	_ = client.Close()
	if isKeyExit {
		return true
	}
	return false
}
