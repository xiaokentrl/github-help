// Package ipc 是两个程序之间的那根线：一行一条 JSON，请求进来、回执出去，事件由服务往界面向推。
// 通道本身不认方法，方法名全部来自 contract 包，这样加一个方法只改 contract 一处。
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// maxLineBytes 挡住一条畸形消息把内存吃光。
const maxLineBytes = 1 << 20

// decoder 读一行解一条。
type decoder struct {
	sc *bufio.Scanner
}

func newDecoder(c net.Conn) *decoder {
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	return &decoder{sc: sc}
}

func (d *decoder) next() (*contract.Request, error) {
	if !d.sc.Scan() {
		if err := d.sc.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	var req contract.Request
	if err := json.Unmarshal(d.sc.Bytes(), &req); err != nil {
		return nil, fmt.Errorf("%s: 收到的请求不是合法 JSON: %w", errs.CodeIPCFrame, err)
	}
	if req.Method == "" {
		return nil, fmt.Errorf("%s: 收到的请求没有方法名", errs.CodeIPCNoMethod)
	}
	return &req, nil
}

// encoder 写一条并立刻冲出去，界面等的是这一条不是这一批。
type encoder struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func newEncoder(w io.Writer) *encoder {
	return &encoder{w: bufio.NewWriter(w)}
}

func (e *encoder) write(v any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("%s: 回不出去: %w", errs.CodeIPCWrite, err)
	}
	if _, err := e.w.Write(append(data, '\n')); err != nil {
		return err
	}
	return e.w.Flush()
}
