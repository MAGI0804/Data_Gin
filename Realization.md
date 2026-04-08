# Storage 模块实现步骤

基于 `Storage.md` 架构设计文档和现有 `gin-biz-web-api` 项目架构，以下是 **Storage 模块具体实现步骤**。

## 📋 整体实施路线

| 阶段 | 主要内容 | 预计时间 | 输出物 |
|------|----------|----------|--------|
| **第一阶段：基础结构** | 数据库表创建、模型层、DAO层 | 1-2周 | 4张表、4个模型、4个DAO |
| **第二阶段：核心业务** | Service层、Controller层、Request验证 | 2-3周 | 4个Service、3个Controller、3个Request |
| **第三阶段：异步处理** | 定时任务、异步任务 | 1周 | 1个Crontab、1个Job |
| **第四阶段：集成配置** | 路由注册、配置扩展 | 1周 | 路由组、配置文件 |
| **第五阶段：测试文档** | 单元测试、API文档、迁移脚本 | 1周 | 测试用例、Swagger文档 |

---

## 🗃️ 步骤1：使用 AutoMigrate 自动同步数据库表

**原则：** 使用 GORM 的 AutoMigrate 功能自动创建和更新表结构，避免手动编写 SQL 脚本。

### 1.1 在应用启动时自动迁移

**位置：** 在数据库初始化后添加迁移代码，建议在 `bootstrap/database.go` 的 `setupDBMySQL()` 函数末尾添加：

```go
// bootstrap/database.go 在 setupDBMySQL() 函数末尾添加
func setupDBMySQL() {
    // ... 现有数据库初始化代码 ...

    // 数据库迁移 - 自动同步表结构
    autoMigrateTables()
}

// autoMigrateTables 自动迁移数据存储相关表
func autoMigrateTables() {
    console.Info("auto migrating data storage tables...")

    // 获取默认数据库连接
    db := database.DB

    // 迁移数据存储相关表
    err := db.AutoMigrate(
        &model.DataSource{},     // 数据源配置表
        &model.RawData{},        // 原始数据表
        &model.ProcessedData{},  // 处理结果表
        &model.Statistics{},     // 数据统计表
    )

    if err != nil {
        logger.Error("数据表自动迁移失败", zap.Error(err))
        console.Exit("数据表自动迁移失败: %v", err)
    }

    console.Success("数据表自动迁移完成")
}
```

### 1.2 创建独立的迁移命令（可选）

**位置：** `cmd/migrate.go`

```go
package cmd

import (
    "github.com/spf13/cobra"
    "gin-biz-web-api/bootstrap"
    "gin-biz-web-api/pkg/console"
    "gin-biz-web-api/pkg/database"
    "gin-biz-web-api/model"
)

var MigrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "运行数据库迁移",
}

var migrateDataTablesCmd = &cobra.Command{
    Use:     "data-tables",
    Short:   "创建/更新数据存储相关表",
    Example: "go run main.go migrate data-tables",
    Run: func(cmd *cobra.Command, args []string) {
        // 初始化配置和数据库连接
        bootstrap.SetupConfig()
        bootstrap.SetupDB()

        console.Info("开始迁移数据存储表...")

        db := database.DB
        err := db.AutoMigrate(
            &model.DataSource{},
            &model.RawData{},
            &model.ProcessedData{},
            &model.Statistics{},
        )

        console.ExitIf(err)
        console.Success("数据存储表迁移完成")
    },
}

func init() {
    MigrateCmd.AddCommand(migrateDataTablesCmd)
}
```

**注册命令：** 在 `cmd/root.go` 中添加 `MigrateCmd`

### 1.3 执行迁移

**方式一：应用启动时自动迁移**
```bash
# 正常启动应用时会自动执行迁移
go run main.go server
```

**方式二：手动执行迁移命令**
```bash
# 单独执行数据表迁移
go run main.go migrate data-tables
```

**方式三：开发环境快速迁移**
```go
// 在测试代码或 main 函数中直接调用
db.AutoMigrate(&model.DataSource{}, &model.RawData{})
```

### 1.4 迁移注意事项

1. **安全警告**：AutoMigrate 只创建新表和添加新字段，**不会删除字段或修改数据类型**
2. **生产环境**：建议先在测试环境执行，确认无误后再在生产环境执行
3. **数据备份**：执行迁移前建议备份重要数据
4. **版本控制**：对于复杂的表结构变更，建议使用数据库版本管理工具（如 golang-migrate）

### 1.5 模型定义要求

为了 AutoMigrate 正常工作，模型定义需要：

1. **明确字段类型**：使用正确的 GORM 标签
2. **JSON 字段**：使用 `gorm.io/datatypes.JSON`
3. **枚举字段**：使用 `gorm:"type:enum('value1','value2')"`
4. **字段长度**：指定字符串字段长度，如 `size:100`
5. **索引定义**：在需要查询的字段上添加索引

---

## 🏗️ 步骤2：实现模型层 (Model)

**位置：** `model/`

```go
// model/data_source_model.go
package model

import "gorm.io/datatypes"

type DataSource struct {
    *BaseModel
    Name           string         `gorm:"column:name;size:100;not null"`
    Type           string         `gorm:"column:type;size:50;not null"`
    Config         datatypes.JSON `gorm:"column:config;type:json;not null"`
    Schedule       string         `gorm:"column:schedule;size:100"`
    Status         string         `gorm:"column:status;type:enum('active','inactive','error');default:'active'"`
    LastSyncTime   *TimeNormal    `gorm:"column:last_sync_time"`
    LastSyncStatus string         `gorm:"column:last_sync_status;size:20"`
    *CommonTimestampsField
}

func (DataSource) TableName() string { return "data_sources" }

// model/raw_data_model.go、model/processed_data_model.go、model/statistics_model.go 类似
```

**关键点：**
- 继承 `BaseModel` 和 `CommonTimestampsField`
- JSON字段使用 `gorm.io/datatypes.JSON`
- 实现 `TableName()` 方法
- 参考 `user_model.go` 的字段定义风格

---

## 🗄️ 步骤3：实现数据访问层 (DAO)

**位置：** `internal/dao/data_dao/`

```go
// internal/dao/data_dao/data_source_dao.go
package data_dao

import (
    "context"
    "gin-biz-web-api/model"
    "gorm.io/gorm"
)

type DataSourceDAO struct {
    db *gorm.DB
}

func NewDataSourceDAO() *DataSourceDAO {
    return &DataSourceDAO{db: global.DB}
}

func (dao *DataSourceDAO) FindByID(ctx context.Context, id uint) (*model.DataSource, error) {
    var source model.DataSource
    err := dao.db.WithContext(ctx).Where("id = ?", id).First(&source).Error
    return &source, err
}

func (dao *DataSourceDAO) UpdateStatus(ctx context.Context, id uint, status, message string) error {
    return dao.db.WithContext(ctx).Model(&model.DataSource{}).
        Where("id = ?", id).
        Updates(map[string]interface{}{
            "status":              status,
            "last_sync_status":    message,
            "last_sync_time":      gorm.Expr("CURRENT_TIMESTAMP"),
            "updated_at":          time.Now().Unix(),
        }).Error
}

// 类似创建 raw_data_dao.go、processed_data_dao.go、statistics_dao.go
```

**参考模式：** 参考 `internal/dao/auth_dao/user_dao.go`

---

## 🔧 步骤4：实现服务层 (Service)

**位置：** `internal/service/data_svc/`

```go
// internal/service/data_svc/collect_service.go
package data_svc

import (
    "context"
    "gin-biz-web-api/internal/dao/data_dao"
    "gin-biz-web-api/pkg/job"
)

type CollectService struct {
    dataSourceDAO *data_dao.DataSourceDAO
    rawDataDAO    *data_dao.RawDataDAO
    jobClient     *job.Client
}

func NewCollectService() *CollectService {
    return &CollectService{
        dataSourceDAO: data_dao.NewDataSourceDAO(),
        rawDataDAO:    data_dao.NewRawDataDAO(),
        jobClient:     job.NewClient(),
    }
}

func (s *CollectService) CollectFromSource(ctx context.Context, sourceID uint) (string, error) {
    // 1. 获取数据源配置
    // 2. 根据类型调用采集器（API/数据库/文件）
    // 3. 保存原始数据
    // 4. 投递异步处理任务
    // 5. 更新数据源状态
    return "采集完成", nil
}
```

**服务划分：**
- `collect_service.go` - 数据采集（外部API、数据库）
- `ingest_service.go` - 数据接收（外部推送）
- `process_service.go` - 数据处理（清洗、转换）
- `query_service.go` - 数据查询（原始数据、处理数据）

---

## 🎮 步骤5：实现控制器层 (Controller)

**位置：** `internal/controller/data_ctrl/`

```go
// internal/controller/data_ctrl/ingest_controller.go
package data_ctrl

import (
    "github.com/gin-gonic/gin"
    "gin-biz-web-api/internal/requests/data_request"
    "gin-biz-web-api/internal/service/data_svc"
    "gin-biz-web-api/pkg/errcode"
    "gin-biz-web-api/pkg/responses"
)

type IngestController struct {
    service *data_svc.IngestService
}

func NewIngestController() *IngestController {
    return &IngestController{service: data_svc.NewIngestService()}
}

func (ctrl *IngestController) IngestData(c *gin.Context) {
    var req data_request.IngestRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        responses.New(c).ToErrorResponse(errcode.InvalidParams, err.Error())
        return
    }

    result, err := ctrl.service.IngestData(c.Request.Context(), &req)
    if err != nil {
        responses.New(c).ToErrorResponse(errcode.OperateFailed, err.Error())
        return
    }

    responses.New(c).ToResponse(gin.H{
        "request_id":    result.RequestID,
        "status":        "accepted",
        "accepted_count": result.AcceptedCount,
        "message":       "数据接收成功",
    })
}
```

**控制器划分：**
- `collect_controller.go` - `POST /api/v1/data/collect/{id}`, `GET /api/v1/data/collect/status/{job_id}`
- `ingest_controller.go` - `POST /api/v1/data/ingest`, `POST /api/v1/data/ingest/batch`
- `query_controller.go` - `GET /api/v1/data/raw`, `GET /api/v1/data/processed`

---

## 📝 步骤6：实现请求验证层 (Request)

**位置：** `internal/requests/data_request/`

```go
// internal/requests/data_request/ingest_request.go
package data_request

type IngestRequest struct {
    DataSource string                   `json:"data_source" binding:"required,max=100"`
    DataType   string                   `json:"data_type" binding:"required,max=50"`
    Data       []map[string]interface{} `json:"data" binding:"required,min=1,max=1000"`
}

func (r IngestRequest) Messages() map[string]string {
    return map[string]string{
        "DataSource.required": "数据源不能为空",
        "DataType.required":   "数据类型不能为空",
        "Data.required":       "数据内容不能为空",
    }
}
```

---

## ⏰ 步骤7：实现定时任务

**位置：** `crontab/data_collect_crontab.go`

```go
package crontab

import (
    "gin-biz-web-api/internal/service/data_svc"
    "gin-biz-web-api/global"
    "gin-biz-web-api/pkg/logger"
)

type DataCollectCrontab struct{}

func (d DataCollectCrontab) GetSpec() string {
    // 可从配置或数据库读取
    return "0 */30 * * * *" // 每30分钟执行
}

func (d DataCollectCrontab) Run() {
    logger.Info("开始执行数据采集定时任务")

    ctx := context.Background()
    service := data_svc.NewCollectService()

    // 查询所有活跃的数据源
    // 根据schedule判断是否需要执行
    // 调用service.CollectFromSource()
}
```

**注册到全局：** 在 `global/crontab.go` 的 `CronTasks` 中添加

---

## 🚀 步骤8：实现异步任务

**位置：** `job/data_process_job.go`

```go
package job

import (
    "context"
    "encoding/json"
    "github.com/hibiken/asynq"
    "gin-biz-web-api/internal/dao/data_dao"
    "gin-biz-web-api/internal/service/data_svc"
)

type DataProcessTask struct {
    RawDataID uint `json:"raw_data_id"`
}

func (t *DataProcessTask) Type() string { return "data:process" }

func (t *DataProcessTask) Process(ctx context.Context) error {
    // 1. 获取原始数据
    // 2. 更新状态为processing
    // 3. 调用process_service处理
    // 4. 保存处理结果
    // 5. 更新原始数据状态为processed
    return nil
}
```

**投递任务示例：**
```go
task := &DataProcessTask{RawDataID: 123}
jobClient.Enqueue(ctx, task)
```

---

## 🛣️ 步骤9：注册路由

**位置：** `bootstrap/router.go`

```go
// 在setupRouter函数中添加
dataCtrl := controller.NewDataController()

dataGroup := r.Group("/api/v1/data")
dataGroup.Use(middleware.AuthJWT()) // 需要认证
{
    dataGroup.POST("/collect/:source_id", dataCtrl.CollectController.ManualCollect)
    dataGroup.GET("/collect/status/:job_id", dataCtrl.CollectController.CollectStatus)

    dataGroup.POST("/ingest", dataCtrl.IngestController.IngestData)
    dataGroup.POST("/ingest/batch", dataCtrl.IngestController.IngestBatchData)

    dataGroup.GET("/raw", dataCtrl.QueryController.GetRawData)
    dataGroup.GET("/processed", dataCtrl.QueryController.GetProcessedData)
}
```

---

## ⚙️ 步骤10：扩展配置文件

**位置：** `etc/config.yaml`

```yaml
DataStorage:
  Collect:
    MaxRetries: 3
    Timeout: 30
    RateLimit: 10

  Ingest:
    MaxBatchSize: 1000
    MaxPayloadSize: 10485760  # 10MB
    EnableCompression: true

  Process:
    WorkerCount: 5
    QueueSize: 10000
    RetryDelay: 300

  Storage:
    RawDataRetentionDays: 90
    ProcessedDataRetentionDays: 365
    BackupEnabled: true
    BackupSchedule: "0 2 * * *"
```

**配置结构体：** 在 `config/` 下创建 `data_storage.go`

---

## 🧪 步骤11：编写单元测试

**测试重点：**
1. **DAO层**：使用测试数据库，测试CRUD操作
2. **Service层**：使用mock测试业务逻辑
3. **Controller层**：使用Gin测试工具测试HTTP接口

```go
// internal/service/data_svc/collect_service_test.go
func TestCollectService_CollectFromAPI(t *testing.T) {
    mockDAO := mock.NewMockDataSourceDAO()
    mockDAO.On("FindByID", mock.Anything, uint(1)).
        Return(&model.DataSource{Type: "api"}, nil)

    service := &CollectService{dataSourceDAO: mockDAO}
    result, err := service.CollectFromSource(context.Background(), 1)

    assert.NoError(t, err)
    assert.Contains(t, result, "采集")
}
```

---

## 📚 步骤12：编写API文档

**位置：** `docs/api/data_storage.md` 或集成 Swagger

```markdown
# 数据存储API文档

## 1. 手动触发数据采集
`POST /api/v1/data/collect/{source_id}`

**请求头：**
```
Authorization: Bearer {jwt_token}
```

**响应：**
```json
{
  "job_id": "uuid-1234",
  "status": "started",
  "message": "数据采集任务已启动"
}
```
```

---

## 🔄 实施顺序建议

### 第一周：基础结构
1. 创建数据库表
2. 编写模型层（4个模型）
3. 编写DAO层（4个DAO）

### 第二周：核心业务
1. 编写Service层（4个Service）
2. 编写Controller层（3个Controller）
3. 编写Request验证（3个Request）

### 第三周：异步处理
1. 编写定时任务
2. 编写异步任务
3. 集成到现有队列系统

### 第四周：集成配置
1. 注册路由
2. 扩展配置文件
3. 编写全局初始化代码

### 第五周：测试文档
1. 编写单元测试
2. 编写API文档
3. 创建迁移脚本

---

## ⚠️ 注意事项

1. **复用现有组件**：利用项目的认证、缓存、队列、日志
2. **错误处理**：统一使用 `pkg/errcode` 错误码
3. **日志规范**：关键操作使用结构化日志
4. **性能优化**：大数据量使用分页、批量插入
5. **安全考虑**：API认证、SQL注入防护、输入验证

---

完成以上步骤后，Storage模块即可提供完整的数据采集、接收、处理和查询功能，与现有项目架构无缝集成。