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

var tabTest = "531"

var dbTest = OpenMysql(mysqlHost, mysqlPort, mysqlUser, mysqlPassword, mysqlDatabase)

func TestSelectRouters(t *testing.T) {
	_, err := SelectRouters(dbTest)
	if err != nil {
		t.Fatalf("SelectRouter: %s\n", err)
		return
	}
}

func TestSelectClientsOneDay(t *testing.T) {
	bg := time.Now()
	bgf := bg.Format("2006-01-02")
	cs, err := SelectClients(dbTest, tabTest, bgf, "")
	if err != nil {
		t.Fatalf("SelectClients: %s\n", err)
		return
	}
	for i := 0; i < len(cs); i++ {
		t.Logf("SelectClients: %s, %s, %#v\n", tabTest, bgf, cs[i])
	}
}

func TestSelectClientsMultiDays(t *testing.T) {
	end := time.Now()
	bg := time.Now().AddDate(0, 0, -2)

	bgf := bg.Format("2006-01-02")
	endf := end.Format("2006-01-02")

	cs, err := SelectClients(dbTest, tabTest, bgf, endf)
	if err != nil {
		t.Fatalf("SelectClients: %s\n", err)
		return
	}
	for i := 0; i < len(cs); i++ {
		t.Logf("SelectClients: %s, %s, %s, %#v\n", tabTest, bgf, endf, cs[i])
	}
}

func TestSelectClientsBeginLTEnd(t *testing.T) {
	bg := time.Now()
	end := time.Now().AddDate(0, 0, -2)

	bgf := bg.Format("2006-01-02")
	endf := end.Format("2006-01-02")

	cs, err := SelectClients(dbTest, tabTest, bgf, endf)
	if err != nil {
		t.Logf("SelectClients: %s\n", err)
		return
	}
	for i := 0; i < len(cs); i++ {
		t.Logf("SelectClients: %#v, %s, %#v, %#v\n", tabTest, bgf, endf, cs[i])
	}
}

func TestSelectClientsOneDayBeginEnd(t *testing.T) {
	bg := time.Now()
	bgf := bg.Format("2006-01-02")
	cs, err := SelectClients(dbTest, tabTest, bgf, bgf)
	if err != nil {
		t.Fatalf("SelectClients: %s\n", err)
		return
	}
	for i := 0; i < len(cs); i++ {
		t.Logf("SelectClients: %s, %s, %#v\n", tabTest, bgf, cs[i])
	}
}
