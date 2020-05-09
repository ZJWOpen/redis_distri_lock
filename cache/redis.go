package cache

import (
	"fmt"
	"time"

	redigo "github.com/gomodule/redigo/redis"
)

var SyncRedis *PoolClient

type PoolClient struct {
	Pool *redigo.Pool
}

type Config struct {
	Redis RedisConf
}

type RedisConf struct {
	Tag         string
	Host        string
	Port        string
	Password    string
	MaxIdle     int
	IdleTimeout int
}

func NewClient(conf Config) *PoolClient {
	url := fmt.Sprintf("redis://%s:%s", conf.Redis.Host, conf.Redis.Port)
	if conf.Redis.Password != "" {
		url = fmt.Sprintf("redis://:%s@%s:%s", conf.Redis.Password, conf.Redis.Host, conf.Redis.Port)
	}
	pool := newPool(url, conf.Redis.MaxIdle, conf.Redis.IdleTimeout)
	return &PoolClient{Pool: pool}
}

type Trans struct {
	conn redigo.Conn
}

func (t *Trans) Send(cmd string, args ...interface{}) {
	t.conn.Send(cmd, args...)
}

func (t *Trans) Exec() (reply interface{}, err error) {
	defer t.conn.Close()
	return t.conn.Do("EXEC")
}

func (self *PoolClient) Do(cmd string, args ...interface{}) (reply interface{}, err error) {
	conn := self.Pool.Get()
	defer conn.Close()
	return conn.Do(cmd, args...)
}

func (self *PoolClient) GetConn() redigo.Conn {
	return self.Pool.Get()
}

func (self *PoolClient) BeginTrans() *Trans {
	conn := self.Pool.Get()
	conn.Send("MULTI")
	return &Trans{conn}
}

func newPool(url string, maxIdle, idleTimeout int) *redigo.Pool {
	return &redigo.Pool{
		MaxIdle:     maxIdle,
		IdleTimeout: time.Duration(idleTimeout) * time.Second,
		Dial: func() (redigo.Conn, error) {
			c, err := redigo.DialURL(url)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
