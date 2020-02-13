// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package sessions

import (
	"net/http"
)

type ISessionManager interface {
	Session(*http.Request, http.ResponseWriter) (ISession, error)
}

type ISessionConfig interface {
	GetCookieName() string
	GetCookiePath() string
	GetMaxAge() int
	GetHttpOnly() bool
	GetSecure() bool
}

type ISessionStore interface {
	GetConfig() ISessionConfig
	Read(string, interface{}) error
	Save(string, interface{}) error
	Clear(string) error
}

type ISession interface {
	Set(string, string) error
	Get(string) (string, error)
	AddFlush(string, string) error
	Remove(string) error
	Save() error
	Clear() error
}
