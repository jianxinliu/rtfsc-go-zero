package syncx

import (
	"io"
	"sync"

	"github.com/zeromicro/go-zero/core/errorx"
)

/**
 * [rtfsc]
 * 主题: resourcemanager.go
 * 摘要: 资源管理器。资源：可关闭的，珍贵的，无需也不必要多次创建的
 * 功能: 管理的资源只创建一次。相当于【单例 + 池化】
 *
 *
 * 应用:
 *	redis 客户端。每个 redis 命令，都需要通过 redis 客户端发送到服务端执行。但是客户端连接并不需要每次都创建
 *               ref: redisclientmanager.go/getClient()
 *
 * [end]
 */

// [rtfsc]
// 资源管理器结构：
// *resources    存储资源的 map。资源用 io.Closer 接口表示，是一个可关闭的实例 【池化的体现】
// *singleFlight 用于控制相同的资源的创建操作只有一次 【单例的体现】
// *lock         因为 map 不是并发安全的，用于操作 map 时加锁。此处用于直接注入一个现有的资源实例
// [end]
// A ResourceManager is a manager that used to manage resources.
type ResourceManager struct {
	resources    map[string]io.Closer
	singleFlight SingleFlight
	lock         sync.RWMutex
}

// NewResourceManager returns a ResourceManager.
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		resources:    make(map[string]io.Closer),
		singleFlight: NewSingleFlight(),
	}
}

// [rtfsc]
// 关闭资源管理器，并将所持有的资源一一关闭（通过调用 io.Closer.Close 方法）并返回过程中出现的错误
// 关闭所有资源之后，将资源 map 设置为 nil, 防止在调用 Close 后继续使用资源管理器
// [end]
// Close closes the manager.
// Don't use the ResourceManager after Close() called.
func (manager *ResourceManager) Close() error {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	var be errorx.BatchError
	for _, resource := range manager.resources {
		if err := resource.Close(); err != nil {
			be.Add(err)
		}
	}

	// release resources to avoid using it later
	manager.resources = nil

	return be.Err()
}

// [rtfsc]
// 获取一个资源。存在则返回，不存在则调用传入的 create 函数进行创建
// 利用 singleFlight 的能力，保证相同的资源只创建一次
// 主要逻辑：
// 1. 从 resource map 获取，存在则返回
// 2. 不存在则调用 create 方法进行创建
// 3. 将创建得到的资源放入 resource map
// [end]
// GetResource returns the resource associated with given key.
func (manager *ResourceManager) GetResource(key string, create func() (io.Closer, error)) (
	io.Closer, error) {
	val, err := manager.singleFlight.Do(key, func() (any, error) {
		manager.lock.RLock()
		resource, ok := manager.resources[key]
		manager.lock.RUnlock()
		if ok {
			return resource, nil
		}

		resource, err := create()
		if err != nil {
			return nil, err
		}

		manager.lock.Lock()
		defer manager.lock.Unlock()
		manager.resources[key] = resource

		return resource, nil
	})
	if err != nil {
		return nil, err
	}

	return val.(io.Closer), nil
}

// Inject injects the resource associated with given key.
func (manager *ResourceManager) Inject(key string, resource io.Closer) {
	manager.lock.Lock()
	manager.resources[key] = resource
	manager.lock.Unlock()
}
