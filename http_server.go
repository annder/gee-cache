package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const basePath = "/_geecache/"

// HTTPpool
type HTTPPool struct {
	self     string
	basePath string
}

//  新建一个HTTPPool
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{self, basePath}
}

// 日志文件
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// Server 服务器
func (p *HTTPPool) ServerHTTP(w http.ResponseWriter, r *http.Request) {
	// 如果没有前缀
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path:" + r.URL.Path)
	}

	p.Log("%s %s", r.Method, r.URL.Path)
	//  切割basePATH
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)

	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 组名
	groupName := parts[0]
	// 键名
	key := parts[1]

	group := GetGroup(groupName)

	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Centent-Type", "application/octet-stream")
	w.Write(view.ByteSlice())
}
