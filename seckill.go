package main

import (
	"errors"
	"sync"
	"sync/atomic"
)

// 秒杀里最怕的就是超卖：100 件商品，1 万个请求同时来，不能卖出 101 单。
// 这里用两层保护：
//   1) 原子计数器做"快速拒绝"——超过库存的请求直接拦在门外，连锁都不用进
//   2) 真正扣减时再加锁，保证库存不会算错

// ErrSoldOut 没货了。
// ErrDup 同一个用户重复秒杀。
var (
	ErrSoldOut = errors.New("已售罄")
	ErrDup     = errors.New("你已经秒杀过了")
)

// Seckill 是一个商品的秒杀活动。
type Seckill struct {
	mu    sync.Mutex
	stock int64 // 剩余库存
	users map[string]bool
	total int64 // 初始库存（用于统计）

	// claimed 是已经"占位"的人数（含成功和重复），用原子计数做快速拦截。
	claimed int64

	success int64 // 成功下单数（原子）
	failed  int64 // 失败数（原子）
}

func newSeckill(stock int64) *Seckill {
	return &Seckill{
		stock: stock,
		total: stock,
		users: make(map[string]bool),
	}
}

// Try 尝试秒杀一个用户。返回是否成功。
// 设计要点：先用原子计数拦掉超额请求，再进锁做精确扣减和去重，
// 这样绝大多数超量请求在原子层就被挡掉，不会堆积在锁上。
func (s *Seckill) Try(user string) (bool, error) {
	// 快速拦截：占位数已经 >= 库存，直接售罄。
	// 注意这里是 >= 而不是 >，因为 claimed 可能略超（并发下原子+1 后才知道超没超），
	// 所以锁里还要再精确校验一次。
	if atomic.AddInt64(&s.claimed, 1) > s.total {
		atomic.AddInt64(&s.claimed, -1) // 没真买到，把占位还回去
		atomic.AddInt64(&s.failed, 1)
		return false, ErrSoldOut
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 去重：同一用户只能买一次。
	if s.users[user] {
		atomic.AddInt64(&s.failed, 1)
		return false, ErrDup
	}
	// 精确校验库存（原子层可能多放了几个进来）。
	if s.stock <= 0 {
		atomic.AddInt64(&s.failed, 1)
		return false, ErrSoldOut
	}

	s.stock--
	s.users[user] = true
	atomic.AddInt64(&s.success, 1)
	return true, nil
}

func (s *Seckill) Stock() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stock
}

// Stats 返回活动统计。
func (s *Seckill) Stats() map[string]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]int64{
		"total":   s.total,
		"stock":   s.stock,
		"success": atomic.LoadInt64(&s.success),
		"failed":  atomic.LoadInt64(&s.failed),
		"claimed": atomic.LoadInt64(&s.claimed),
	}
}
