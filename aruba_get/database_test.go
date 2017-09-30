package main

import (
	"testing"
	"time"
)

var (
	mysqlHost     = "127.0.0.1"
	mysqlPort     = "3306"
	mysqlUser     = "root"
	mysqlPassword = "123789"
	mysqlDatabase = "aruba"
)

var routerTest = &Router{
	Code: "531",
}

var tabTest = "531"
var csTest = []*Client{
	&Client{
		Name:    "",
		IP:      "10.62.15.11",
		MAC:     "01:01:01:01:01:01:01:01",
		OS:      "Win 7",
		Network: "eth1",
		AP:      "02:02:02:02:02:02:02:02",
		Role:    "Mac-Auth",
	},
	&Client{
		Name:    "",
		IP:      "10.62.15.114",
		MAC:     "03:03:03:03:03:03:03:03",
		OS:      "",
		Network: "eth1",
		AP:      "02:02:02:02:02:02:02:02",
		Role:    "client-wired",
	},
}

var userTest = &UserPassword{
	User:     "admin",
	Password: "admin",
	Admin:    true,
}

var dbTest = OpenMysql(mysqlHost, mysqlPort, mysqlUser, mysqlPassword, mysqlDatabase)

func TestInsertRouter(t *testing.T) {
	if err := InsertRouters(dbTest, []*Router{routerTest}); err != nil {
		t.Fatalf("%s\n", err)
		return
	}
	t.Logf("insert routers%s\n", routerTest)
}

func TestUpdateRouter(t *testing.T) {
	var rt = &Router{
		Code:    "531",
		Name:    "TEST",
		GateWay: "101.22.29.1",
		Area:    "531",
	}
	err := UpdateRouter(dbTest, rt)
	if err != nil {
		t.Fatalf("UpdateRouter: %s\n", err)
		return
	}
	t.Logf("UpdateRouter: %#v\n", rt)
}

func TestSelectRoutersAndTables(t *testing.T) {
	rs, err := SelectRouters(dbTest)
	if err != nil {
		t.Fatalf("SelectRouter: %s\n", err)
		return
	}
	err = CreateTables(dbTest, rs)
	if err != nil {
		t.Fatalf("CreateTables: %s\n", err)
	}
}

func TestInsertClients(t *testing.T) {
	err := InsertClients(dbTest, tabTest, csTest)
	if err != nil {
		t.Fatalf("InsertClients: %s\n", err)
		return
	}
	for i := 0; i < len(csTest); i++ {
		t.Logf("InsertClients: %#v\n", csTest[i])
	}
}

func TestSelectClients(t *testing.T) {
	ts := time.Now()
	tt := ts.Format("2006-01-02")
	cs, err := SelectClients(dbTest, tabTest, tt)
	if err != nil {
		t.Fatalf("SelectClients: %s\n", err)
		return
	}
	for i := 0; i < len(cs); i++ {
		t.Logf("SelectClients: %#v, %s, %#v\n", tabTest, tt, cs[i])
	}
}

func TestInsertUser(t *testing.T) {
	err := InsertUser(dbTest, userTest)
	if err != nil {
		t.Fatalf("InsertUser: %s\n", err)
		return
	}
	t.Logf("InsertUser: %#v\n", userTest)
}

func TestUpdateUser(t *testing.T) {
	var ut1 = &UserPassword{
		User:     "admin",
		Password: "pass",
		Admin:    false,
	}
	err := UpdateUser(dbTest, ut1)
	if err != nil {
		t.Fatalf("UpdateUser: %s\n", err)
		return
	}
	t.Logf("UpdateUser: %#v\n", ut1)
}

func TestSelectUser(t *testing.T) {
	up, err := SelectUser(dbTest, userTest.User)
	if err != nil {
		t.Fatalf("SelectUser: %s\n", err)
		return
	}
	t.Logf("SelectUser: %s, %#v\n", userTest.User, up)
}

func TestDeleteUser(t *testing.T) {
	err := DeleteUser(dbTest, userTest.User)
	if err != nil {
		t.Fatalf("DeleteUser: %s\n", err)
		return
	}

	t.Logf("DeleteUser: %s\n", userTest.User)
}
