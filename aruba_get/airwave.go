package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

//Airwave为配置信息：
//https://ip/LOGIN
//https://ip/api/list_view.json?list=ap_list&fv_id=0&ap_folder_id=32&expand_all=1
type Airwave struct {
	//airwave域名或者IP地址，建议使用IP
	Addr string `json:"address"`
	//airwave登录用户名
	User string `json:"user"`
	//用户密码
	Password string `json:"password"`
	//ap目录ID
	ApFolderID int `json:"ap_folder_id"`

	records []*Record
}

type ApList struct {
	Records []*Record `json:"records"`
}

//获取的record记录
type Record struct {
	ApFolderID       *ApFolderID       `json:"ap_folder_id"`
	Type             *Type             `json:"type"`
	ControllerID     *ControllerID     `json:"controller_id"`
	ICMPAddress      *ICMPAddress      `json:"icmp_address"`
	MonitoringStatus *MonitoringStatus `json:"monitoring_status"`
}

//只需要value：Aruba RAP-3WN
type Type struct {
	Value string `json:"value"`
}

type ApFolderID struct {
	ApFolderID int    `json:"ap_folder_ip"`
	Value      string `json:"value"`
}

//Value的wan口ip
type ICMPAddress struct {
	Value string `json:"value"`
}

type ControllerID struct {
	Value string `json:"value"`
}

//ap状态
type MonitoringStatus struct {
	Value string `json:"value"`
}

var skipRedirect = errors.New(`stop redirect`)

//禁用golang http模块的自动跳转301和302
func skipRedirects(req *http.Request, via []*http.Request) error {
	if len(via) > 0 {
		return skipRedirect
	}
	return nil
}

//client禁用tls证书验证，并自定义超时时间
func NewClient(timeout int) *http.Client {
	var tr http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	var Jar, _ = cookiejar.New(nil)
	client := &http.Client{
		Transport:     tr,
		CheckRedirect: skipRedirects,
		Timeout:       time.Duration(timeout) * time.Second,
		Jar:           Jar,
	}
	return client
}

//获取登录cookie
func (aw *Airwave) GetCookies(client *http.Client) (*http.Cookie, error) {
	var _login = fmt.Sprintf("https://%s/LOGIN", aw.Addr)
	var value = url.Values{}
	value.Set("credential_0", aw.User)
	value.Set("credential_1", aw.Password)
	value.Set("login", "Log In")
	value.Set("destination", "/")

	res, err := client.PostForm(_login, value)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok && urlErr.Err != skipRedirect {
			return nil, errors.New(fmt.Sprintf("when get cookie: %s", err))
		}
	}
	cookie := strings.SplitN(res.Header.Get("Set-Cookie"), "; ", 2)
	if len(cookie[0]) <= 0 {
		return nil, errors.New("get cookie length is zero")
	}

	kv := strings.Split(cookie[0], "=")
	if len(kv) < 2 {
		return nil, errors.New("cookie key or value invalid")
	}

	//fmt.Printf("%v\n", kv)
	res.Body.Close()
	return &http.Cookie{Name: kv[0], Value: kv[1]}, nil
}

//获取*Airwave，包含record切片，获取各个AP的wan ip, cookie是认证时服务器返回的
func (aw *Airwave) GetRaps(client *http.Client, cookie *http.Cookie) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s/api/list_view.json?list=ap_list&fv_id=0&ap_folder_id=%d&expand_all=1", aw.Addr, aw.ApFolderID), nil)
	if err != nil {
		return err
	}
	req.Header["host"] = []string{aw.Addr}
	req.Header["Referer"] = []string{fmt.Sprintf("https://%s/index.html", aw.Addr)}

	req.AddCookie(cookie)
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	bs, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return err
	}

	//fmt.Printf("%s\n", bs)
	var list ApList
	if err = json.Unmarshal(bs, &list); err != nil {
		return errors.New(fmt.Sprintf("when Unmarshal airwave: %s", err))
	}
	aw.records = list.Records
	return nil
}

//根据controller_id的value值非空获取rap参数
func (aw *Airwave) GetRouters(client *http.Client) ([]*Router, error) {
	var cookie, err = aw.GetCookies(client)
	if err != nil {
		return nil, err
	}
	if err = aw.GetRaps(client, cookie); err != nil {
		return nil, err
	}
	var routers = make([]*Router, 0)
	for i := 0; i < len(aw.records); i++ {
		record := aw.records[i]
		if record.ControllerID.Value == "" {
			continue
		}
		var up bool = false
		if record.MonitoringStatus.Value == "Up" {
			up = true
		}
		r := &Router{
			Code:   strings.ToUpper(record.ControllerID.Value),
			Area:   record.ApFolderID.Value,
			Wanip:  record.ICMPAddress.Value,
			status: up,
		}
		routers = append(routers, r)
	}
	return routers, nil
}
