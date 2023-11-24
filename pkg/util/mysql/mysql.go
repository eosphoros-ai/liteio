package mysql

import (
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

// DSNProvidor provides DataSourceName
type DSNProvidor interface {
	DSN() string
}

type ConnectInfo struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	User   string `yaml:"user"`
	Passwd string `yaml:"passwd"`
	DB     string `yaml:"db"`
}

// DSN returns MySQL DataSourceName
func (m ConnectInfo) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&interpolateParams=true", m.User, m.Passwd, m.Host, m.Port, m.DB)
}

// NewMySQLEngine returns db engine of mysql
func NewMySQLEngine(mysql DSNProvidor, ping, verbose bool) (engine *xorm.Engine, err error) {
	engine, err = xorm.NewEngine("mysql", mysql.DSN())
	if err != nil {
		return
	}
	if ping {
		err = engine.Ping()
		if err != nil {
			return
		}
	}
	engine.ShowSQL(verbose)
	engine.SetMaxIdleConns(1)
	engine.DB().SetMaxOpenConns(1)
	return
}

func IsAccessDeniedError(err error) bool {
	if driverErr, ok := err.(*mysql.MySQLError); ok {
		if driverErr.Number == ER_ACCESS_DENIED_ERROR {
			return true
		}
	}
	return false
}
