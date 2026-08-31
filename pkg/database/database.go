package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
)

type DBClientConfig struct {
	DBConfig        gorm.Dialector
	LG              gormLogger.Interface
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

type MySQLClient struct {
	DB    *gorm.DB
	SQLDB *sql.DB
}

var (
	once             sync.Once
	mysqlInitErr     error
	mysqlCollections map[string]*MySQLClient
	DB               *gorm.DB // 默认 mysql 连接的 DB 对象
	SQLDB            *sql.DB  // 默认 mysql 连接中的 database/sql 包里的 *sql.DB 对象
)

func Instance(group ...string) *MySQLClient {
	if len(group) > 0 {
		if client, ok := mysqlCollections[group[0]]; ok {
			return client
		}
		console.Exit("The MySQL instance object named [%s] group could not be found!", group[0])
	}

	return mysqlCollections["default"]
}

func ConnectMySQL(configs map[string]*DBClientConfig) error {
	once.Do(func() {
		mysqlInitErr = connectMySQL(configs)
	})
	return mysqlInitErr
}

func connectMySQL(configs map[string]*DBClientConfig) error {
	if len(configs) == 0 {
		return fmt.Errorf("mysql configuration is empty")
	}
	clients := make(map[string]*MySQLClient, len(configs))
	for group, cfg := range configs {
		if cfg == nil {
			closeMySQLClients(clients)
			return fmt.Errorf("mysql group %s configuration is nil", group)
		}
		if err := validateDBClientConfig(cfg); err != nil {
			closeMySQLClients(clients)
			return fmt.Errorf("mysql group %s: %w", group, err)
		}
		client, err := NewMysqlClient(cfg.DBConfig, cfg.LG)
		if err != nil {
			closeMySQLClients(clients)
			return fmt.Errorf("connect mysql group %s: %w", group, err)
		}
		configureConnectionPool(client.SQLDB, cfg)
		clients[group] = client
	}
	if clients["default"] == nil {
		closeMySQLClients(clients)
		return fmt.Errorf("default mysql group is missing")
	}
	mysqlCollections = clients
	setSimpleHelper()
	return nil
}

func validateDBClientConfig(cfg *DBClientConfig) error {
	if cfg.DBConfig == nil {
		return fmt.Errorf("dialector is nil")
	}
	if cfg.MaxOpenConns <= 0 {
		return fmt.Errorf("max open connections must be positive")
	}
	if cfg.MaxIdleConns < 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return fmt.Errorf("max idle connections must be between zero and max open connections")
	}
	if cfg.ConnMaxLifetime <= 0 {
		return fmt.Errorf("connection max lifetime must be positive")
	}
	if cfg.ConnMaxIdleTime <= 0 || cfg.ConnMaxIdleTime > cfg.ConnMaxLifetime {
		return fmt.Errorf("connection max idle time must be positive and not exceed max lifetime")
	}
	if cfg.ConnectTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}
	if cfg.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}
	if cfg.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}
	return nil
}

func configureConnectionPool(sqlDB *sql.DB, cfg *DBClientConfig) {
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

func NewMysqlClient(dbConfig gorm.Dialector, lg gormLogger.Interface) (*MySQLClient, error) {
	mysql := &MySQLClient{}

	var err error
	mysql.DB, err = gorm.Open(dbConfig, &gorm.Config{Logger: lg})
	if err != nil {
		return nil, fmt.Errorf("open gorm connection: %w", err)
	}

	// 获取底层的 sqlDB
	// *gorm.DB 对象的 DB() 方法，可以直接获取到 database/sql 包里的 *sql.DB 对象
	mysql.SQLDB, err = mysql.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql connection: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mysql.SQLDB.PingContext(ctx); err != nil {
		_ = mysql.SQLDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return mysql, nil
}

// Close 关闭所有数据库连接
func Close() {
	closeMySQLClients(mysqlCollections)
}

func closeMySQLClients(clients map[string]*MySQLClient) {
	for group, mysql := range clients {
		if mysql == nil || mysql.SQLDB == nil {
			continue
		}
		if err := mysql.SQLDB.Close(); err != nil {
			zap.L().Error("MySQL", zap.String("group", group), zap.Error(err))
		}
	}
}

func setSimpleHelper() {
	defaultInstance := Instance()
	if defaultInstance != nil {
		DB = defaultInstance.DB
		SQLDB = defaultInstance.SQLDB
	}
}

// DropAllTables 删除所有表（其实是直接删库跑路，😊）
// most dangerous !!!
func DropAllTables(group ...string) error {
	var err error
	console.Danger("Most dangerous!")

	switch config.GetString("cfg.database.driver") {
	case "mysql":
		err = dropMysqlDatabase(group...)
	default:
		console.Exit("database driver not supported")
	}

	return err
}

// dropMysqlDatabase 删除数据表
func dropMysqlDatabase(group ...string) error {
	dbname := CurrentDatabase(group...)
	db := Instance(group...).DB
	var tables []string

	// 读取所有数据表
	err := db.Table("information_schema.tables").
		Where("table_schema = ?", dbname).
		Pluck("table_name", &tables).
		Error
	if err != nil {
		return err
	}

	// 暂时关闭外键检测
	db.Exec("SET foreign_key_checks = 0;")

	// 删除所有表
	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return err
		}
	}

	// 开启 MySQL 外键检测
	db.Exec("SET foreign_key_checks = 1;")
	return nil
}

// CurrentDatabase 返回当前数据库名称
func CurrentDatabase(group ...string) string {
	return Instance(group...).DB.Migrator().CurrentDatabase()
}

// TableName 获取当前对象的表名称
// eg：database.TableName(&model.User{})
// output: "users"
func TableName(obj interface{}, group ...string) string {
	db := Instance(group...).DB
	stmt := &gorm.Statement{DB: db}
	_ = stmt.Parse(obj)
	return stmt.Schema.Table
}
