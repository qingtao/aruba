package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Data struct {
	IP   string
	MAC  string
	OS   string
	TIME string
}

type byTime []*Data

const timeFormat = "2006-01-02 15:04:05"

func (a byTime) Len() int      { return len(a) }
func (a byTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool {
	ai, _ := time.Parse(timeFormat, a[i].TIME)
	bi, _ := time.Parse(timeFormat, a[j].TIME)
	return ai.Before(bi)
}

type Clients struct {
	Code string  `json:"code"`
	Data []*Data `json:"data"`
}

//mysql database configure
type MysqlDB struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DB       string `json:"db"`
}

//配置文件
type Config struct {
	Addr     string  `json:"addr"`
	Duration int     `json:'duration"`
	Database MysqlDB `json:"database"`
	cache    string
	db       *sql.DB
}

func (cfg *Config) OpenMysql() {
	db := OpenMysql(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DB)
	cfg.db = db
}

//生成配置文件到file
func CreateConfigFile(file string, cfg *Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(file, b, 0666); err != nil {
		fmt.Println("generate configure", err)
		return err
	}
	return nil
}

//读取配置文件
func ReadConfigFile(file string) (*Config, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var cfg Config
	//解析json
	if err = json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

//获取程序当前目录
func Basedir() string {
	p, err := filepath.Abs(os.Args[0])
	if err != nil {
		log.Fatalln(err)
	}
	return filepath.Dir(p)
}

//创建指定目录
func Mkdir(dir string) {
	if _, err := os.Stat(dir); err != nil {
		err = os.MkdirAll(dir, os.ModeDir|0755)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func NewLogger(f string) *log.Logger {
	file, err := os.Create(f)
	if err != nil {
		log.Fatalln(err)
	}
	return log.New(file, "", 1|2)
}
