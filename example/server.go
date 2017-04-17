package main

import (
	"net/rpc"
	"log"
	"net"
	"fmt"
)

type Args struct {
	A, B int
}
type Arith int

func (t *Arith) Multiply(args *Args, reply *(int)) error {
	*reply = args.A * args.B
	return nil
}
func main() {
	newServer := rpc.NewServer()
	newServer.Register(new(Arith))
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":8888")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	fmt.Println("启动服务")
	newServer.Accept(l)
}
