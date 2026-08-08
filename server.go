package main

import (
	"net/http"
	"sync"
)

// Engine 管理一次秒杀活动的对外接口。
type Engine struct {
	sk *Seckill
	// queue 是异步削峰用的请求队列，worker 从里面取请求慢慢处理，
	// 前端先拿到"已受理"，最终成绩以 /result 查为准。
	queue chan string
	mu    sync.Mutex
	done  map[string]bool
}

func newEngine(stock int64, queueSize int) *Engine {
	if queueSize <= 0 {
		queueSize = 1000
	}
	e := &Engine{
		sk:    newSeckill(stock),
		queue: make(chan string, queueSize),
		done:  make(map[string]bool),
	}
	// 启一个 worker 串行处理队列，模拟后台落单。
	go e.worker()
	return e
}

func (e *Engine) worker() {
	for user := range e.queue {
		ok, _ := e.sk.Try(user)
		e.mu.Lock()
		e.done[user] = ok
		e.mu.Unlock()
	}
}

// Register 挂上秒杀的 HTTP 接口。
func (e *Engine) Register(mux *http.ServeMux) {
	// 同步秒杀：直接返回成不成
	mux.HandleFunc("/seckill", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user == "" {
			writeErr(w, "需要 user 参数", http.StatusBadRequest)
			return
		}
		ok, err := e.sk.Try(user)
		if err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": ok})
	})

	// 异步秒杀：先入队，返回受理中
	mux.HandleFunc("/seckill/async", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		if user == "" {
			writeErr(w, "需要 user 参数", http.StatusBadRequest)
			return
		}
		select {
		case e.queue <- user:
			writeJSON(w, map[string]string{"status": "queued"})
		default:
			writeJSON(w, map[string]string{"status": "queue_full"})
		}
	})

	// 查异步结果
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		e.mu.Lock()
		ok, exists := e.done[user]
		e.mu.Unlock()
		if !exists {
			writeJSON(w, map[string]string{"status": "pending"})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": ok})
	})

	// 查库存和活动统计
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, e.sk.Stats())
	})
}
