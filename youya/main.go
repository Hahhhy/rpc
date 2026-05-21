package main

import (
	"context"
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
	var sendMu sync.Mutex

	for {
		var req Request
		err := dec.Decode(&req)
		if err != nil {
			return
		}

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

			sendMu.Lock()
			enc.Encode(&resp)
			sendMu.Unlock()
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

func (c *Client) Close() {
	c.sendMu.Lock()
	c.conn.Close()
	c.sendMu.Unlock()
	fmt.Println("\n客户端: 主动关闭连接...")
}

func (c *Client) receive() {
	for {
		var resp Response
		err := c.dec.Decode(&resp)
		if err != nil {
			fmt.Println("后台通知: 检测到网络连接已断开，开始挂...")
			c.mu.Lock()
			for reqID, ch := range c.pending {
				close(ch)
				delete(c.pending, reqID)
			}
			c.mu.Unlock()

			fmt.Println("后台通知: 挂完，所有挂起协程唤醒。")
			return
		}

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()

		if ok {
			ch <- resp.Result
		}
	}
}

func (c *Client) Call(ctx context.Context, funcName string, args ...interface{}) interface{} {
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

	c.sendMu.Lock()
	err := c.enc.Encode(&req)
	c.sendMu.Unlock()
	if err != nil {
		return nil
	}
	//result := <-ch
	select {
	case result := <-ch:
		return result
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, reqID)
		c.mu.Unlock()
		fmt.Printf("超时请求")
		return nil
	}
}

func slowAdd(a, b int) int {
	time.Sleep(2 * time.Second)
	return a + b
}

func main() {
	gob.Register(int(0))
	registerFunc("slowAdd", slowAdd)

	go serverListen()
	time.Sleep(500 * time.Millisecond)

	client := NewClient("127.0.0.1:8080")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			fmt.Printf("客户端协程 %d: 发起 slowAdd 请求，预计需要等 2 秒...\n", num)
			res := client.Call(ctx, "slowAdd", num, 100)

			if res == nil {
				fmt.Printf("客户端协程 %d: 悲剧了，被唤醒了！\n", num)
			} else {
				fmt.Printf("客户端协程 %d: 成功拿到正常结果 %d\n", num, res)
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	//等0.5就给他挂
	client.Close()

	wg.Wait()
	fmt.Println("主线程:goobye")
}
