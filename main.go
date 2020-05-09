package main

import (
	"time"

	"github.com/jinzhu/configor"
	"github.com/sirupsen/logrus"
	"just.for.test/redistest/cache"
	"just.for.test/redistest/schedule"
)

var testCron int32 = int32(2)
var (
	dailySched = schedule.NewInShanghai("Test Cron Job", testCron)
)

func Println() error {
	logrus.Println("Test Func....")
	return nil
}

func main() {
	var Config cache.Config
	configor.Load(&Config, "config.yml")

	cache.SyncRedis = cache.NewClient(Config)
	cache.InitRedSync(cache.SyncRedis)
	dailySched.Task("Test Job").DisLock(5 * time.Minute).AddFunc(Println).DoCron("*/1 * * * * *")

	dailySched.Start()
	//http.ListenAndServe(":8080", nil)
	select {}
}
