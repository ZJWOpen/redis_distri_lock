package cache

import (
	"time"

	"gopkg.in/redsync.v1"
)

var (
	RedSync *redsync.Redsync
)

type RedisDisLock struct {
	*redsync.Mutex
}

func NewRedisDisLock(key string, timeout time.Duration, retries int) *RedisDisLock {
	return &RedisDisLock{
		Mutex: RedSync.NewMutex("dislock:"+key,
			redsync.SetExpiry(timeout),
			redsync.SetRetryDelay(50*time.Millisecond),
			redsync.SetTries(retries)),
	}
}

func (l *RedisDisLock) Lock() error {
	return l.Mutex.Lock()
}

func (l *RedisDisLock) Unlock() bool {
	return l.Mutex.Unlock()
}

// initialize redis connection pool for redis distributed lock to use
func InitRedSync(c *PoolClient) {
	RedSync = redsync.New([]redsync.Pool{
		redsync.Pool(c.Pool),
	})
}
