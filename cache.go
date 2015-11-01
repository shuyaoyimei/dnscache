package main

import (
	"time"
	"github.com/miekg/dns"
	"errors"
	"github.com/golang/groupcache/lru"
)

type Mesg struct {
	Msg    *dns.Msg
	Expire time.Time
}

type Cache interface {
	Get(key string) (Msg *dns.Msg, err error)
	Set(key string, Msg *dns.Msg) error
	Remove(key string)
	Length() int
}

type MemoryCache struct {
	CacheStorage *lru.Cache
	Expire   time.Duration
	Maxcount int
}

func (c *MemoryCache) Get(key string) (*dns.Msg, error) {
	mesg, ok := c.CacheStorage.Get(key)
	if !ok {
		return nil, errors.New("Key not found")
	}
	msg := mesg.(Mesg)
	if msg.Expire.Before(time.Now()) {
		c.Remove(key)
		return nil, errors.New("Key expires")
	}

	return msg.Msg, nil

}

func (c *MemoryCache) Set(key string, msg *dns.Msg) error {

	expire := time.Now().Add(c.Expire)
	mesg := Mesg{msg, expire}
	c.CacheStorage.Add(key, mesg)
	return nil
}

func (c *MemoryCache) Remove(key string) {
	c.CacheStorage.Remove(key)
}


func (c *MemoryCache) Length() int {
	return c.CacheStorage.Len()
}

