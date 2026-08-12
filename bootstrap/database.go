package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	// GORM 的 MySQL 数据库驱动导入
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

const schemaMigrationVersion = "2026-08-12-report-center-v5"
const schemaMigrationLockName = "data_gin_schema_migration_v1"

type schemaMigrationRecord struct {
	Version   string    `gorm:"column:version;primaryKey;size:64"`
	AppliedAt time.Time `gorm:"column:applied_at;not null"`
}

func (schemaMigrationRecord) TableName() string { return "app_schema_versions" }

// setupDB 初始化数据库和 ORM
func setupDB() {
	setupDBConnection()
	if autoMigrateOnStartup() {
		if err := ApplySchemaMigrations(); err != nil {
			console.Exit("database schema migration failed: %v", err)
		}
	}
}

func autoMigrateOnStartup() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("AUTO_MIGRATE_ON_STARTUP")))
	return value == "1" || value == "true" || value == "yes"
}

// ApplySchemaMigrations runs schema and access-control migrations explicitly.
// The HTTP server skips this expensive path by default so it can become ready quickly.
func ApplySchemaMigrations() (resultErr error) {
	db := database.DB
	if db == nil {
		return fmt.Errorf("database connection is unavailable")
	}
	if database.SQLDB == nil {
		return fmt.Errorf("database SQL connection is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.SQLDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	conn, err := database.SQLDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire schema migration connection: %w", err)
	}
	defer conn.Close()
	locked, err := acquireSchemaMigrationLock(ctx, conn)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("another schema migration is running")
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		if releaseErr := releaseSchemaMigrationLock(releaseCtx, conn); releaseErr != nil {
			resultErr = errors.Join(resultErr, releaseErr)
		}
	}()
	if err := db.AutoMigrate(&schemaMigrationRecord{}); err != nil {
		return fmt.Errorf("prepare schema migration marker: %w", err)
	}
	var count int64
	if err := db.Model(&schemaMigrationRecord{}).Where("version = ?", schemaMigrationVersion).Count(&count).Error; err != nil {
		return fmt.Errorf("read schema migration marker: %w", err)
	}
	if count == 1 {
		console.Info("database schema is current: %s", schemaMigrationVersion)
		return nil
	}
	if err := autoMigrateTables(); err != nil {
		return err
	}
	marker := schemaMigrationRecord{Version: schemaMigrationVersion, AppliedAt: time.Now().UTC()}
	if err := db.Create(&marker).Error; err != nil {
		return fmt.Errorf("write schema migration marker: %w", err)
	}
	return nil
}

func acquireSchemaMigrationLock(ctx context.Context, conn *sql.Conn) (bool, error) {
	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 60)", schemaMigrationLockName).Scan(&locked); err != nil {
		return false, fmt.Errorf("acquire schema migration lock: %w", err)
	}
	return locked.Valid && locked.Int64 == 1, nil
}

func releaseSchemaMigrationLock(ctx context.Context, conn *sql.Conn) error {
	var released sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", schemaMigrationLockName).Scan(&released); err != nil {
		return fmt.Errorf("release schema migration lock: %w", err)
	}
	if !released.Valid || released.Int64 != 1 {
		return fmt.Errorf("schema migration lock was not released")
	}
	return nil
}

func setupDBConnection() {

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
}

// autoMigrateTables 自动迁移数据存储相关表
func autoMigrateTables() error {
	console.Info("auto migrating data storage tables...")

	// 获取默认数据库连接
	db := database.DB

	// 检查数据库连接是否成功
	if db == nil {
		return fmt.Errorf("database connection is unavailable")
	}

	if err := prepareMethodPipelineIndexes(db); err != nil {
		return err
	}
	if err := prepareMallWeatherVersionIndexes(db); err != nil {
		logger.Error("修复商场天气版本索引失败", zap.Error(err))
		return fmt.Errorf("repair mall weather version indexes: %w", err)
	}
	if err := prepareReportVersionDatasourceSnapshot(db); err != nil {
		logger.Error("回填报表版本数据源快照失败", zap.Error(err))
		return fmt.Errorf("prepare report version datasource snapshot: %w", err)
	}

	// 迁移数据存储相关表
	models := []interface{}{
		&model.User{}, // 用户表
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.UserRole{},
		&model.UserMallScope{},
		&model.AuthAudit{},
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
	models = append(models, reportCenterMigrationModels()...)
	err := db.AutoMigrate(models...)

	if err != nil {
		logger.Error("数据表自动迁移失败", zap.Error(err))
		return fmt.Errorf("auto migrate tables: %w", err)
	}
	if err := verifyMallWeatherVersionIndexes(db); err != nil {
		logger.Error("校验商场天气版本索引失败", zap.Error(err))
		return fmt.Errorf("verify mall weather version indexes: %w", err)
	}

	console.Success("数据表自动迁移完成")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	synchronized, err := auth_svc.SyncExistingConsoleAdminPermissions(ctx, db)
	if err != nil {
		logger.Error("同步 admin 天气权限失败", zap.Error(err))
		return fmt.Errorf("sync admin permissions: %w", err)
	}
	if synchronized {
		console.Success("admin 天气权限同步完成")
	} else {
		console.Info("admin 尚未完成控制台身份初始化，首次控制台登录时将同步天气权限")
	}
	if err := auth_svc.SyncAccessControlSeeds(ctx, db); err != nil {
		logger.Error("同步账号权限种子失败", zap.Error(err))
		return fmt.Errorf("sync access control seeds: %w", err)
	}
	console.Success("账号权限种子同步完成")
	return nil
}

func reportCenterMigrationModels() []interface{} {
	return []interface{}{
		&model.ReportDatasource{},
		&model.ReportDefinition{},
		&model.ReportVersion{},
		&model.ReportParameter{},
		&model.ReportColumn{},
		&model.ReportGrant{},
		&model.ReportRun{},
		&model.ReportExport{},
		&model.ReportAudit{},
	}
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
		&model.OpenAPICredential{},
		&model.DataAuthorizationAudit{},
	}
}

func prepareMethodPipelineIndexes(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.MethodStep{}) {
		return nil
	}

	const indexName = "idx_pipeline_step_code"
	if !db.Migrator().HasIndex(&model.MethodStep{}, indexName) {
		return nil
	}

	if err := db.Migrator().DropIndex(&model.MethodStep{}, indexName); err != nil {
		logger.Error("删除旧方法步骤唯一索引失败", zap.String("index", indexName), zap.Error(err))
		return fmt.Errorf("drop legacy method step index %s: %w", indexName, err)
	}
	return nil
}
