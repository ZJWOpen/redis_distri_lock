package schedule

import (
	"fmt"
	"runtime"
	"time"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"just.for.test/redistest/cache"
)

type DisLocker interface {
	Lock() error
	UnLock() bool
}

// Scheduler 定时任务器
type Scheduler struct {
	name            string //     主要用于dislock, redislogger namespace 区分
	cron            *cron.Cron
	locker          DisLocker
	serviceName     string
	redisLoggerSlot int32
}

// NewInShanghai 以上海时间为准
func NewInShanghai(name string, redisLoggerSlot int32) *Scheduler {
	var ShangHai, _ = time.LoadLocation("Asia/Shanghai")
	return &Scheduler{
		name:            name,
		cron:            cron.NewWithLocation(ShangHai),
		redisLoggerSlot: redisLoggerSlot,
	}
}

// Task 新建定时任务
func (sche *Scheduler) Task(name string, conditions ...Condition) *Task {
	return NewTask(Context{
		Name:            sche.name + ":" + name,
		DisLocker:       nil,
		Cron:            sche.cron,
		LockNamePrefix:  fmt.Sprintf("lock:%s", sche.serviceName),
		RedisLoggerSlot: sche.redisLoggerSlot,
	}, conditions...)
}

// Start 启动定时器
func (sche *Scheduler) Start() {
	sche.cron.Start()
}

// Stop 关闭定时器
func (sche *Scheduler) Stop() {
	sche.cron.Stop()
}

// Task 定时任务
type Task struct {
	ctx        Context
	spec       string // cron 语法
	f          func()
	conditions []Condition
}

// NewTask 新建任务
func NewTask(ctx Context, conditions ...Condition) *Task {
	task := &Task{ctx: ctx}
	task.conditions = append(task.conditions, conditions...)
	return task
}
func (task *Task) DisLock(timeout time.Duration) *Task {
	task.ctx.LockName = fmt.Sprintf("dislock:%s", task.ctx.Name)
	task.ctx.LockTimeout = timeout
	task.ctx.DisLocker = cache.NewRedisDisLock(task.ctx.LockName, timeout, 1)
	task.conditions = append(task.conditions, DisLockCondition)
	return task
}

// Context 定时任务执行的上下文
type Context struct {
	Name            string
	DisLocker       *cache.RedisDisLock
	Cron            *cron.Cron
	LockNamePrefix  string
	LockName        string
	LockTimeout     time.Duration
	RedisLoggerSlot int32
	Retries         int // 失败重试次数，默认为0
}

// ConditionFunc 任务执行控制函数
type ConditionFunc func(Context) error

// Condition 任务执行控制器
type Condition func(ConditionFunc) ConditionFunc

// Async 异步的方式立即执行该任务函数
func (task *Task) Async() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				log.Printf("Task async: panic running job: %v\n%s", r, buf)
			}
		}()
		task.f()
	}()
}

// AddCondition 添加条件控制器
func (task *Task) AddCondition(condi Condition) *Task {
	task.conditions = append(task.conditions, condi)
	return task
}
func (task *Task) Retry(n int) *Task {
	if n >= 0 && n <= 10 {
		task.ctx.Retries = n
	}
	return task
}

// spec 语法同contab
func (task *Task) DoCron(spec string) *Task {
	task.spec = spec
	if err := task.ctx.Cron.AddFunc(task.spec, task.f); err != nil {
		panic(err)
	}
	return task
}

// AddFunc 添加需要运行的函数
func (task *Task) AddFunc(funcs ...func() error) *Task {
	var taskFunc = func(_ Context) error {
		for _, f := range funcs {
			retries := task.ctx.Retries
			for {
				if err := f(); err != nil {
					if retries > 0 {
						retries--
						continue
					} else {
						break
					}
				} else {
					break
				}
			}
		}
		return nil
	}

	for i := len(task.conditions) - 1; i >= 0; i-- {
		taskFunc = task.conditions[i](taskFunc)
	}

	task.f = func() {
		taskFunc(task.ctx)
	}

	return task
}

var DisLockCondition = Condition(func(next ConditionFunc) ConditionFunc {

	return func(ctx Context) error {
		if ctx.DisLocker == nil {
			log.Println("warning! dislock unset!")
			return next(ctx)
		}

		err := ctx.DisLocker.Lock()
		if err != nil {
			log.Println("task was locked by", ctx.LockName)
			return nil
		}
		defer ctx.DisLocker.Unlock()

		return next(ctx)
	}
})
