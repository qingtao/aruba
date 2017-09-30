package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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
	Debug:   false,
	Hour:    10,
	Minute:  20,
	Timeout: 10,
	Airwave: &Airwave{
		Addr:       "5.5.5.16",
		User:       "user",
		Password:   "password",
		ApFolderID: 32,
	},
	Rap3: &Rap3{
		Path:   "swarm.cgi",
		User:   "admin",
		Passwd: "passwd",
		Cmd:    `%27show%20clients%20wired%27`,
		OnlyPC: true,
		IncludeMac: []string{
			`c0:3f:d5:7e:fd:ee`,
			`44:37:e6:ce:78:8a`,
		},
	},
	Database: &MysqlDB{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "root",
		Password: "123456",
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
		switch {
		case cfg.Airwave == nil:
			fmt.Println("configure contain invaild airwave config")
		case cfg.Rap3 == nil:
			fmt.Println("configure contain invalid rap3 config")
		case cfg.Database == nil:
			fmt.Println("configure contain invalid database config")
		case cfg.Hour < 0 || cfg.Hour > 4:
			fmt.Println("hour must between 0 and 4")
		default:
			fmt.Printf("%s is ok\n", CONF)
		}
		return
	}

	fi := filepath.Join(tmpDir, "aruba.log")

	var logger = NewLogger(fi)
	logger.Printf("%s started\n", os.Args[0])
	logger.Printf("version: %s\n", version)
	pid, pidFile := os.Getpid(), filepath.Join(tmpDir, "aruba.pid")
	logger.Printf("pid: %d, path: %s\n", pid, pidFile)
	if err := ioutil.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0666); err != nil {
		logger.Printf("write pid: %s\n", err)
		return
	}

	//设置数据库连接
	cfg.OpenMysql()
	logger.Printf("connect to mysql %s:%s\n", cfg.Database.Host, cfg.Database.Port)
	//启动定时器
	tick, err := Cron(cfg.Hour, cfg.Minute)
	if err != nil {
		log.Fatalln(err)
	}

	logger.Printf("cron_jobs at %d:%d\n", cfg.Hour, cfg.Minute)

	client := NewClient(cfg.Timeout)
	var wg sync.WaitGroup

	for range tick {
		//打印日志
		logger.Println("--------")
		logger.Println("starting connect remote airwave")

		awRs, err := cfg.Airwave.GetRouters(client)
		if err != nil {
			logger.Println("get routers from airwave error: ", err)
			continue
		}
		logger.Printf("get routers number: %d\n", len(awRs))
		logger.Println("--------")

		rss, err := Diff(cfg.db, awRs)
		if err != nil {
			logger.Println("diff routers error: ", err)
			continue
		}
		for i := 0; i < len(rss); i++ {
			logger.Printf("new router found, code: %s\n", rss[i].Code)
		}
		if err = SyncRouters(cfg.db, rss); err != nil {
			logger.Println("add new routers into database error: ", err)
			continue
		}
		logger.Println("add new routers success")
		logger.Println("--------")
		logger.Println("update router and show client wired starting")
		for _, r := range awRs {
			if r.AutoUpdate != 0 {
				err = UpdateRouter(cfg.db, r)
				if err != nil {
					logger.Printf("update router %s failed: %s\n", r.Code, err)
				}
				if cfg.Debug {
					logger.Printf("update router %s success\n", r.Code)
				}
			}
			if !r.status {
				logger.Printf("%s status is %v, skip\n", r.Code, r.status)
				continue
			}
			wg.Add(1)
			//为每个路由器启动一个goroutine
			go func(client *http.Client, router *Router) {
				defer wg.Done()
				//获取在线的客户端
				cs, err := cfg.Rap3.GetClientsWired(client, router.Wanip)
				if err != nil {
					logger.Printf("code %s show clients wired failed by wan ip %s\n", router.Code, router.Wanip)
					if router.GateWay == "" {
						logger.Printf("code %s gateway is not exists\n", router.Code)
						return
					}

					logger.Printf("code %s retry by gateway %s\n", router.Code, router.GateWay)
					cs, err = cfg.Rap3.GetClientsWired(client, router.GateWay)
					if err != nil {
						logger.Printf("code %s show clients wired retry use gateway failed\n", router.Code)
						return
					}
					logger.Printf("code %s retry by gateway success\n", router.Code)
				} else if cfg.Debug {
					logger.Printf("code %s show clients wired by wan ip %s\n", router.Code, router.GateWay)
				}
				//插入数据到数据库表，表名为r.Code
				if err = InsertClients(cfg.db, router.Code, cs); err != nil {
					logger.Printf("code %s insert data failed: %s\n", router.Code, err)
				} else if cfg.Debug {
					logger.Printf("code %s insert data success, first: %s\n", router.Code, cs)
				}
			}(client, r)
		}
		wg.Wait()

		logger.Println("--------")
		/*
			logger.Println("update service provider of network")
			for _, r := range awRs {
				if r.AutoUpdate == 0 {
						if cfg.Debug {
							logger.Printf("%s do not need auto update\n", r.Code)
						}
					logger.Printf("%s do not need auto update\n", r.Code)
					continue
				}
				if err = UpdateRouterSP(cfg.db, r); err != nil {
					logger.Printf("when update %s sp error: %s\n", r.Code, err)
					continue
				}
				logger.Printf("%s update service provider success: %s\n", r.Code, r.SP)
			}
		*/

		logger.Println("jobs end.")
		logger.Println("--------")
	}
}
