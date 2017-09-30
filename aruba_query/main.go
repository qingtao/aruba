package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	version = "v20170215"
)

var (
	//生成配置文件
	INITCONFIG = flag.Bool("gen", false, "生成配置文件模板")
	//版本信息
	VERSION = flag.Bool("version", false, "打印版本信息")
	TEST    = flag.Bool("test", false, "测试配置文件")
)

//配置JSON模板
var cfgT = &Config{
	Addr:     "127.0.0.1:50053",
	Duration: 10,
	Database: MysqlDB{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "root",
		Password: "123789",
		DB:       "aruba",
	},
}

func main() {
	flag.Parse()
	//程序目录路径
	baseDir := Basedir()
	confDir, tmpDir := filepath.Join(baseDir, "etc"), filepath.Join(baseDir, "tmp")

	Mkdir(confDir)
	Mkdir(tmpDir)

	if *VERSION {
		fmt.Printf("version: %s\n", version)
		return
	}

	if *INITCONFIG {
		exampleConfig := filepath.Join(confDir, "config.json.example")
		if err := CreateConfigFile(exampleConfig, cfgT); err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("create example for config:\n%s\n", exampleConfig)
		return
	}

	CONF := filepath.Clean(filepath.Join(confDir, "config.json"))

	cfg, err := ReadConfigFile(CONF)
	if err != nil {
		log.Fatalln("read config file error: ", err)
	}

	if *TEST {
		fmt.Printf("%s is ok\n", CONF)
		return
	}

	var logger = NewLogger(filepath.Join(tmpDir, "aruba.log"))
	logger.Println("aruba_query started")
	logger.Printf("version: %s\n", version)

	pid, pidFile := os.Getpid(), filepath.Join(tmpDir, "aruba.pid")
	logger.Printf("pid: %d, path: %s\n", pid, pidFile)
	if err := ioutil.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0666); err != nil {
		logger.Printf("write pid: %s\n", err)
		return
	}

	wf := filepath.Join(baseDir, "etc", "whitelist")
	srv, err := NewServer(wf, logger)
	if err != nil {
		log.Fatalln("read whitelist error: ", err)
	}

	//设置数据库连接
	cfg.OpenMysql()
	if err = cfg.db.Ping(); err != nil {
		logger.Printf("connect to mysql %s:%s failed: %s\n", cfg.Database.Host, cfg.Database.Port, err)
	}
	logger.Printf("connect to mysql %s:%s success\n", cfg.Database.Host, cfg.Database.Port)
	go srv.Listen(cfg.Addr, cfg, logger)

	time.Sleep(5 * time.Second)
	ech, done := make(chan error), make(chan bool)
	go func() {
		for {
			select {
			case err := <-ech:
				logger.Printf("%s\n", err)
			case <-done:
				logger.Println("update service provider success")
			}
		}
	}()

	d := time.Duration(cfg.Duration) * time.Minute
	logger.Printf("cron job for update service provider once every %s", d)

	for {
		//5分钟后更新没有完成, 取消任务
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer cancel()

		UpdateSP(ctx, cfg.db, ech, done, cfg.Addr)

		select {
		case <-ctx.Done():
			logger.Printf("when update sp error: %s\n", ctx.Err())
		default:
		}
		time.Sleep(d)
	}
}
