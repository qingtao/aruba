package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

var aw = &Airwave{
	Addr:       "5.5.2.46",
	User:       "J-Admin",
	Password:   "SF@admin123",
	ApFolderID: 32,
}

var (
	mysqlHost     = "127.0.0.1"
	mysqlPort     = "3306"
	mysqlUser     = "root"
	mysqlPassword = "123789"
	mysqlDatabase = "aruba"
)

func TestAw(t *testing.T) {
	rs, err := aw.GetRouters()
	if err != nil {
		t.Fatalf("%s\n", err)
		return
	}
	db := OpenMysql(mysqlHost, mysqlPort, mysqlUser, mysqlPassword, mysqlDatabase)
	rss, err := Diff(db, rs)
	if err != nil {
		t.Fatalf("%s\n", err)
		return
	}
	for i := 0; i < len(rss); i++ {
		t.Logf("%#v\n", rss[i])
	}
	if err = SyncRouters(db, rss); err != nil {
		t.Fatalf("%s\n", err)
		return
	}
}

func TestAWCookie(t *testing.T) {
	c := NewClient(5)
	ctx, cancel := context.WithCancel(context.Background())
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		cancel()
		if len(via) > 0 {
			return errors.New("02013bac")
		}
		return nil
	}

	var _login = fmt.Sprintf("https://%s/LOGIN", aw.Addr)
	var value = url.Values{}
	value.Set("credential_0", aw.User)
	value.Set("credential_1", aw.Password)
	value.Set("login", "Log In")
	value.Set("destination", "/")

	req, err := http.NewRequest("POST", _login, nil)
	if err != nil {
		t.Fatalf("%s\n", err)
	}

	req.Form = value
	req.WithContext(ctx)
	res, err := c.PostForm(_login, value)
	if err != nil {
		urlErr, ok := err.(*url.Error)
		if !ok {
			t.Fatalf("when get cookie: %s\n", err)
		}
		if urlErr.Err != context.Canceled {
			t.Logf("%s, want:%s\n", urlErr.Err, context.Canceled)
		}
		t.Logf("%s\n", urlErr.Err)
	}
	t.Logf("%s\n", res.Request.URL)
	cookie := strings.SplitN(res.Header.Get("Set-Cookie"), "; ", 2)
	if len(cookie[0]) <= 0 {
		t.Fatalf("get cookie length is zero")
	}

	kv := strings.Split(cookie[0], "=")
	if len(kv) < 2 {
		t.Fatalf("cookie key or value invalid")
	}

	//fmt.Printf("%v\n", kv)
	t.Logf("Name: %s, Values: %s\n", kv[0], kv[1])

}
