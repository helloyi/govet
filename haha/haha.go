package haha

import (
	ctxb "context"
	. "go/ast"

	"fmt"

	ctxa "golang.org/x/net/context"
)

var (
	balala = fmt.Errorf("balala")
	pilili = fmt.Errorf("pilili")
	errFoo = fmt.Errorf("pilili")
	ErrBar = fmt.Errorf("pilili")
)

type contextAsField struct {
	c1 ctxa.Context
	c2 ctxb.Context

	n Node
}

// EmptySliceDecl ...
func EmptySliceDecl() {
	v1 := []int{}
	v2 := []contextAsField{}
	v3 := []map[int]int{}

	for k, _ := range v1 {
		fmt.Println(k)
	}
	fmt.Println(v1, v2, v3)
}

type recv struct {
	a, b int
}

func (r *recv) func1() int {
	r.a += 1
	r.b -= 1
	r.b += 22
	r.b -= 22
	return r.a
}

func (_ *recv) func2() int {
	return 1
}

func (this *recv) func3() int {
	return this.a
}

func (self *recv) func4() int {
	return self.b
}

func (r *recv) func5() (int, error) {
	return r.b, nil
}
func (r *recv) func6() (int, error, int) {
	return r.b, nil, r.a
}
