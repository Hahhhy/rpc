package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"
)

type Request struct {
	ID       int
	FuncName string
	Args     []interface{}
}

type Response struct {
	ID     int
	Result interface{}
}

var funcs = make(map[string]reflect.Value)

func registerFunc(name string, fn interface{}) {
	funcs[name] = reflect.ValueOf(fn)
	fmt.Println("服务端: 成功注册函数", name)
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)

	for {
		var req Request
		err := dec.Decode(&req)
		if err != nil {
			return
		}
		fmt.Printf("服务端: 收到请求 ID=%d, 调用 %s\n", req.ID, req.FuncName)

		f := funcs[req.FuncName]
		args := make([]reflect.Value, len(req.Args))
		for i, arg := range req.Args {
			args[i] = reflect.ValueOf(arg)
		}

		results := f.Call(args)
		out := results[0].Interface()

		resp := Response{
			ID:     req.ID,
			Result: out,
		}
		enc.Encode(&resp)
	}
}

func serverListen() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn)
	}
}

type Client struct {
	conn    net.Conn
	dec     *gob.Decoder
	enc     *gob.Encoder
	seq     int
	pending map[int]chan interface{}
}

func NewClient(addr string) *Client {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	client := &Client{
		conn:    conn,
		dec:     gob.NewDecoder(conn),
		enc:     gob.NewEncoder(conn),
		seq:     0,
		pending: make(map[int]chan interface{}),
	}

	return client
}

func (c *Client) receive() {
	for {
		var resp Response
		err := c.dec.Decode(&resp)
		if err != nil {
			fmt.Println("读取后台退出:", err)
			return
		}
		ch, ok := c.pending[resp.ID]
		if ok {
			ch <- resp.Result
			delete(c.pending, resp.ID)
		}
	}
}

func (c *Client) Call(funcName string, args ...interface{}) interface{} {
	c.seq++
	reqID := c.seq

	ch := make(chan interface{})
	c.pending[reqID] = ch

	req := Request{
		ID:       reqID,
		FuncName: funcName,
		Args:     args,
	}
	c.enc.Encode(&req)

	result := <-ch
	return result
}
func add(a, b int) int      { return a + b }
func multiply(a, b int) int { return a * b }

func main() {
	gob.Register(int(0))

	registerFunc("add", add)
	registerFunc("multiply", multiply)

	go serverListen()
	time.Sleep(1 * time.Second)

	client := NewClient("127.0.0.1:8080")
	go client.receive()

	var wg sync.WaitGroup
	for i := 1; i <= 666; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			res := client.Call("add", num, 10)
			fmt.Printf("客户端协程 %d: 拿到结果 %d\n", num, res)
		}(i)
	}
	wg.Wait()
}

// goroutine 251 [runnable]:
// main.main.gowrap2()
//         /home/hahhhy/rpc/yibu/main.go:149
// runtime.goexit({})
//         /usr/local/go/src/runtime/asm_amd64.s:1700 +0x1
// created by main.main in goroutine 1
//         /home/hahhhy/rpc/yibu/main.go:149 +0xf7

// goroutine 252 [runnable]:
// main.main.gowrap2()
//         /home/hahhhy/rpc/yibu/main.go:149
// runtime.goexit({})
//         /usr/local/go/src/runtime/asm_amd64.s:1700 +0x1
// created by main.main in goroutine 1
//         /home/hahhhy/rpc/yibu/main.go:149 +0xf7
// exit status 2
//跑崩了，一多起来，map并发读？乱读啥的-》》》对map的操作加一把锁
