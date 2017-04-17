package main

import (
	"pool"
	"fmt"
	"net"
	"net/rpc"
	"time"
)

type Args struct {
	A, B int
}

var PoolList pool.Pool

func newClient() (interface{}, error) {
	address, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	if err != nil {
		panic(err)
	}
	conn, _ := net.DialTCP("tcp", nil, address)
	client := rpc.NewClient(conn)
	return client, nil
}

func Init() {
	close := func(v interface{}) error { return v.(*rpc.Client).Close() }
	poolConfig := &pool.PoolConf{
		MinCap:      1000,
		MaxCap:      3000,
		New:         newClient,
		Close:       close,
		IdleTimeout: 15 * time.Second,
	}
	var err error
	PoolList, err = pool.NewPool(poolConfig)
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	fmt.Println("初始化成功...", PoolList.Len())
}

func main() {
	Init()
	for i := 0; i <= 200000; i++ {
		go send(i)
	}
	select {}
}

func send(i int) {
	v, err := PoolList.Get()
	if err != nil {
		fmt.Println("err:", err, i)
		return
	}
	defer PoolList.Put(v)
	client := v.(*rpc.Client)
	args := &Args{7, 8}
	reply := new(int)
	err = client.Call("Arith.Multiply", args, &reply)
	if err != nil {
		fmt.Println("arith error:", err, i)
	}
	fmt.Println(i,"--", *reply)
}
