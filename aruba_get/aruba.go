package main

import (
	//"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

type Client struct {
	Name    string
	IP      string
	MAC     string
	OS      string
	Network string
	AP      string
	Role    string
}

func (c Client) String() string {
	return fmt.Sprintf("Name:%s, IP:%s, MAC:%s, OS:%s, Role:%s", c.Name, c.IP, c.MAC, c.OS, c.Role)
}

/*
	Regexp used to split column
	(Name)\s{2}(IP Address)\s{2}(Mac Address)\s{2}(OS)\s{2}(Network)\s+(Access Point)\s{2}(Role)\s{2}.*
	Name  IP Address   MAC Address        OS      Network  Access Point       Role           Speed (mbps)
	----  ----------   -----------        --      -------  ------------       ----           ------------
*/
var re = regexp.MustCompile(`(.+)\s{2}([.0-9]+)\s{2,}([:a-zA-Z0-9]+)\s{2}(.+)\s{2}(eth[012])\s+([:a-zA-Z0-9]+)\s{2}(.+)\s{2}.*`)

// SSID's Data contain authentication key
type SSID struct {
	Name string `xml:"name,attr"`
	Data string `xml:",chardata"`
}

func (id SSID) String() string {
	return fmt.Sprintf("name: %s, data: %s", id.Name, id.Data)
}

type Login struct {
	XMLName xml.Name `xml:"re"`
	SSID    []SSID   `xml:"data"`
}

func (a Login) String() string {
	var s string
	if len(a.SSID) > 0 {
		for i := 0; i < len(a.SSID); i++ {
			s += fmt.Sprintf("%s\n", a.SSID[i])
		}
	}
	return s
}

//获取SID
func (a Login) GetSID() string {
	if len(a.SSID) > 0 {
		for _, v := range a.SSID {
			if v.Name == "sid" {
				return v.Data
			}
		}
	}
	return ""
}

type Rap3 struct {
	Path   string `json:"path"`
	User   string `json:"user"`
	Passwd string `json:"passwd"`
	Cmd    string `json:"cmd"`
	//特殊MAC地址，查询同一厂商的设备，用于识别rap没有识别的电脑
	IncludeMac []string `json:"include_mac"`
	OnlyPC     bool     `json:"only_pc"`
}

// create new url
func (rap Rap3) NewRequestURL(op, ip, sid string) (string, error) {
	p := fmt.Sprintf("https://%s:4343/%s", ip, rap.Path)
	var u string
	switch op {
	//opcode = login
	case "login":
		u = fmt.Sprintf("%s?opcode=login&user=%s&passwd=%s&refresh=false",
			p, rap.User, rap.Passwd)
	//opcode = support
	case "support":
		if sid == "" {
			return "", errors.New(`sid is ""`)
		}
		u = fmt.Sprintf("%s?opcode=support&sid=%s&cmd=%s&refresh=false",
			p, sid, rap.Cmd)
	}
	//fmt.Println(u)
	return u, nil
}

func (rap Rap3) TrimMAC() {
	for i := 0; i < len(rap.IncludeMac); i++ {
		if len(rap.IncludeMac[i]) >= 8 {
			rap.IncludeMac[i] = strings.ToLower(string(rap.IncludeMac[i][:8]))
		}
	}
}

func (rap Rap3) GetClientsWired(client *http.Client, ip string) (c []*Client, err error) {
	//登录地址
	lu, err := rap.NewRequestURL("login", ip, "")

	if err != nil {
		return nil, err
	}
	login, err := client.Get(lu)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("login failed: %s\n", err))
	}

	b, err := ioutil.ReadAll(login.Body)
	defer login.Body.Close()
	//
	//fmt.Printf("%s\n", b)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("read body failed: %s\n", err))
	}

	var li Login
	//  parse step of login
	if err = xml.Unmarshal(b, &li); err != nil {
		return nil, errors.New(fmt.Sprintf("xml unmarshal failed: %s\n", err))
	}

	// use aruba, run command: show clients wired
	cu, err := rap.NewRequestURL("support", ip, li.GetSID())
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", cu, nil)

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("show clients wired failed: %s\n", err))
	}
	bs, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("read clients list failed: %s\n", err))
	}

	// parse lines in body
	cs := strings.Split(string(bs), "\n")

	if cs != nil {
		// split cs in lines
		for _, line := range cs {
			// get client name, ip, mac...
			cl := re.FindStringSubmatch(line)
			if cl != nil {
				isPC := true
				// append to c, if cl is matched and length is 8
				if len(cl) == 8 {
					for i := 0; i < len(cl); i++ {
						// delete space prefix or suffix
						cl[i] = strings.TrimSpace(cl[i])
					}
					// when only_pc is set false, all clients return
					if rap.OnlyPC {
						switch {
						case strings.HasPrefix(cl[4], "Win"):
							isPC = true
						case cl[2] == `0.0.0.0`:
							isPC = false
						default:
							for _, mac := range rap.IncludeMac {
								if strings.HasPrefix(strings.ToLower(cl[3]), mac) {
									isPC = true
									break
								}
							}
							isPC = false
						}
					}
				}
				if isPC {
					c = append(c, &Client{cl[1], cl[2], cl[3], cl[4], cl[5], cl[6], cl[7]})
				}
			}
		}
	}
	return
}
