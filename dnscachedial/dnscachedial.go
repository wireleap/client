// Copyright (c) 2022 Wireleap

package dnscachedial

import (
	"context"
	"net"
	"sync"
)

// Control is the type of a cache controller.
type Control struct {
	sync.Mutex
	cache map[string][]string
}

// New creates a new DNS cache.
func New() *Control { return &Control{cache: map[string][]string{}} }

// Cache explicitly adds an address to the DNS cache.
func (c *Control) Cache(ctx context.Context, addr string) (err error) {
	c.Lock()
	c.cache[addr], err = net.DefaultResolver.LookupHost(ctx, addr)
	c.Unlock()
	return
}

// Get retrieves the cached resolved addresses of addr.
func (c *Control) Get(addr string) (r []string) {
	c.Lock()
	r = c.cache[addr]
	c.Unlock()
	return
}

// Flush flushes the cache, removing all cached addresses.
func (c *Control) Flush() {
	c.Lock()
	for k, _ := range c.cache {
		delete(c.cache, k)
	}
	c.Unlock()
}

type DialCtxFunc func(context.Context, string, string) (net.Conn, error)

// Cover creates a new DNS caching DialCtxFunc from an original DialCtxFunc.
func (c *Control) Cover(orig DialCtxFunc) DialCtxFunc {
	return func(ctx context.Context, network string, hostport string) (_ net.Conn, err error) {
		c.Lock()
		defer c.Unlock()
		// host:port given but only host needs to be looked up/stored
		host, port, err := net.SplitHostPort(hostport)
		if err != nil {
			return
		}
		// cache new addresses
		var addrs []string
		if addrs = c.cache[host]; addrs == nil {
			if addrs, err = net.DefaultResolver.LookupHost(ctx, host); err != nil {
				return
			}
			c.cache[host] = addrs
		}
		// rotate address list & dial
		if len(addrs) > 1 {
			c.cache[host] = append(addrs[1:], addrs[0])
		}
		return orig(ctx, network, net.JoinHostPort(addrs[0], port))
	}
}
