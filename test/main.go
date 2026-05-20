package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"reflect"
	"time"
)

type Data struct {
	FuncName string
	Args     []interface{}
}

var funcs = make(map[string]reflect.Value)

func registerFunc(name string, fn interface{}) {
	if _, ok := funcs[name]; ok {
		fmt.Println("Function already registered")
		return
	}
	funcs[name] = reflect.ValueOf(fn)
	fmt.Println("服务端: 成功注册函数", name)
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	var data Data
	err := gob.NewDecoder(conn).Decode(&data)
	if err != nil {
		fmt.Println("服务端解码错误:", err)
		return
	}

	fmt.Printf("服务端: 收到调用请求 - 函数名: %s, 参数: %v\n", data.FuncName, data.Args)

	if _, ok := funcs[data.FuncName]; !ok {
		fmt.Println("服务端: 找不到该函数")
		return
	}

	f := funcs[data.FuncName]
	args := make([]reflect.Value, len(data.Args))
	for i, arg := range data.Args {
		args[i] = reflect.ValueOf(arg)
	}
	results := f.Call(args)

	//gob: decoding into local type *int, received remote type interface
	out := results[0].Interface()
	//func (enc *Encoder) Encode(e interface{}) error
	gob.NewEncoder(conn).Encode(out)
	//gob.NewEncoder(conn).Encode(&out)
}

func serverListen() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	fmt.Println("服务端: 开始在 :8080 端口监听...")
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		handleConn(conn)
	}
}

func callRPC(addr string, funcName string, args ...interface{}) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	reqData := Data{
		FuncName: funcName,
		Args:     args,
	}
	err = gob.NewEncoder(conn).Encode(&reqData)
	if err != nil {
		fmt.Println("客户端发送失败:", err)
		return
	}

	var result int
	err = gob.NewDecoder(conn).Decode(&result)
	if err != nil {
		fmt.Println("客户端接收失败:", err)
		return
	}

	fmt.Printf("客户端: 调用 %s 成功，拿到最终结果: %d\n", funcName, result)
}

func add(a, b int) int {
	return a + b
}

func multiply(a, b int) int {
	return a * b
}

func main() {

	registerFunc("add", add)
	registerFunc("multiply", multiply)

	go serverListen()
	// 	panic: dial tcp 127.0.0.1:8080: connect: connection refused

	// goroutine 1 [running]:
	// main.callRPC({0x5592ed?, 0xc00011c020?}, {0x55787e, 0x3}, {0xc00012c100, 0x2, 0x2})
	//         /home/hahhhy/rpc/test/main.go:73 +0x5fe
	// main.main()
	//         /home/hahhhy/rpc/test/main.go:115 +0xf2
	// exit status 2
	//服务端的 net.Listen 还没来得及向操作系统成功绑定 8080 端口，客户端的 net.Dial就链接
	time.Sleep(1 * time.Second)

	fmt.Println("\n--- 开始模拟客户端发起 RPC 调用 ---")
	callRPC("127.0.0.1:8080", "add", 10, 20)
	callRPC("127.0.0.1:8080", "multiply", 5, 6)
	//每次调用一次，连接就挂了，加一个长连接-》bing
}
