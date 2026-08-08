package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":9400", "监听地址")
	stock := flag.Int64("stock", 100, "秒杀商品库存")
	queue := flag.Int("queue", 1000, "异步队列长度")
	flag.Parse()

	e := newEngine(*stock, *queue)
	mux := http.NewServeMux()
	e.Register(mux)

	log.Printf("seckill 启动 %s，库存 %d", *addr, *stock)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
