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

// 两个协程拿了同一个id
// 计算错误
// 客户端协程 157: 拿到结果 239
// 服务端: 收到请求 ID=166, 调用 add
// 服务端: 收到请求 ID=172, 调用 add
// 服务端: 收到请求 ID=174, 调用 add
// 服务端: 收到请求 ID=173, 调用 add
// 客户端协程 159: 拿到结果 170
// 客户端协程 229: 拿到结果 168
// 客户端协程 160: 拿到结果 240
// 客户端协程 158: 拿到结果 169
// 客户端协程 230: 拿到结果 241
// 客户端协程 231: 拿到结果 242
// 客户端协程 232: 拿到结果 243
// 客户端协程 234: 拿到结果 245
// 客户端协程 233: 拿到结果 244
// 客户端协程 235: 拿到结果 171
type Client struct {
	conn net.Conn
	dec  *gob.Decoder
	enc  *gob.Encoder
	mu   sync.Mutex
	seq  int
}

func NewClient(addr string) *Client {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	return &Client{
		conn: conn,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(conn),
		seq:  0,
	}
}

func (c *Client) Call(funcName string, args ...interface{}) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seq++
	reqID := c.seq

	req := Request{
		ID:       reqID,
		FuncName: funcName,
		Args:     args,
	}
	c.enc.Encode(&req)

	var resp Response
	c.dec.Decode(&resp)

	return resp.Result
}

func add(a, b int) int      { return a + b }
func multiply(a, b int) int { return a * b }

func main() {

	registerFunc("add", add)
	registerFunc("multiply", multiply)

	go serverListen()
	time.Sleep(1 * time.Second)
	client := NewClient("127.0.0.1:8080")

	var wg sync.WaitGroup
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			// 每次调用都在复用同一个 client 里的 conn
			res := client.Call("add", num, 10)
			fmt.Printf("客户端协程 %d: 拿到结果 %d\n", num, res)
		}(i)
	}

	wg.Wait()
	//一个等待另一个？？刚开始没加锁-》数据混乱-》加锁-》一个等另一个-》异步是什么意思？
}
