package main

import (
	"fmt"
	"sync"
	"testing"
)

func TestNoOversell(t *testing.T) {
	const stock = 100
	const users = 1000
	sk := newSeckill(stock)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var okCount, soldOut, dup int
	for i := 0; i < users; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := sk.Try(fmt.Sprintf("u%d", i))
			mu.Lock()
			defer mu.Unlock()
			if ok {
				okCount++
			} else if err == ErrSoldOut {
				soldOut++
			} else if err == ErrDup {
				dup++
			}
		}(i)
	}
	wg.Wait()

	if okCount != stock {
		t.Fatalf("成功数应为库存 %d, 实际 %d", stock, okCount)
	}
	if sk.Stock() != 0 {
		t.Fatalf("剩余库存应为 0, 实际 %d", sk.Stock())
	}
	// 每个用户只出现一次，不可能有 dup
	if dup != 0 {
		t.Fatalf("不应有重复秒杀, 实际 %d", dup)
	}
	// 成功 + 售罄 应等于总请求数（去重为 0 时）
	if okCount+soldOut != users {
		t.Fatalf("成功+售罄应等于请求数 %d, 实际 %d", users, okCount+soldOut)
	}
	st := sk.Stats()
	if st["success"] != int64(stock) {
		t.Fatalf("统计 success 应为 %d, 实际 %d", stock, st["success"])
	}
}

func TestDupUserRejected(t *testing.T) {
	sk := newSeckill(10)
	ok, err := sk.Try("alice")
	if !ok || err != nil {
		t.Fatal("第一次应成功")
	}
	ok, err = sk.Try("alice")
	if ok {
		t.Fatal("重复用户不应再成功")
	}
	if err != ErrDup {
		t.Fatalf("应返回 ErrDup, 得到 %v", err)
	}
}

func TestEngineConcurrent(t *testing.T) {
	const stock = 50
	e := newEngine(stock, 200)

	// 并发调秒杀，验证总成功数不超库存。
	var mu sync.Mutex
	okTotal := 0
	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, _ := e.sk.Try(fmt.Sprintf("u%d", i))
			if ok {
				mu.Lock()
				okTotal++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if okTotal != stock {
		t.Fatalf("并发成功数应等于库存 %d, 实际 %d", stock, okTotal)
	}
	if e.sk.Stock() != 0 {
		t.Fatalf("库存应为 0, 实际 %d", e.sk.Stock())
	}
}
