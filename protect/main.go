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
		go func(request Request) {
			f := funcs[request.FuncName]
			args := make([]reflect.Value, len(request.Args))
			for i, arg := range request.Args {
				args[i] = reflect.ValueOf(arg)
			}

			results := f.Call(args)
			out := results[0].Interface()

			resp := Response{
				ID:     request.ID,
				Result: out,
			}
			enc.Encode(&resp)
		}(req)
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

	mu     sync.Mutex
	sendMu sync.Mutex
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
	go client.receive()
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

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
		//考虑这个地方的顺序
		//往 Channel 里塞数据这一步，放在了 Unlock 之后
		// 为什么？因为塞数据可能会阻塞，千万不要拿着 Map 的锁去阻塞
		if ok {
			ch <- resp.Result
		}
	}
}

func (c *Client) Call(funcName string, args ...interface{}) interface{} {
	c.mu.Lock()
	c.seq++
	reqID := c.seq
	ch := make(chan interface{})
	c.pending[reqID] = ch
	c.mu.Unlock()

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

	registerFunc("add", add)
	registerFunc("multiply", multiply)

	go serverListen()
	time.Sleep(1 * time.Second)

	client := NewClient("127.0.0.1:8080")

	var wg sync.WaitGroup
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			res := client.Call("add", num, 100)
			fmt.Printf("客户端协程 %d: 拿到结果 %d\n", num, res)
		}(i)
	}
	wg.Wait()
}

//对资源的并发保护？？
