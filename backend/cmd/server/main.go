package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"tiantianxiangshang/internal/server"
	"tiantianxiangshang/internal/settings"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18081", "本机监听地址")
	data := flag.String("data", "data", "配置目录")
	web := flag.String("web", "dist/web", "前端目录")
	flag.Parse()
	host, _, e := net.SplitHostPort(*addr)
	if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("服务仅允许监听本机回环地址")
	}
	store, e := settings.Open(*data)
	if e != nil {
		log.Fatal(e)
	}
	s := &http.Server{Addr: *addr, Handler: server.New(store).Handler(*web), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("天天向上 http://%s", *addr)
	log.Fatal(s.ListenAndServe())
}
