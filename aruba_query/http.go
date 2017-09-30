package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Server struct {
	*http.ServeMux
	IPS    []net.IP
	IPNET  []*net.IPNet
	Logger *log.Logger
}

func NewServer(wl string, l *log.Logger) (*Server, error) {
	mux := http.NewServeMux()
	str, err := ioutil.ReadFile(wl)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	var ipnet []*net.IPNet

	for _, allow := range strings.Fields(string(str)) {
		_, ipNet, err := net.ParseCIDR(allow)
		if err != nil {
			allowIP := net.ParseIP(allow)
			if allowIP == nil {
				return nil, errors.New(fmt.Sprintf("contain not a valid ip address: %s\n", err))
			}
			ips = append(ips, allowIP)
			continue
		}
		ipnet = append(ipnet, ipNet)
	}
	var srv = new(Server)
	srv.ServeMux = mux
	srv.IPS = ips
	srv.IPNET = ipnet
	srv.Logger = l

	return srv, nil
}

//检查ip权限
func (srv *Server) Allowed(ip net.IP) bool {
	for i := 0; i < len(srv.IPNET); i++ {
		if srv.IPNET[i].Contains(ip) {
			return true
		}
	}
	for j := 0; j < len(srv.IPS); j++ {
		if srv.IPS[j].Equal(ip) {
			return true
		}
	}
	return false
}

//实现ServeHTTP: 并检查IP地址是否允许访问
func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var ip string
	if xforwarfor := r.Header.Get("X-Forward-For"); xforwarfor != "" {
		ip = xforwarfor
		if srv.Allowed(net.ParseIP(xforwarfor)) {
			srv.ServeMux.ServeHTTP(w, r)
		}
	} else if tcpAddr, _ := net.ResolveTCPAddr("tcp", r.RemoteAddr); srv.Allowed(tcpAddr.IP) {
		ip = tcpAddr.IP.String()
		srv.ServeMux.ServeHTTP(w, r)
	} else {
		srv.Logger.Printf("client %s connect not allowed\n", ip)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "ip no allowed: %s", ip)
	}
}

func (cfg *Config) UpdateRouter(w http.ResponseWriter, r *http.Request, lg *log.Logger) {
	/*
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	*/
	if err := r.ParseForm(); err != nil {
		lg.Printf("[Error] client %s: %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	codeValue := r.FormValue("code")
	if codeValue == "" {
		lg.Printf("[Error] client %s: code is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "code is empty")
		return
	}
	code := strings.ToUpper(codeValue)
	router, err := SelectRouter(cfg.db, code)
	if err != nil {
		lg.Printf("[Error] client %s: select code %s %s\n", r.RemoteAddr, code, err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "code %s not found in data", codeValue)
		return
	}

	router.Name = r.FormValue("name")
	router.GateWay = r.FormValue("gateway")
	router.Wanip = r.FormValue("wanip")
	area := r.FormValue("area")
	if area != "" {
		router.Area = area
	}
	router.SP = r.FormValue("service_provider")
	autoupdate := r.FormValue("auto_update")
	if autoupdate == "yes" {
		router.AutoUpdate = 1
	} else {
		router.AutoUpdate = 0
	}

	if err := UpdateRouter(cfg.db, router); err != nil {
		lg.Printf("update router error: %s\n", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, `{"status": "update router success"}`)
}

func (cfg *Config) GetRouters(w http.ResponseWriter, r *http.Request, lg *log.Logger) {
	routers, err := SelectRouters(cfg.db)
	if err != nil {
		lg.Printf("[Error] client %s: select routers %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	b, err := json.MarshalIndent(routers, "", "  ")
	if err != nil {
		lg.Println("json", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "print json error")
		return
	}
	callback := r.FormValue("callback")
	var s string
	if callback != "" {
		s = fmt.Sprintf("%s(%s)", callback, b)
	} else {
		s = string(b)
	}
	lg.Printf("client %s get routers success\n", r.RemoteAddr)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "%s", s)
}

type analysis struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Gateway string `json:"gateway"`
	Count   int    `json:"count"`
}

//实现sort.Sort接口
type byCount []*analysis

func (a byCount) Len() int           { return len(a) }
func (a byCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCount) Less(i, j int) bool { return a[i].Count < a[j].Count }

//分析所有路由器上指定月份（如：2016-04）的在线客户端次数
func (cfg *Config) AnalysisOfCounts(w http.ResponseWriter, r *http.Request, lg *log.Logger) {
	if err := r.ParseForm(); err != nil {
		lg.Printf("[Error] client %s: %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	year := r.FormValue("year")
	//如果year为空, 则等于当前年份
	if year == "" {
		year = fmt.Sprintf("%d", time.Now().Year())
	}

	//month不能为空
	month := r.FormValue("month")

	if month == "" {
		lg.Printf("[Error] client %s: month is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "month is empty")
		return
	}
	//开始时间为每月的第一天
	begin := fmt.Sprintf("%s-%s-01", year, month)
	const queryFormat = `2006-01-02`
	t, err := time.Parse(queryFormat, begin)
	if err != nil {
		lg.Printf("analysis of month %s error: %s\n", begin, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}

	//结束时间为下一个月的第一天
	end := t.AddDate(0, 1, 0).Format(queryFormat)

	var res string
	// 执行查询操作
	rs, err := SelectRouters(cfg.db)
	if err != nil {
		lg.Printf("analysis of month %s error: %s\n", begin, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}
	var as = make([]*analysis, 0)
	for i := 0; i < len(rs); i++ {
		cs, err := SelectClientsByTime(cfg.db, rs[i].Code, begin, end, "")
		a := &analysis{
			Code:    rs[i].Code,
			Name:    rs[i].Name,
			Gateway: rs[i].GateWay,
		}
		if err != nil {
			lg.Printf("analysis of month %s error: %s %s\n", begin, rs[i].Code, err)
			a.Count = -1
		} else {
			a.Count = len(cs)
		}
		as = append(as, a)
		if len(as) == 0 {
			lg.Print("analysis of month error: the length of result is 0")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "the length of result is 0")
			return
		}

		//反向排序，count值大的在前
		sort.Sort(sort.Reverse(byCount(as)))

		b, err := json.MarshalIndent(as, "", "  ")
		if err != nil {
			lg.Printf("analysis of month error: %s\n", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, err)
			return
		}
		res = string(b)
	}

	//jquery AJAX callback随机函数名
	callback := r.FormValue("callback")
	var s string
	if callback != "" {
		s = fmt.Sprintf("%s(%s)", callback, res)
	} else {
		s = res
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	fmt.Fprintf(w, "%s", s)
}

//获取单台路由器的指定月份统计信息
func (cfg *Config) AnalysisOfRouter(w http.ResponseWriter, r *http.Request, lg *log.Logger) {
	if err := r.ParseForm(); err != nil {
		lg.Printf("[Error] client %s: %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	//code不能为空值
	code := r.FormValue("code")
	if code == "" {
		lg.Printf("[Error] client %s: code is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "code is empty")
		return
	}
	code = strings.ToUpper(code)

	year := r.FormValue("year")
	if year == "" {
		year = fmt.Sprintf("%d", time.Now().Year())
	}

	//month不能是空
	month := r.FormValue("month")
	if month == "" {
		lg.Printf("[Error] client %s: month is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "month is empty")
		return
	}
	begin := fmt.Sprintf("%s-%s-01", year, month)

	const queryFormat = `2006-01-02`
	t, err := time.Parse(queryFormat, begin)
	if err != nil {
		lg.Printf("analysis of %s error: begin %s\n", code, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}

	end := t.AddDate(0, 1, 0).Format(queryFormat)

	ds, err := SelectClientsByTime(cfg.db, code, begin, end, "")
	if err != nil {
		lg.Printf("analysis of %s error: %s\n", code, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}
	sort.Sort(byTime(ds))
	cs := &Clients{Code: code, Data: ds}
	b, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		lg.Printf("analysis of %s error: %s\n", code, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}

	callback := r.FormValue("callback")
	var s string
	if callback != "" {
		s = fmt.Sprintf("%s(%s)", callback, b)
	} else {
		s = string(b)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	fmt.Fprintf(w, "%s", s)
}

//查询指定设备在目标月份在线明细
func (cfg *Config) AnalysisOfClient(w http.ResponseWriter, r *http.Request, lg *log.Logger) {
	if err := r.ParseForm(); err != nil {
		lg.Printf("[Error] client %s: %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	if code == "" {
		lg.Printf("[Error] client %s: code is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "code is empty")
		return
	}
	code = strings.ToUpper(code)

	//mac地址必须提供
	mac := r.FormValue("mac")
	if mac == "" {
		lg.Printf("[Error] client %s: mac is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "mac is empty")
		return
	}
	mac = strings.ToLower(mac)

	year := r.FormValue("year")
	if year == "" {
		year = fmt.Sprintf("%d", time.Now().Year())
	}

	month := r.FormValue("month")
	if month == "" {
		lg.Printf("[Error] client %s: month is empty\n", r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "month is empty")
		return
	}
	begin := fmt.Sprintf("%s-%s-01", year, month)

	const queryFormat = `2006-01-02`
	t, err := time.Parse(queryFormat, begin)
	if err != nil {
		lg.Printf("analysis of %s error: begin %s\n", code, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}

	end := t.AddDate(0, 1, 0).Format(queryFormat)

	ds, err := SelectClientsByTime(cfg.db, code, begin, end, mac)
	if err != nil {
		lg.Printf("analysis of %s error: %s\n", code, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}
	sort.Sort(byTime(ds))
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		lg.Printf("analysis of %s error: %s\n", code, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, err)
		return
	}

	callback := r.FormValue("callback")
	var s string
	if callback != "" {
		s = fmt.Sprintf("%s(%s)", callback, b)
	} else {
		s = string(b)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	fmt.Fprintf(w, "%s", s)
}

func (srv *Server) Listen(addr string, cfg *Config, logger *log.Logger) {
	srv.HandleFunc("/admin/r/g", func(w http.ResponseWriter, r *http.Request) {
		cfg.GetRouters(w, r, logger)
	})
	srv.HandleFunc("/admin/r/u", func(w http.ResponseWriter, r *http.Request) {
		cfg.UpdateRouter(w, r, logger)
	})

	srv.HandleFunc("/a/counts", func(w http.ResponseWriter, r *http.Request) {
		cfg.AnalysisOfCounts(w, r, logger)
	})
	srv.HandleFunc("/a/router", func(w http.ResponseWriter, r *http.Request) {
		cfg.AnalysisOfRouter(w, r, logger)
	})
	srv.HandleFunc("/a/client", func(w http.ResponseWriter, r *http.Request) {
		cfg.AnalysisOfClient(w, r, logger)
	})

	ui := Basedir() + "/ui"
	srv.Handle("/", http.FileServer(http.Dir(ui)))

	log.Fatalln(http.ListenAndServe(addr, srv))
}
