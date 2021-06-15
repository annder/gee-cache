package main

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// 传入到里面的group
type CallGroup struct {
	mu sync.Mutex
	m  map[string]*call
}

// 这个函数就是限制疯狂打在mysql上。
func (g *CallGroup) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		// 等他执行完毕
		c.wg.Wait()
		return c.val, c.err
	}

	c := new(call)
	c.wg.Add(1)

	g.m[key] = c
	// 执行完毕之后就释放锁
	defer g.mu.Unlock()

	c.val, c.err = fn()

	c.wg.Done()
	g.mu.Lock()
	delete(g.m, key)
	defer g.mu.Unlock()

	return c.val, c.err
}
