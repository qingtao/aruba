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
	SP         string `json:"servcie_provider"`
	AutoUpdate int    `json:"auto_update"`
	/*
		User     string `json:"user"`
		Password string `json:"password"`
	*/

	status bool
}

//转换Code为大写字母
func ToUpper(r *Router) *Router {
	r.Code = strings.ToUpper(r.Code)
	return r
}

//添加router信息到routers
func InsertRouters(db *sql.DB, rs []*Router) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var query = `insert into routers (code, name, gateway, wanip, area, sp, autoupdate) values (?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil
	}
	defer stmt.Close()

	for i := 0; i < len(rs); i++ {
		r := ToUpper(rs[i])
		r.AutoUpdate = 1
		_, err := stmt.Exec(r.Code, r.Name, r.GateWay, r.Wanip, r.Area, r.SP, r.AutoUpdate)
		if err != nil {
			if el := tx.Rollback(); el != nil {
				err = el
			}
			return err
		}
	}
	return tx.Commit()
}

func UpdateRouter(db *sql.DB, r *Router) error {
	r = ToUpper(r)
	_, err := db.Exec(`update routers set name=?, gateway=?, wanip=?, area=?, autoupdate=? where code = ?`, r.Name, r.GateWay, r.Wanip, r.Area, r.AutoUpdate, r.Code)
	return err
}

func UpdateRouterSP(db *sql.DB, r *Router) error {
	r = ToUpper(r)
	_, err := db.Exec(`update routers set sp=? where code = ?`, r.SP, r.Code)
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

//根据routers列表创建每个表
func CreateTables(db *sql.DB, rs []*Router) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for i := 0; i < len(rs); i++ {
		r := ToUpper(rs[i])
		var query = fmt.Sprintf("create table IF NOT EXISTS %s like template_client", r.Code)
		_, err := tx.Exec(query)
		if err != nil {
			if e1 := tx.Rollback(); e1 != nil {
				err = e1
			}
			return err
		}
	}
	return tx.Commit()
}

func Diff(db *sql.DB, rs []*Router) ([]*Router, error) {
	dbrouters, err := SelectRouters(db)
	if err != nil {
		return nil, err
	}

	var routers = make([]*Router, 0)
DIFF:
	for _, r := range rs {
		for _, dbr := range dbrouters {
			if r.Code == dbr.Code {
				r.Name = dbr.Name
				r.GateWay = dbr.GateWay
				if dbr.Area != "" {
					r.Area = dbr.Area
				}
				r.AutoUpdate = dbr.AutoUpdate
				continue DIFF
			}
		}
		routers = append(routers, r)
	}
	return routers, nil
}

func SyncRouters(db *sql.DB, rs []*Router) error {
	if err := CreateTables(db, rs); err != nil {
		return err
	}
	if err := InsertRouters(db, rs); err != nil {
		return err
	}
	return nil
}

//添加client信息到表tab
func InsertClients(db *sql.DB, tab string, cs []*Client) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var query = fmt.Sprintf(`insert into %s (name, ip, mac, os, network, ap, role)
	values (?, ?, ?, ?, ?, ?, ?)`, tab)

	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil
	}
	defer stmt.Close()

	for i := 0; i < len(cs); i++ {
		c := cs[i]
		_, err = stmt.Exec(c.Name, c.IP, c.MAC, c.OS, c.Network, c.AP, c.Role)
		if err != nil {
			if e1 := tx.Rollback(); e1 != nil {
				err = e1
			}
			return err
		}
	}
	return tx.Commit()
}

//使用month(timestamp)
func SelectClientsByTime(db *sql.DB, tab, begin, end string) ([]*Client, error) {
	var query = fmt.Sprintf(`select name, ip, mac, os, network, ap, role from %s where time between ? and ?`, tab)
	rows, err := db.Query(query, begin, end)
	if err != nil {
		return nil, err
	}
	var cs = make([]*Client, 0)
	for rows.Next() {
		var c = new(Client)
		if err = rows.Scan(&c.Name, &c.IP, &c.MAC, &c.OS, &c.Network, &c.AP, &c.Role); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return cs, nil
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
