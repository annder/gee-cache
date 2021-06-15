package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
)

const (
	defaultBasePath = "/_geecache/" // 默认基本路径
	defaultReplicas = 50            // 默认副本数
)

//  HTTP 客户端
type httpGetter struct {
	baseURL string
}

// 获取远程服务器
func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v%v",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)
	res, err := http.Get(u)

	if err != nil {
		return nil, err
	}
	// 等待关闭
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s:", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return nil, fmt.Errorf("reading response body : %v", err)
	}
	return bytes, nil
}

//  HTTP  池
type RequestHTTPPool struct {
	self       string
	basePath   string
	mu         sync.Mutex // 池锁
	peers      *Map
	httpGetter map[string]*httpGetter
}

// 设置本地服务器的缓存
func (p *RequestHTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 创建hashmap
	p.peers = NewHashMap(defaultReplicas, nil)
	p.httpGetter = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetter[peer] = &httpGetter{peer + p.basePath}
	}
}

// 选择一个守卫
func (p *RequestHTTPPool) PickerPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		log.Println("Picker peer", peer)
		return p.httpGetter[peer], true
	}
	return nil, false
}

