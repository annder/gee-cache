package main

import (
	"fmt"
	"log"
	"sync"
)

// HTTP-SEVRER Group
type Group struct {
	name      string
	getter    Getter
	mainCache Cache
}

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

var (
	mu     sync.RWMutex // 读写锁
	groups = make(map[string]*Group)
)

// 获取组的名字
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: Cache{},
	}
	// TODO:这里有问题
	groups[name] = g
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	if _, ok := g.mainCache.Get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil // TODO: 这里有问题
	}
	return g.load(key)
}

// 加载缓存列表
func (g *Group) load(key string) (value ByteView, err error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value = ByteView{cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.Add(key, value)
}

// 返回生成的组。
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	defer mu.RUnlock()
	return g
}
