package main

import (
	"testing"
)

var ipTest = "119.163.182.204"

var cfgTest = "d:/go/src/aruba/conf/config.json"

func TestReadConfig(t *testing.T) {
	cfg, err := ReadConfigFile(cfgTest)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v\n", cfg)
	login, err := cfg.Rap3.NewRequestURL("login", ipTest)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s\n", login)
	show, err := cfg.Rap3.NewRequestURL("support", ipTest, "360ef8eb47284de6e49efe5dbbf4aaf")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s\n", show)
}
