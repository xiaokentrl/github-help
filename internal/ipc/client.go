package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hlep/github-accel/internal/contract"
	"github.com/hlep/github-accel/internal/errs"
)

// Client 是界面侧的连接。它只认两件事：能不能连上、连上之后能不能问出话、话问完还剩的事件往哪儿交。
// 界面不许自己判断"服务大概还在跑"，一切状态以服务给的那份为准。
//
// 这一份文件为什么在 2026-10-23 联调轮整个改掉：
// 老写法是"每次调用现起一个读的人"，读到事件就 continue 丢掉〔实测：老 client.go:105〕。
// 症状是服务那 5 条事件全推给了空气——进度条永远不动、队列跑完了界面还停在原地，而且一个字也不报。
// 现在改成**一条线只有一个读的人**：回执按 id 交回等它的那次调用，事件交给 Events() 那一个出口。
type Client struct {
	mu   sync.Mutex
	conn net.Conn
	dec  *decoder
	enc  *encoder
	next int64

	// pending 是"这次调用还在等哪一条 id 的回执"。读的人只管照 id 投递，谁等谁自己收。
	pending map[int64]chan *contract.Response
	// events 是事件的唯一出口（容量见 eventQueue）。
	events chan contract.Event
	// dropped 数的是"读的人手里有事件但没人来得及接"的次数，如实给上层看，不吞。
	dropped int64
	// gen 每建一条连接换一个号，用来认出"这条读循环属于已被换掉的那条旧连接"。
	gen int64
}

// eventQueue 给 64：一次批量检测最多连着推几十条进度，队列够深就一条也不会因为画不过来而丢。
// 为什么允许丢而不是卡住读循环：每条事件都自带**完整快照**，丢一条＝少画一帧，
// 下一条来的时候界面看到的还是此刻的真实全状态；而卡住读循环会连带把回执也堵住，那是界面整个冻住。
const eventQueue = 64

// ErrNoConnection 是一个字面意思的错：此刻连不上。调用方拿它决定要不要重连。
var ErrNoConnection = errors.New(errs.CodeIPCDisconnect + ": 连不上后台服务")

// ErrVersionMismatch 是握手比出来的那一句：两侧不是同一份契约。
// 干什么用：拦住"旧服务配新界面"——那种组合不会报错，只会让界面每一行的新字段永远空着。
// 出错时人看得见什么：这一条带两个版本号，界面那句人话直接把"界面要 v几、服务给的是 v几"说出来。
var ErrVersionMismatch = errors.New(errs.CodeIPCShape + ": 界面与后台服务的契约版本对不上")

// NewClient 建一个还没连上的客户端；连哪儿由调用方给（界面侧只管试 Endpoint 给的那个地址）。
func NewClient() *Client {
	return &Client{pending: map[int64]chan *contract.Response{}, events: make(chan contract.Event, eventQueue)}
}

func (c *Client) Connect(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return nil
	}
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNoConnection, err)
	}
	c.conn = conn
	c.dec = newDecoder(conn)
	c.enc = newEncoder(conn)
	c.gen++
	go c.readLoop(conn, c.dec, c.gen)
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

func (c *Client) closeLocked() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.dec = nil
	c.enc = nil
	return err
}

// IsConnected 只回答一件事：现在这条线在不在。
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// Events 交出事件那一条出口。读的人只往这一条 channel 放，界面（或壳）从这一头取。
// 只许有一个消费者：两个消费者会一条被我拿走一条被你拿走，谁看到的都不是完整那段过程。
func (c *Client) Events() <-chan contract.Event { return c.events }

// Dropped 是"这条线上丢过几帧事件"。它给的是数，不是错——丢了不影响正确性（下一条自带完整快照），
// 但如果它一直涨，说明消费那头画不过来，界面上的进度就会看着像跳格：这一句要如实报出来，不许当没事。
func (c *Client) Dropped() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropped
}

// readLoop 是这条线上唯一的读的人。它一结束就把这条连接判废：
// 读不动意味着对端已经没了，留着半条连接只会让下一次调用等到超时。
func (c *Client) readLoop(conn net.Conn, dec *decoder, gen int64) {
	for {
		line, err := readOne(dec)
		if err != nil {
			c.fail(conn, gen)
			return
		}
		if line.Event != nil {
			c.publish(*line.Event)
			continue
		}
		c.deliver(line.Response)
	}
}

// publish 把一条事件交出去；那头来不及接就丢掉这一帧并把 dropped 加一（理由见 eventQueue 那段）。
func (c *Client) publish(ev contract.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case c.events <- ev:
	default:
		c.dropped++
	}
}

// deliver 把回执交给等它的那次调用。认不出的 id 直接扔——那是调用已经超时并判废了这条线。
func (c *Client) deliver(resp *contract.Response) {
	c.mu.Lock()
	ch, ok := c.pending[resp.ID]
	c.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- resp:
	default:
	}
}

// fail 只在"这条连接还是当前这一条"时把它判废。
// 为什么要比 gen：调用方超时后已经换了一条新连接，旧读循环这时才醒过来报错，
// 不该把新连接当成坏的关掉——那会表现为"用着用着自己断了"，最难查。
func (c *Client) fail(conn net.Conn, gen int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == conn {
		_ = c.closeLocked()
		c.abandonAllLocked()
	}
}

// abandonAllLocked 叫醒所有还在等回执的调用：连接没了，再等也是等到超时，不如当场拿到一句实话。
func (c *Client) abandonAllLocked() {
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

// Call 发一个方法、等一条回执。
// 超时不等于失败：服务可能正在写 hosts，所以超时只把连接判废，让上层重连后重新取状态。
func (c *Client) Call(method string, params map[string]any, timeout time.Duration) (any, error) {
	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return nil, ErrNoConnection
	}
	c.next++
	id := c.next
	ch := make(chan *contract.Response, 1)
	c.pending[id] = ch
	enc := c.enc
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := contract.Request{ID: id, Method: method, Params: params}
	if err := enc.write(req); err != nil {
		c.discard()
		return nil, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp, open := <-ch:
		if !open {
			return nil, ErrNoConnection
		}
		if resp.Protocol != contract.ProtocolVersion {
			// 版本对不上先于 OK 判：旧服务回得再"成功"，缺的字段也是空的，界面拿它会摆出一副没事的样子。
			return nil, fmt.Errorf("%w: 界面要 v%d，服务给的是 v%d", ErrVersionMismatch, contract.ProtocolVersion, resp.Protocol)
		}
		if !resp.OK {
			return nil, fmt.Errorf("%s: %s", resp.Code, resp.Err)
		}
		return resp.Result, nil
	case <-timer.C:
		c.discard()
		return nil, fmt.Errorf("%s: 等 %s 的回执超时", errs.CodeIPCTimeout, method)
	}
}

// 一条线上可能先来事件后来回执，所以这里要能认出"这行是事件还是回执"。
type inbound struct {
	Response *contract.Response
	Event    *contract.Event
}

func readOne(d *decoder) (*inbound, error) {
	for {
		if !d.sc.Scan() {
			if err := d.sc.Err(); err != nil {
				return nil, err
			}
			return nil, ErrNoConnection
		}
		line := d.sc.Bytes()
		// 回执带 id、事件带 name：先看有没有 id，认不出就当场报错，不猜。
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(line, &probe); err != nil {
			return nil, fmt.Errorf("%s: 收到的行不是合法 JSON: %w", errs.CodeIPCReply, err)
		}
		if _, hasID := probe["id"]; !hasID {
			var ev contract.Event
			if err := json.Unmarshal(line, &ev); err != nil {
				return nil, err
			}
			return &inbound{Event: &ev}, nil
		}
		var resp contract.Response
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, err
		}
		return &inbound{Response: &resp}, nil
	}
}

func (c *Client) discard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.closeLocked()
		c.abandonAllLocked()
	}
}

// Status 问一次"服务在跑吗、跑到哪一步了"。连不上就返回 contract.ServiceUnavailable。
func (c *Client) Status() (contract.StatusReply, error) {
	var out contract.StatusReply
	res, err := c.Call(contract.MethodGetStatus, nil, 2*time.Second)
	if err != nil {
		out.Service.Phase = contract.ServiceUnavailable
		return out, err
	}
	if err := reMarshal(res, &out); err != nil {
		out.Service.Phase = contract.ServiceUnavailable
		return out, err
	}
	return out, nil
}

// Snapshot 主动取一次完整状态：54 条各是什么、队列里有哪几行、此刻暂停没有。
//
// 干什么用：开机那一下和断线重连之后各问一次。此前这个口根本不存在——快照只随事件推，
// 界面重连后只能干等下一次有人点按钮，树会一直空着。
// 什么情况下会错：把"连不上"当成"一条都没有" → 界面把用户自己写的那几行说成没生效。
// 所以出错时返回的是错，不是一个空快照；调用方拿 ErrNoConnection 决定要不要重连。
func (c *Client) Snapshot() (contract.Snapshot, error) {
	var out contract.Snapshot
	res, err := c.Call(contract.MethodGetSnapshot, nil, 5*time.Second)
	if err != nil {
		return out, err
	}
	if err := reMarshal(res, &out); err != nil {
		return out, err
	}
	return out, nil
}

// reMarshal 把通道上那份 any（解 JSON 出来的 map）转成契约结构体。
// 为什么绕这一道而不是让 Call 直接解到结构体：Call 得保持对任何方法通用，形状只由各调用方声明。
func reMarshal(from any, to any) error {
	b, err := json.Marshal(from)
	if err != nil {
		return fmt.Errorf("%s: 服务给的内容拼不回去: %w", errs.CodeIPCShape, err)
	}
	if err := json.Unmarshal(b, to); err != nil {
		return fmt.Errorf("%s: 服务给的形状看不懂: %w", errs.CodeIPCShape, err)
	}
	return nil
}
