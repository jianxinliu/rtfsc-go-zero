package syncx

import "sync"

/**
 * [rtfsc]
 * 主题: singleflight.go
 * 摘要: 相同的任务，只需要一个人执行完成，剩下的享受成果即可
 * 功能: 多个协程执行同一个任务时，只需要一个执行成功，其余的共享结果即可
 * 应用: 高并发查询某个缓存未命中的记录，则只需允许一个协程去 db 查询即可，再将结果存到缓存，其余的直接获取缓存即可
 * [end]
 */

type (
	// SingleFlight lets the concurrent calls with the same key to share the call result.
	// For example, A called F, before it's done, B called F. Then B would not execute F,
	// and shared the result returned by F which called by A.
	// The calls with the same key are dependent, concurrent calls share the returned values.
	// A ------->calls F with key<------------------->returns val
	// B --------------------->calls F with key------>returns val
	SingleFlight interface {
		Do(key string, fn func() (any, error)) (any, error)
		DoEx(key string, fn func() (any, error)) (any, bool, error)
	}

	// [rtfsc]
	// 表示一次任务
	// wg  用于等待任务执行结束
	// val 任务执行的返回值
	// err 任务执行的错误
	// [end]
	call struct {
		wg  sync.WaitGroup
		val any
		err error
	}

	// [rtfsc]
	// 管理所有任务
	// calls 按 key 存放任务。实际上只存放一个执行中的任务，其他相同 key 的任务进来后，会直接返回已经存在的任务的引用
	// lock  用于在增减任务时加锁，毕竟 map 不是并发安全的
	// [end]
	flightGroup struct {
		calls map[string]*call
		lock  sync.Mutex
	}
)

// NewSingleFlight returns a SingleFlight.
func NewSingleFlight() SingleFlight {
	return &flightGroup{
		calls: make(map[string]*call),
	}
}

// [rtfsc]
// 执行一个任务
// 返回任务执行的结果。结果来自于：
// 1. 首次触发，调用 fn 得到
// 2. 非首次触发，共享首次触发时的结果
// [end]
func (g *flightGroup) Do(key string, fn func() (any, error)) (any, error) {
	c, done := g.createCall(key)
	if done {
		return c.val, c.err
	}

	g.makeCall(c, key, fn)
	return c.val, c.err
}

// [rtfsc]
// 执行一个任务，同 Do
// 不同之处在于，会返回的结果是由本次触发得到的，还是由其他触发得到的 (fresh：新鲜的结果)
// [end]
func (g *flightGroup) DoEx(key string, fn func() (any, error)) (val any, fresh bool, err error) {
	c, done := g.createCall(key)
	if done {
		return c.val, false, c.err
	}

	g.makeCall(c, key, fn)
	return c.val, true, c.err
}

// [rtfsc]
// 创建或者获取一个已经存在的 call，返回其引用
// c    call 的引用
// done 本次 call 是否完成了任务。完成了，则可以直接取 c.val
//
// 过程：
// 1. 操作 map 时加锁
// 2. 看当前的 key 是否有 call 在执行，有则直接取出，并等待这个 call 执行结束后返回。这样，同一个 key 不管调用多少次，都会等待同一个 call 的返回结果
// 3. 没有，则创建一个 call，并加入 map
// [end]
func (g *flightGroup) createCall(key string) (c *call, done bool) {
	g.lock.Lock()
	if c, ok := g.calls[key]; ok {
		g.lock.Unlock()
		c.wg.Wait()
		return c, true
	}

	c = new(call)
	c.wg.Add(1)
	g.calls[key] = c
	g.lock.Unlock()

	return c, false
}

// [rtfsc]
// 实际执行 call, 执行完成后，删除 map 中的 call
// [end]
func (g *flightGroup) makeCall(c *call, key string, fn func() (any, error)) {
	defer func() {
		g.lock.Lock()
		delete(g.calls, key)
		g.lock.Unlock()
		c.wg.Done()
	}()

	c.val, c.err = fn()
}
