package syncx

import (
	"errors"

	"github.com/zeromicro/go-zero/core/lang"
)

/**
 * [rtfsc]
 * 主题: limit.go
 * 摘要: 类似信号量
 * 功能: 用于控制数量，如并发数
 * [end]
 */

// ErrLimitReturn indicates that the more than borrowed elements were returned.
var ErrLimitReturn = errors.New("discarding limited token, resource pool is full, someone returned multiple times")

// Limit controls the concurrent requests.
type Limit struct {
	pool chan lang.PlaceholderType
}

// [rtfsc]
// 创建一个限制器，并规定总的数量
// [end]
// NewLimit creates a Limit that can borrow n elements from it concurrently.
func NewLimit(n int) Limit {
	return Limit{
		pool: make(chan lang.PlaceholderType, n),
	}
}

// [rtfsc]
// 占用一个信号量，通过向 channel 写入实现。
// 利用 channel 的特性，channel 满了之后再写入，就得等待
// 以此来实现限制的作用
// [end]
// Borrow borrows an element from Limit in blocking mode.
func (l Limit) Borrow() {
	l.pool <- lang.Placeholder
}

// [rtfsc]
// 归还信号量
// [end]
// Return returns the borrowed resource, returns error only if returned more than borrowed.
func (l Limit) Return() error {
	select {
	case <-l.pool:
		return nil
	default:
		return ErrLimitReturn
	}
}

// [rtfsc]
// 使用非阻塞的方式尝试获取一个信号量，获取不到则返回 false
// [end]
// TryBorrow tries to borrow an element from Limit, in non-blocking mode.
// If success, true returned, false for otherwise.
func (l Limit) TryBorrow() bool {
	select {
	case l.pool <- lang.Placeholder:
		return true
	default:
		return false
	}
}
