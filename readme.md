# 分布式缓存

## LRU 算法

LRU 认为，如果数据最近被访问过，那么将来被访问的概率也会更高。LRU 算法的实现非常简单，维护一个队列，如果某条记录被访问了，则移动到队尾，那么队首则是最近最少访问的数据，淘汰该条记录即可。

在`go`语言的实现是，维护一个数组链表，然后通过添加和删除的方式，进行操作。

数据结构如下：

```go
type Cache struct {
    maxBytes int64
    nbytes int64
    ll *list.List // 链表
    cache map[string]*list.Element
    OnEvicted func (key string ,value Value)
}

type entry struct {
    key string
    value Value
}

type Value interface {
    Len() int
}
```

在`Cache`这个`struct`当中，`ll`是链表，而`maxBytes`是最大的链表数，`nbytes`是正在用的内存，`cache`代表了当时的键值对，然后实现了一个方法，获取当前链表的长度。

创建一个`cache`：

```go
func NewCache (maxBytes int64 , onEvicted func (string ,Value)) *Cache{
    return &Cache{
        maxBytes:maxBytes,
        ll : list.New(),
        cache : make (map[string]*list.Element),
        OnEvicted : onEvicted,
    }
}
```

### 新建一个缓存

添加缓存值就是添加一个缓存链表，写法如下：

```go
func (c *Cache) Add(key string ,value Value) {
    if ele , ok := c.cache[key] ; ok { // 如果缓存中有这个值
            c.ll.MoveToFront(elem) // 放到最前端
        kv := ele.Value.(*entry)
        c.nbytes += int64(value.Len()) - int64(kv.value.Len()) // 删掉当前值的长度和其他的长度
        kv.value = value // 当前的值，直接等于它
    } else {
        ele := c.ll.PushFront() // 直接推到最前面
        c.cache [key] = ele
        c.nbytes += int64(len(key)) + int64(value.Len()) // 当前存储的长度添加一。
    }
    // 移除最后一个，如果溢出了最大的长度话
    for c.maxBytes != 0 && c.maxBytes < c.nbytes {
        c.RemoveOldst()
    }
}
```

上面的例子里面有一个`RemoveOld`函数，其功能是移除最少访问的节点，方法如下：

```go
func (c *Cache ) RemoveOldest (){
    // 获取最后一位
     ele := c.ll.Back()
    if ele != nil {
        c.ll.Remove(ele) // 删除它
        kv := ele.Value.(*entry)
        delete(c.cache , kv.key) // 删除这个缓存
        c.nbytes -= int(len(kv.key)) + int64(kv.value.Len()) // 将正存储的长度减一
        if c.OnEvicted != nil {
            c.OnEvicted (kv.key , kv.value) // 被驱逐的键值
        }
    }
}
```

### 获取一个元素

添加一个元素，按照 lru 算法，它是被推到第一个链表元素里面的。

```go
func (c *Cache) Get (key string ,value Value) (value Value ,ok bool) {
    if ele, ok := c.cache[key] ; ok {
        c.ll.MoveToFront(ele) // 最新访问的，所以就推到前面
        kv := ele.Value.(*entry)
        return kv.Value,true
    }
    return
}
```

## hash 一致性

为了解决多个携程获取数据导致冲突，我们添加一个 hash 数来解决这个问题，而且每次获取这个缓存值，会造成浪费。

直接实现一个`hashMap`。

```go
// Hash maps bytes to uint32
type Hash func(data []byte) uint32

// Map constains all hashed keys
type Map struct {
    hash     Hash
    replicas int
    keys     []int // Sorted
    hashMap  map[int]string
}

// New creates a Map instance
func New(replicas int, fn Hash) *Map {
    m := &Map{
        replicas: replicas,
        hash:     fn,
       hashMap:  make(map[int]string),
    }
    if m.hash == nil {
        m.hash = crc32.ChecksumIEEE
    }
    return m
}

func (m *Map) Add(keys ...string) {
    for _, key := range keys {
        for i := 0; i < m.replicas; i++ {
            hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
            m.keys = append(m.keys, hash)
            m.hashMap[hash] = key
        }
    }
    sort.Ints(m.keys)
}

func (m *Map) Get(key string) string {
    if len(m.keys) == 0 {
        return ""
    }

    hash := int(m.hash([]byte(key)))
    // Binary search for appropriate replica.
        idx := sort.Search(len(m.keys), func(i int) bool {
        return m.keys[i] >= hash
    })

    return m.hashMap[m.keys[idx%len(m.keys)]]
}
```

这样就有效的减少了空间的浪费，从而获取`key`的速度更快了。而每次创建一个

## 防止缓存击穿

当我们获取`cache`的时候，并不知道这个`cache`到底会不会存在。如果是在单机的情况下不会出现，只用判断它是否存在，而在并发的模式下就不同了。

因为它会抢占资源，而且纤程不知道它删除掉了，所以在做并发的时候，会有意识的保护这个资源。

创建一个局部等待方式，将它作为`struct`保存起来，还有`Group`也是，有意地保存这个方法。

```go
import "sync"

type call struct {
    wg sync.WaitGroup
    val interface{}
    err error
}

type Group struct {
    mu sync.Mutex
    m map[string]*call
}
```

接着我们将他们“锁”起来，以便在携程抢占资源，实现非常简单，就是将`map`里面的值锁一下，然后等待释放。

```go
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
```

