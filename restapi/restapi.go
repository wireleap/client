// Copyright (c) 2021 Wireleap

package restapi

import "net/http"

// api server stub
type T struct{}

func New() *T { return &T{} }

func (t *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
