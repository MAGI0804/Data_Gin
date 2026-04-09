package bootstrap

import (
	"fmt"
	"time"

	// GORM 的 MySQL 数据库驱动导入
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

// setupDB 初始化数据库和 ORM
func setupDB() {

	console.Info("init database ...")

	switch config.GetString("cfg.database.driver") {
	case "mysql":
		setupDBMySQL()
	default:
		console.Exit("database driver not supported")
	}

}

func setupDBMySQL() {

	configs := config.Get("cfg.database.mysql")

	dbConfigs := make(map[string]*database.DBClientConfig)

	for group := range configs.(map[string]interface{}) {
		cfgPrefix := "cfg.database.mysql." + group + "."
		username := config.GetString(cfgPrefix + "username")
		password := config.GetString(cfgPrefix + "password")
		host := config.GetString(cfgPrefix + "host")
		port := config.GetString(cfgPrefix + "port")
		db := config.GetString(cfgPrefix + "database")
		charset := config.GetString(cfgPrefix + "charset")

		// 构建 dsn 信息。DSN 全称为 Data Source Name，表示【数据源信息】
		// user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
			username, password, host, port, db, charset)

		var dbConfig gorm.Dialector
		dbConfig = mysql.New(mysql.Config{
			DSN: dsn,
		})

		var cfg database.DBClientConfig
		cfg.DBConfig = dbConfig
		cfg.LG = logger.NewGormLogger()
		cfg.MaxOpenConns = config.GetInt(cfgPrefix + "max_open_connections")
		cfg.MaxIdleConns = config.GetInt(cfgPrefix + "max_idle_connections")
		cfg.ConnMaxLifetime = time.Duration(config.GetInt(cfgPrefix+"max_life_seconds")) * time.Second

		dbConfigs[group] = &cfg
	}

	database.ConnectMySQL(dbConfigs)

	// 数据库迁移 - 自动同步表结构
	autoMigrateTables()
}

// autoMigrateTables 自动迁移数据存储相关表
func autoMigrateTables() {
	console.Info("auto migrating data storage tables...")

	// 获取默认数据库连接
	db := database.DB

	// 检查数据库连接是否成功
	if db == nil {
		console.Warning("Database connection not available, skipping auto migration")
		return
	}

	// 迁移数据存储相关表
	err := db.AutoMigrate(
		&model.User{},             // 用户表
		&model.DataSource{},       // 数据源配置表
		&model.RawData{},          // 原始数据表
		&model.ProcessedData{},    // 处理结果表
		&model.DataStatistics{},   // 数据统计表
		&model.QIMAI_ORDER_DATA{}, //企迈订单表
		&model.TokenData{},        //验证信息表
	)

	if err != nil {
		logger.Error("数据表自动迁移失败", zap.Error(err))
		console.Warning("数据表自动迁移失败: %v", err)
		return
	}

	console.Success("数据表自动迁移完成")
}
