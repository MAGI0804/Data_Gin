package bootstrap

import (
	"fmt"
	"time"

	// GORM 的 MySQL 数据库驱动导入
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gin-biz-web-api/global"
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
		username := global.Credentials.DBUsername()
		password := global.Credentials.DBPassword()
		host := config.GetString(cfgPrefix + "host")
		port := config.GetString(cfgPrefix + "port")
		db := config.GetString(cfgPrefix + "database")
		charset := config.GetString(cfgPrefix + "charset")

		// 构建 dsn 信息。DSN 全称为 Data Source Name，表示【数据源信息】
		// user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Asia%%2FShanghai",
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

	prepareMethodPipelineIndexes(db)

	// 迁移数据存储相关表
	models := []interface{}{
		&model.User{},                  // 用户表
		&model.DataSource{},            // 数据源配置表
		&model.RawData{},               // 原始数据表
		&model.ProcessedData{},         // 处理结果表
		&model.DataStatistics{},        // 数据统计表
		&model.QIMAI_ORDER_DATA{},      //企迈订单表
		&model.TokenData{},             //验证信息表
		&model.YOUZAN_ORDER_DATA{},     //有赞订单表
		&model.YOUZAN_RETURN_DATA{},    //有赞退款订单表
		&model.BojunRetailOrder{},      //伯俊零售单表
		&model.SourceDefinition{},      //通用数据源配置表
		&model.RawRecord{},             //通用原始记录表
		&model.CleanTableDefinition{},  //清洗表配置表
		&model.CleanRecord{},           //通用清洗记录表
		&model.TransformRule{},         //清洗规则表
		&model.DestinationDefinition{}, //通用推送目标表
		&model.DeliveryTask{},          //通用推送任务表
		&model.PipelineRun{},           //运行记录表
		&model.DeliveryLog{},           //推送日志表
		&model.RuntimeConfig{},         //运行配置表
		&model.PipelineDefinition{},    //方法拼接流水线表
		&model.PipelineStage{},         //流水线大块阶段表
		&model.MethodStep{},            //方法步骤表
		&model.MethodParam{},           //方法入参表
		&model.MethodOutput{},          //方法出参表
		&model.StageGeneratedConfig{},  //阶段生成配置表
		&model.StepRun{},               //方法步骤运行明细表
		&model.ExcelMatchJob{},         //Excel匹配导出任务表
		&model.ExcelMatchJobLog{},      //Excel任务处理日志表
		&model.ExcelMatchScheme{},      //Excel匹配参数方案表
		// 有赞分销订单表
		&model.YouzanDistributionOrder{},
	}
	models = append(models, mallWeatherMigrationModels()...)
	err := db.AutoMigrate(models...)

	if err != nil {
		logger.Error("数据表自动迁移失败", zap.Error(err))
		console.Warning("数据表自动迁移失败: %v", err)
		return
	}

	console.Success("数据表自动迁移完成")
}

func mallWeatherMigrationModels() []interface{} {
	return []interface{}{
		&model.Mall{},
		&model.MallGeocodeRun{},
		&model.MallGeocodeCandidate{},
		&model.MallCoordinateAudit{},
		&model.ProviderRawSnapshot{},
		&model.MallWeatherFetchRun{},
		&model.MallWeatherFetchAttempt{},
		&model.MallWeatherRealtime{},
		&model.MallWeatherMinutely{},
		&model.MallWeatherHourly{},
		&model.MallWeatherDaily{},
		&model.MallWeatherAlert{},
		&model.MallWeatherAlertRelation{},
		&model.MallWeatherLifeIndex{},
		&model.MallWeatherLatest{},
		&model.MallWeatherExportProfile{},
		&model.MallWeatherExportJob{},
		&model.MallWeatherFeishuRun{},
		&model.MallWeatherSheetRow{},
		&model.AsyncJobOutbox{},
		&model.MallWeatherUserPermission{},
		&model.APIIdempotencyRecord{},
	}
}

func prepareMethodPipelineIndexes(db *gorm.DB) {
	if !db.Migrator().HasTable(&model.MethodStep{}) {
		return
	}

	const indexName = "idx_pipeline_step_code"
	if !db.Migrator().HasIndex(&model.MethodStep{}, indexName) {
		return
	}

	if err := db.Migrator().DropIndex(&model.MethodStep{}, indexName); err != nil {
		logger.Error("删除旧方法步骤唯一索引失败", zap.String("index", indexName), zap.Error(err))
		console.Warning("删除旧方法步骤唯一索引失败: %v", err)
	}
}
