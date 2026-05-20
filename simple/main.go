package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Server struct {
	funcs map[string]reflect.Value
}

func NewServer() *Server {
	return &Server{
		funcs: make(map[string]reflect.Value),
	}
}

func (s *Server) Register(name string, f interface{}) {
	if _, ok := s.funcs[name]; ok {
		panic(fmt.Sprintf("函数 %s 已经注册过了", name))
	}
	s.funcs[name] = reflect.ValueOf(f)
}

func (s *Server) Call(name string, args ...interface{}) ([]reflect.Value, error) {
	f, ok := s.funcs[name]
	if !ok {
		return nil, errors.New("找不到对应的函数")
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}

	out := f.Call(in)
	return out, nil
}

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}

func main() {
	server := NewServer()
	server.Register("Add", Add)
	server.Register("Multiply", Multiply)
	fmt.Println("=== 模拟客户端发起调用 ===")
	results, err := server.Call("Add", 10, 20)
	if err != nil {
		fmt.Println("调用失败:", err)
	} else {
		fmt.Printf("Add(10, 20) 结果是: %d\n", results[0].Int())
	}

	results2, err := server.Call("Multiply", 5, 6)
	if err != nil {
		fmt.Println("调用失败:", err)
	} else {
		fmt.Printf("Multiply(5, 6) 结果是: %d\n", results2[0].Int())
	}
	//没有client端，直接模拟调用，加client-》test
}
