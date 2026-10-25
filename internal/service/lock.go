package service

import (
	"github.com/hlep/github-accel/internal/errs"
)

// lock.go 管一件事：同一时刻只许有一个写入碰到 hosts 文件。
//
// 干什么用：界面上「应用这一条」「恢复这一条」「恢复默认」三颗按钮能在同一瞬间被点下去，
// 队列之外的任何一道关都挡不住"两条写入各写各的"。这把锁在服务侧，界面上的置灰只挡手点〔已定：技术方案第 4.2 节末〕。
//
// 什么情况下会错、出错时人看得见什么：锁没拿住就写 → 后写的把前写的盖掉，
//
//	用户看到的是"我明明应用了两条，文件里只剩一条"；
//
// 拿住了却忘了还 → 整条队列停摆，界面从此不动。所以还锁一定在 defer 里，出错也还。
//
// 怎么回来：队列本身是一条串行通道（L3 的永久解），正常情况下这把锁一次也不会撞；
// 真撞上了说明有人绕开队列直接调了写入——那一条用例（TestWriteLockRefusesSecondWriter）当场红。

// withWriteLock 在写锁里跑一段。跑不动就立刻回 GAC-HOSTS-006，不排队、不等待——
// 排队会让界面上那一条任务看起来"卡住了"，而这件事的实话是"有别的写入在跑，请等它结束"。
func (s *Service) withWriteLock(fn func() error) error {
	s.mu.Lock()
	if s.writeHeld {
		s.mu.Unlock()
		return newWriteBusy()
	}
	s.writeHeld = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.writeHeld = false
		s.mu.Unlock()
	}()
	return fn()
}

// writeLockHeld 给 GetStatus 的 writeLockHeld 那个真值。
func (s *Service) writeLockHeld() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeHeld
}

// newWriteBusy 就是"已经有写入在跑"那一句。码与参数在这里造，整句人话在前端语言文件那一个家〔已定：D6 乙〕。
func newWriteBusy() error {
	return errs.New(errs.CodeHostsBusy, nil, "已经有一个写入在跑，请等它结束")
}
