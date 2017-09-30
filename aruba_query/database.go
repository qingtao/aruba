package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

//连接数据库
func OpenMysql(host, port, user, password, database string) *sql.DB {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		user, password, host, port, database))
	if err != nil {
		log.Fatalf("open mysql failed: %s", err)
	}
	return db
}

//用于初始化数据库和连接路由器
type Router struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	GateWay    string `json:"gateway"`
	Wanip      string `json:"wanip"`
	Area       string `json:"area"`
	SP         string `json:"service_provider"`
	AutoUpdate int    `json:"auto_update"`
}

func UpdateRouterSP(db *sql.DB, r *Router) error {
	r = ToUpper(r)
	_, err := db.Exec(`update routers set sp=? where code = ?`, r.SP, r.Code)
	return err
}

//转换Code为大写字母
func ToUpper(r *Router) *Router {
	r.Code = strings.ToUpper(r.Code)
	return r
}

func UpdateRouter(db *sql.DB, r *Router) error {
	r = ToUpper(r)
	_, err := db.Exec(`update routers set name=?, gateway=?, area=?, sp=?, autoupdate=? where code = ?`, r.Name, r.GateWay, r.Area, r.SP, r.AutoUpdate, r.Code)
	return err
}

func DeleteRouter(db *sql.DB, r *Router) error {
	r = ToUpper(r)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`delete from routers where code = ?`, r.Code); err != nil {
		return err
	}
	if _, err = tx.Exec(`drop table IF EXITS ?`, r.Code); err != nil {
		return err
	}
	return tx.Commit()

}

func SelectRouter(db *sql.DB, code string) (*Router, error) {
	code = strings.ToUpper(code)
	row := db.QueryRow(`select code, name, gateway, wanip, area, sp, autoupdate from routers where code = ?`, code)
	var router = new(Router)
	if err := row.Scan(&router.Code, &router.Name, &router.GateWay, &router.Wanip, &router.Area, &router.SP, &router.AutoUpdate); err != nil {
		return nil, err
	}
	return router, nil
}

//从tab表获取router列表
func SelectRouters(db *sql.DB) ([]*Router, error) {
	var rs = make([]*Router, 0)
	rows, err := db.Query(`select code, name, gateway, wanip, area, sp, autoupdate from routers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r = new(Router)
		if err = rows.Scan(&r.Code, &r.Name, &r.GateWay, &r.Wanip, &r.Area, &r.SP, &r.AutoUpdate); err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return rs, nil
}

//使用month(timestamp)
func SelectClientsByTime(db *sql.DB, tab, begin, end, mac string) ([]*Data, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if mac != "" {
		query := fmt.Sprintf(`select ip, mac, os, time from %s where time between ? and ? and mac = ?`, tab)
		rows, err = db.Query(query, begin, end, mac)
	} else {
		query := fmt.Sprintf(`select ip, mac, os, time from %s where time between ? and ?`, tab)
		rows, err = db.Query(query, begin, end)
	}
	if err != nil {
		return nil, err
	}

	var ds = make([]*Data, 0)
	for rows.Next() {
		var d = new(Data)
		if err = rows.Scan(&d.IP, &d.MAC, &d.OS, &d.TIME); err != nil {
			return nil, err
		}
		ds = append(ds, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return ds, nil
}

type UserPassword struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Admin    bool   `json:"admin"`
}

func InsertUser(db *sql.DB, user *UserPassword) error {
	_, err := db.Exec(`insert into users (user, password, admin) values (?, ?, ?)`, user.User, user.Password, user.Admin)
	return err
}

func DeleteUser(db *sql.DB, username string) error {
	_, err := db.Exec(`delete from users where user = ?`, username)
	return err
}

func UpdateUser(db *sql.DB, user *UserPassword) error {
	_, err := db.Exec(`update users set user=?, password=?, admin=?`,
		user.User, user.Password, user.Admin)
	return err
}

func SelectUser(db *sql.DB, username string) (*UserPassword, error) {
	row := db.QueryRow(`select user, password, admin from users where user = ?`, username)
	var up = new(UserPassword)
	if err := row.Scan(&up.User, &up.Password, &up.Admin); err != nil {
		return nil, err
	}
	return up, nil
}
