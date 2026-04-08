# 单服务器数据存储架构设计

## 概述

基于 `gin-biz-web-api` 项目和单服务器环境约束，设计一个轻量级、高效的数据存储架构，支持以下核心业务需求：

1. **从外部API获取数据**：定时或手动触发从第三方API采集数据
2. **接收外部数据**：提供标准API接口接收外部系统推送的数据

本架构充分利用现有项目基础设施（MySQL、Redis、异步队列、计划任务），在保证功能完整性的同时，简化部署和运维复杂度。

## 架构原则

### 设计约束
- **单服务器部署**：所有组件运行在单一物理/虚拟服务器上
- **资源有限**：合理利用CPU、内存、磁盘资源
- **快速部署**：基于现有项目，最小化额外组件依赖
- **易于维护**：清晰的代码结构和配置管理

### 技术选型
| 组件 | 技术方案 | 说明 |
|------|---------|------|
| Web框架 | Gin | 现有项目基础，高性能HTTP框架 |
| 数据库 | MySQL + GORM | 关系型数据存储，支持复杂查询 |
| 缓存/队列 | Redis + asynq | 内存缓存，异步任务队列 |
| 计划任务 | cron + 项目内置crontab | 定时数据采集 |
| 配置管理 | Viper + YAML | 多环境配置支持 |
| 日志系统 | Zap + Lumberjack | 结构化日志，日志轮转 |

## 系统架构

### 架构图
```
┌─────────────────────────────────────────────────────────┐
│                   应用层 (Application)                   │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  数据采集   │  │  数据接收   │  │  数据查询   │    │
│  │  Controller │  │  Controller │  │  Controller │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
├─────────────────────────────────────────────────────────┤
│                   服务层 (Service)                      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  采集服务   │  │  接收服务   │  │  处理服务   │    │
│  │   External  │  │   Ingest    │  │   Process   │    │
│  │    API      │  │   Service   │  │   Service   │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
├─────────────────────────────────────────────────────────┤
│                   数据访问层 (DAO)                      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  数据模型   │  │  数据访问   │  │  缓存管理   │    │
│  │    Model    │  │  对象 DAO   │  │   Cache     │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
├─────────────────────────────────────────────────────────┤
│                   存储层 (Storage)                      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   MySQL     │  │   Redis     │  │   文件系统   │    │
│  │  数据库     │  │  缓存/队列   │  │  文件存储   │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### 组件说明

#### 1. 数据采集模块 (External Data Collector)
- **功能**：从第三方API定时获取数据
- **触发方式**：定时任务(cron)、手动API调用、事件触发
- **特点**：支持重试机制、速率限制、错误处理

#### 2. 数据接收模块 (Data Ingest API)
- **功能**：提供标准RESTful API接收外部数据推送
- **协议**：HTTP/HTTPS，JSON格式
- **特性**：身份验证、数据验证、异步处理

#### 3. 数据处理模块 (Data Processing Service)
- **功能**：数据清洗、转换、验证、丰富
- **执行方式**：同步处理（简单转换）、异步队列（复杂处理）
- **技术**：使用asynq异步任务队列

#### 4. 数据存储模块 (Data Storage)
- **MySQL**：结构化数据存储，支持事务和复杂查询
- **Redis**：缓存热点数据，存储临时状态，作为消息队列
- **文件系统**：存储原始数据文件、日志文件、上传文件

#### 5. 数据查询模块 (Data Query API)
- **功能**：提供数据查询和统计接口
- **特性**：分页、过滤、排序、聚合查询

## 数据流程

### 场景1：从外部API获取数据
```
1. 定时任务触发 → 2. 调用外部API → 3. 数据验证 → 4. 存储原始数据
       ↓
5. 投递处理任务 → 6. 异步处理 → 7. 存储处理结果 → 8. 更新缓存
```

### 场景2：接收外部数据推送
```
1. API请求 → 2. 身份验证 → 3. 数据验证 → 4. 存储原始数据
       ↓
5. 返回接收确认 → 6. 异步处理（并行） → 7. 存储处理结果
```

## 数据库设计

### 核心数据表

#### 1. 数据源配置表 (data_sources)
存储外部API的配置信息，用于数据采集。
```sql
CREATE TABLE data_sources (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '数据源名称',
    type VARCHAR(50) NOT NULL COMMENT '数据类型: api/database/file',
    config JSON NOT NULL COMMENT '连接配置（API地址、认证信息等）',
    schedule VARCHAR(100) COMMENT '采集计划（cron表达式）',
    status ENUM('active', 'inactive', 'error') DEFAULT 'active',
    last_sync_time TIMESTAMP NULL COMMENT '最后同步时间',
    last_sync_status VARCHAR(20) COMMENT '最后同步状态',
    created_at INT UNSIGNED NOT NULL DEFAULT 0,
    updated_at INT UNSIGNED NOT NULL DEFAULT 0,
    INDEX idx_status (status),
    INDEX idx_last_sync (last_sync_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据源配置';
```

#### 2. 原始数据表 (raw_data)
存储从外部API获取或接收的原始数据。
```sql
CREATE TABLE raw_data (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    data_source_id BIGINT UNSIGNED NOT NULL COMMENT '数据源ID',
    external_id VARCHAR(255) COMMENT '外部系统ID',
    data_type VARCHAR(50) NOT NULL COMMENT '数据类型',
    raw_content JSON NOT NULL COMMENT '原始数据内容',
    metadata JSON COMMENT '元数据（来源、时间戳等）',
    status ENUM('pending', 'processing', 'processed', 'error') DEFAULT 'pending',
    error_message TEXT COMMENT '错误信息',
    created_at INT UNSIGNED NOT NULL DEFAULT 0,
    processed_at INT UNSIGNED DEFAULT 0 COMMENT '处理完成时间',
    INDEX idx_data_source (data_source_id),
    INDEX idx_status_created (status, created_at),
    INDEX idx_external_id (external_id),
    INDEX idx_data_type (data_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='原始数据存储';
```

#### 3. 处理结果表 (processed_data)
存储清洗和处理后的数据。
```sql
CREATE TABLE processed_data (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    raw_data_id BIGINT UNSIGNED NOT NULL COMMENT '原始数据ID',
    data_type VARCHAR(50) NOT NULL COMMENT '数据类型',
    data_fields JSON NOT NULL COMMENT '处理后的数据字段',
    quality_score DECIMAL(5,2) DEFAULT 100.00 COMMENT '数据质量评分',
    version INT UNSIGNED DEFAULT 1 COMMENT '数据版本',
    is_current BOOLEAN DEFAULT TRUE COMMENT '是否当前版本',
    created_at INT UNSIGNED NOT NULL DEFAULT 0,
    updated_at INT UNSIGNED NOT NULL DEFAULT 0,
    UNIQUE KEY uk_raw_data_version (raw_data_id, version),
    INDEX idx_data_type (data_type),
    INDEX idx_quality (quality_score),
    INDEX idx_current (is_current),
    FOREIGN KEY (raw_data_id) REFERENCES raw_data(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='处理后的数据';
```

#### 4. 数据统计表 (data_statistics)
存储数据统计信息，便于快速查询。
```sql
CREATE TABLE data_statistics (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    stat_date DATE NOT NULL COMMENT '统计日期',
    data_type VARCHAR(50) NOT NULL COMMENT '数据类型',
    data_source_id BIGINT UNSIGNED COMMENT '数据源ID',
    total_count INT UNSIGNED DEFAULT 0 COMMENT '总数据量',
    processed_count INT UNSIGNED DEFAULT 0 COMMENT '已处理数量',
    error_count INT UNSIGNED DEFAULT 0 COMMENT '错误数量',
    avg_quality_score DECIMAL(5,2) DEFAULT 0 COMMENT '平均质量分',
    created_at INT UNSIGNED NOT NULL DEFAULT 0,
    updated_at INT UNSIGNED NOT NULL DEFAULT 0,
    UNIQUE KEY uk_date_type_source (stat_date, data_type, data_source_id),
    INDEX idx_stat_date (stat_date),
    INDEX idx_data_type (data_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据统计';
```

## API接口设计

### 数据采集API

#### 1. 手动触发数据采集
```http
POST /api/v1/data/collect/{data_source_id}
Authorization: Bearer {token}

Response:
{
  "job_id": "uuid-1234",
  "status": "started",
  "message": "数据采集任务已启动"
}
```

#### 2. 查询采集状态
```http
GET /api/v1/data/collect/status/{job_id}
Authorization: Bearer {token}

Response:
{
  "job_id": "uuid-1234",
  "status": "completed",
  "progress": 100,
  "result": {
    "total": 150,
    "success": 148,
    "failed": 2
  }
}
```

### 数据接收API

#### 1. 接收数据推送
```http
POST /api/v1/data/ingest
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "data_source": "external_system",
  "data_type": "user_activity",
  "data": [
    {
      "id": "event-001",
      "timestamp": "2024-01-15T10:30:00Z",
      "user_id": "user-123",
      "action": "login",
      "details": {...}
    }
  ]
}

Response:
{
  "request_id": "req-5678",
  "status": "accepted",
  "accepted_count": 1,
  "message": "数据接收成功，已进入处理队列"
}
```

#### 2. 批量数据接收
```http
POST /api/v1/data/ingest/batch
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "batch_id": "batch-001",
  "items": [
    {
      "data_source": "system_a",
      "data_type": "metrics",
      "data": {...}
    },
    {
      "data_source": "system_b",
      "data_type": "logs",
      "data": {...}
    }
  ]
}
```

### 数据查询API

#### 1. 查询原始数据
```http
GET /api/v1/data/raw
Authorization: Bearer {token}
Query Parameters:
  - data_type: 数据类型
  - data_source_id: 数据源ID
  - status: 状态
  - start_time: 开始时间
  - end_time: 结束时间
  - page: 页码（默认1）
  - limit: 每页数量（默认20）

Response:
{
  "data": [
    {
      "id": 1,
      "data_source_id": 1,
      "data_type": "user_activity",
      "raw_content": {...},
      "status": "processed",
      "created_at": 1673856000
    }
  ],
  "meta": {
    "total": 150,
    "page": 1,
    "limit": 20,
    "pages": 8
  }
}
```

#### 2. 查询处理后的数据
```http
GET /api/v1/data/processed
Authorization: Bearer {token}
Query Parameters:
  - data_type: 数据类型
  - start_date: 开始日期
  - end_date: 结束日期
  - quality_min: 最低质量分

Response:
{
  "data": [
    {
      "id": 1,
      "data_type": "user_activity",
      "data_fields": {
        "user_id": "user-123",
        "action": "login",
        "timestamp": "2024-01-15T10:30:00Z",
        "device": "mobile"
      },
      "quality_score": 95.5,
      "created_at": 1673856000
    }
  ],
  "summary": {
    "total_count": 150,
    "avg_quality": 92.3
  }
}
```

## 代码结构扩展

### 新增目录结构
```
internal/
├── controller/
│   ├── data_ctrl/
│   │   ├── collect_controller.go      # 数据采集控制器
│   │   ├── ingest_controller.go       # 数据接收控制器
│   │   └── query_controller.go        # 数据查询控制器
│   └── ...
├── service/
│   ├── data_svc/
│   │   ├── collect_service.go         # 数据采集服务
│   │   ├── ingest_service.go          # 数据接收服务
│   │   ├── process_service.go         # 数据处理服务
│   │   └── query_service.go           # 数据查询服务
│   └── ...
├── dao/
│   ├── data_dao/
│   │   ├── data_source_dao.go         # 数据源DAO
│   │   ├── raw_data_dao.go            # 原始数据DAO
│   │   ├── processed_data_dao.go      # 处理数据DAO
│   │   └── statistics_dao.go          # 统计DAO
│   └── ...
└── ...
model/
├── data_source_model.go               # 数据源模型
├── raw_data_model.go                  # 原始数据模型
├── processed_data_model.go            # 处理数据模型
├── statistics_model.go                # 统计模型
└── ...
crontab/
├── data_collect_crontab.go            # 数据采集定时任务
└── ...
job/
├── data_process_job.go                # 数据处理异步任务
└── ...
```

### 核心代码示例

#### 1. 数据模型定义
```go
// model/data_source_model.go
package model

import "gorm.io/datatypes"

type DataSource struct {
    *BaseModel

    Name           string         `gorm:"column:name;size:100;not null" json:"name"`
    Type           string         `gorm:"column:type;size:50;not null" json:"type"`
    Config         datatypes.JSON `gorm:"column:config;type:json;not null" json:"config"`
    Schedule       string         `gorm:"column:schedule;size:100" json:"schedule"`
    Status         string         `gorm:"column:status;type:enum('active','inactive','error');default:'active'" json:"status"`
    LastSyncTime   *TimeNormal    `gorm:"column:last_sync_time" json:"last_sync_time"`
    LastSyncStatus string         `gorm:"column:last_sync_status;size:20" json:"last_sync_status"`

    *CommonTimestampsField
}

func (DataSource) TableName() string {
    return "data_sources"
}

// RawData 原始数据模型
type RawData struct {
    *BaseModel

    DataSourceID uint           `gorm:"column:data_source_id;not null" json:"data_source_id"`
    ExternalID   string         `gorm:"column:external_id;size:255" json:"external_id"`
    DataType     string         `gorm:"column:data_type;size:50;not null" json:"data_type"`
    RawContent   datatypes.JSON `gorm:"column:raw_content;type:json;not null" json:"raw_content"`
    Metadata     datatypes.JSON `gorm:"column:metadata;type:json" json:"metadata"`
    Status       string         `gorm:"column:status;type:enum('pending','processing','processed','error');default:'pending'" json:"status"`
    ErrorMessage string         `gorm:"column:error_message;type:text" json:"error_message"`
    ProcessedAt  int            `gorm:"column:processed_at;default:0" json:"processed_at"`

    *CommonTimestampsField
}

func (RawData) TableName() string {
    return "raw_data"
}
```

#### 2. 数据采集服务
```go
// internal/service/data_svc/collect_service.go
package data_svc

import (
    "context"
    "fmt"
    "time"

    "gin-biz-web-api/internal/dao/data_dao"
    "gin-biz-web-api/pkg/job"
    "gin-biz-web-api/pkg/logger"
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

// CollectFromSource 从指定数据源采集数据
func (s *CollectService) CollectFromSource(ctx context.Context, sourceID uint) (string, error) {
    // 1. 获取数据源配置
    source, err := s.dataSourceDAO.FindByID(ctx, sourceID)
    if err != nil {
        logger.Error("获取数据源失败", zap.Uint("source_id", sourceID), zap.Error(err))
        return "", fmt.Errorf("获取数据源失败: %w", err)
    }

    // 2. 根据类型调用不同的采集器
    var data []map[string]interface{}
    switch source.Type {
    case "api":
        data, err = s.collectFromAPI(ctx, source)
    case "database":
        data, err = s.collectFromDatabase(ctx, source)
    default:
        err = fmt.Errorf("不支持的数据源类型: %s", source.Type)
    }

    if err != nil {
        // 更新数据源状态为错误
        updateErr := s.dataSourceDAO.UpdateStatus(ctx, sourceID, "error", err.Error())
        if updateErr != nil {
            logger.Error("更新数据源状态失败", zap.Error(updateErr))
        }
        return "", fmt.Errorf("数据采集失败: %w", err)
    }

    // 3. 保存原始数据
    var rawDataIDs []uint
    for _, item := range data {
        rawData := &model.RawData{
            DataSourceID: sourceID,
            DataType:     source.Config.GetString("data_type"),
            RawContent:   item,
            Metadata: map[string]interface{}{
                "collected_at": time.Now().Unix(),
                "source":       source.Name,
            },
            Status: "pending",
        }

        id, err := s.rawDataDAO.Create(ctx, rawData)
        if err != nil {
            logger.Error("保存原始数据失败", zap.Error(err))
            continue
        }
        rawDataIDs = append(rawDataIDs, id)

        // 4. 投递异步处理任务
        s.jobClient.Enqueue(ctx, &job.DataProcessTask{
            RawDataID: id,
        })
    }

    // 5. 更新数据源状态
    s.dataSourceDAO.UpdateSyncStatus(ctx, sourceID, time.Now(), "success")

    return fmt.Sprintf("采集 %d 条数据，保存 %d 条数据", len(data), len(rawDataIDs)), nil
}

// collectFromAPI 从API采集数据
func (s *CollectService) collectFromAPI(ctx context.Context, source *model.DataSource) ([]map[string]interface{}, error) {
    // 实现API调用逻辑
    // 这里简化示例，实际需要根据配置调用HTTP API
    config := source.Config
    url := config.GetString("url")
    method := config.GetString("method", "GET")

    // 使用现有项目的HTTP客户端
    // ...

    return []map[string]interface{}{}, nil
}
```

#### 3. 数据接收控制器
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
    return &IngestController{
        service: data_svc.NewIngestService(),
    }
}

// IngestData 接收数据推送
func (ctrl *IngestController) IngestData(c *gin.Context) {
    // 1. 参数验证
    var req data_request.IngestRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        responses.New(c).ToErrorResponse(errcode.InvalidParams, err.Error())
        return
    }

    // 2. 调用服务处理
    result, err := ctrl.service.IngestData(c.Request.Context(), &req)
    if err != nil {
        responses.New(c).ToErrorResponse(errcode.OperateFailed, err.Error())
        return
    }

    // 3. 返回响应
    responses.New(c).ToResponse(gin.H{
        "request_id":    result.RequestID,
        "status":        "accepted",
        "accepted_count": result.AcceptedCount,
        "message":       "数据接收成功，已进入处理队列",
    })
}

// IngestBatchData 批量接收数据
func (ctrl *IngestController) IngestBatchData(c *gin.Context) {
    var req data_request.BatchIngestRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        responses.New(c).ToErrorResponse(errcode.InvalidParams, err.Error())
        return
    }

    result, err := ctrl.service.IngestBatchData(c.Request.Context(), &req)
    if err != nil {
        responses.New(c).ToErrorResponse(errcode.OperateFailed, err.Error())
        return
    }

    responses.New(c).ToResponse(gin.H{
        "batch_id":      result.BatchID,
        "status":        "accepted",
        "accepted_count": result.AcceptedCount,
        "failed_count":  result.FailedCount,
        "message":       "批量数据接收成功",
    })
}
```

#### 4. 数据处理异步任务
```go
// job/data_process_job.go
package job

import (
    "context"
    "encoding/json"

    "github.com/hibiken/asynq"

    "gin-biz-web-api/internal/dao/data_dao"
    "gin-biz-web-api/internal/service/data_svc"
    "gin-biz-web-api/pkg/logger"
)

type DataProcessTask struct {
    RawDataID uint
}

func (t *DataProcessTask) Type() string {
    return "data:process"
}

func (t *DataProcessTask) Payload() ([]byte, error) {
    return json.Marshal(t)
}

func (t *DataProcessTask) Process(ctx context.Context) error {
    logger.Info("开始处理数据", zap.Uint("raw_data_id", t.RawDataID))

    // 1. 获取原始数据
    rawDataDAO := data_dao.NewRawDataDAO()
    rawData, err := rawDataDAO.FindByID(ctx, t.RawDataID)
    if err != nil {
        logger.Error("获取原始数据失败", zap.Error(err))
        return err
    }

    // 2. 更新状态为处理中
    rawDataDAO.UpdateStatus(ctx, t.RawDataID, "processing", "")

    // 3. 调用处理服务
    processService := data_svc.NewProcessService()
    processedData, err := processService.ProcessRawData(ctx, rawData)
    if err != nil {
        // 更新状态为错误
        rawDataDAO.UpdateStatus(ctx, t.RawDataID, "error", err.Error())
        logger.Error("数据处理失败", zap.Error(err))
        return err
    }

    // 4. 保存处理结果
    processedDAO := data_dao.NewProcessedDataDAO()
    _, err = processedDAO.Create(ctx, processedData)
    if err != nil {
        logger.Error("保存处理结果失败", zap.Error(err))
        return err
    }

    // 5. 更新原始数据状态
    rawDataDAO.UpdateStatus(ctx, t.RawDataID, "processed", "")

    logger.Info("数据处理完成", zap.Uint("raw_data_id", t.RawDataID))
    return nil
}
```

## 配置扩展

### 新增配置文件项
```yaml
# etc/config.yaml 新增配置
DataStorage:
  # 数据采集配置
  Collect:
    MaxRetries: 3
    Timeout: 30
    RateLimit: 10  # 每秒请求限制

  # 数据接收配置
  Ingest:
    MaxBatchSize: 1000
    MaxPayloadSize: 10485760  # 10MB
    EnableCompression: true

  # 数据处理配置
  Process:
    WorkerCount: 5
    QueueSize: 10000
    RetryDelay: 300  # 重试延迟秒数

  # 数据存储配置
  Storage:
    RawDataRetentionDays: 90   # 原始数据保留90天
    ProcessedDataRetentionDays: 365  # 处理数据保留1年
    BackupEnabled: true
    BackupSchedule: "0 2 * * *"  # 每天凌晨2点备份
```

### 环境变量支持
```bash
# 数据采集相关
DATA_COLLECT_MAX_RETRIES=3
DATA_COLLECT_TIMEOUT=30
DATA_INGEST_MAX_BATCH_SIZE=1000

# Redis队列配置
REDIS_QUEUE_HOST=localhost
REDIS_QUEUE_PORT=6379
REDIS_QUEUE_DB=2
```

## 部署与运维

### 单服务器部署方案
```
单服务器（推荐配置）：
- CPU: 4核以上
- 内存: 8GB以上
- 磁盘: 100GB SSD（系统+数据）
- 网络: 100Mbps+

软件栈：
- MySQL 8.0+ (数据存储)
- Redis 6.0+ (缓存/队列)
- Go 1.19+ (应用运行)
- Nginx (反向代理，可选)
```

### 启动流程
1. **数据库初始化**
```sql
-- 执行数据库脚本
mysql -u root -p < scripts/init_database.sql

-- 或通过迁移工具
go run main.go migrate data_tables
```

2. **启动应用**
```bash
# 开发环境
go run main.go server

# 生产环境
./gin-biz-web-api server \
  --env=prod \
  --config_path=etc/ \
  --port=8080 \
  --mode=release
```

3. **启动异步任务处理器**
```bash
# 启动数据处理worker
./gin-biz-web-api job:work \
  --queues=data_process \
  --concurrency=5
```

### 监控与维护

#### 健康检查接口
```http
GET /api/v1/health
Response:
{
  "status": "healthy",
  "components": {
    "database": "connected",
    "redis": "connected",
    "queue": "active",
    "storage": "82%"
  },
  "metrics": {
    "total_data": 15000,
    "pending_tasks": 5,
    "processing_rate": "120/min"
  }
}
```

#### 关键指标监控
- **数据采集成功率**：`data_collect_success_rate`
- **数据接收延迟**：`data_ingest_latency_ms`
- **队列积压情况**：`queue_backlog_count`
- **存储使用率**：`storage_usage_percent`
- **数据处理吞吐量**：`data_process_throughput`

#### 日志监控
```bash
# 查看数据采集日志
tail -f storage/logs/data_collect.log

# 查看错误日志
grep "ERROR" storage/logs/app.log | tail -20

# 监控API访问日志
tail -f storage/logs/access.log | grep "/api/v1/data/"
```

## 性能优化建议

### 数据库优化
1. **索引策略**
   - 为查询频繁的字段创建索引
   - 使用复合索引覆盖常见查询模式
   - 定期分析索引使用情况，移除无用索引

2. **查询优化**
   - 使用分页查询避免大数据量返回
   - 合理使用连接查询和子查询
   - 定期执行 `ANALYZE TABLE` 更新统计信息

3. **分区策略**（数据量大时）
   - 按时间分区：`raw_data` 表按 `created_at` 分区
   - 按数据类型分区：`processed_data` 按 `data_type` 分区

### 缓存策略
1. **热点数据缓存**
   ```go
   // 使用Redis缓存查询结果
   func (s *QueryService) GetProcessedData(ctx context.Context, query *QueryParams) ([]*ProcessedData, error) {
       cacheKey := fmt.Sprintf("processed_data:%s", query.CacheKey())

       // 尝试从缓存获取
       cached, err := s.cache.Get(ctx, cacheKey)
       if err == nil {
           return cached, nil
       }

       // 缓存未命中，查询数据库
       data, err := s.processedDAO.Query(ctx, query)
       if err != nil {
           return nil, err
       }

       // 写入缓存，设置5分钟过期
       s.cache.Set(ctx, cacheKey, data, 5*time.Minute)

       return data, nil
   }
   ```

2. **缓存更新策略**
   - 写穿透：更新数据库时同步更新缓存
   - 延迟双删：更新数据后删除缓存，延迟再次删除
   - 缓存预热：系统启动时加载热点数据到缓存

### 异步处理优化
1. **批量处理**
   - 合并小任务为批量任务
   - 使用Redis List实现批量队列

2. **优先级队列**
   - 高优先级数据优先处理
   - 重要数据源优先采集

## 安全考虑

### 数据安全
1. **API认证**
   - JWT Token认证
   - API Key认证（用于系统间调用）
   - IP白名单限制

2. **数据加密**
   - 敏感字段加密存储
   - 传输层TLS加密
   - 配置文件加密

3. **访问控制**
   - 基于角色的访问控制（RBAC）
   - API接口权限控制
   - 数据操作审计日志

### 系统安全
1. **输入验证**
   - 所有API输入参数验证
   - SQL注入防护
   - XSS攻击防护

2. **资源限制**
   - API调用频率限制
   - 单次请求数据量限制
   - 并发连接数限制

## 扩展性设计

### 水平扩展准备
虽然当前为单服务器部署，但架构设计支持未来水平扩展：

1. **无状态API层**
   - API服务无状态，可部署多个实例
   - 使用负载均衡分发请求

2. **数据库读写分离**
   - 主库写，从库读
   - 使用中间件自动路由

3. **队列服务独立**
   - Redis队列服务可独立部署
   - 异步Worker可水平扩展

### 功能扩展点
1. **支持更多数据源类型**
   - 文件导入（CSV、Excel、JSON）
   - 数据库直连（MySQL、PostgreSQL）
   - 消息队列（Kafka、RabbitMQ）

2. **数据处理插件**
   - 可插拔的数据处理管道
   - 自定义数据转换规则
   - 数据质量检查插件

3. **存储后端扩展**
   - 支持对象存储（S3、MinIO）
   - 支持时序数据库（InfluxDB）
   - 支持搜索引擎（Elasticsearch）

## 故障处理与恢复

### 常见故障处理
1. **数据采集失败**
   - 自动重试机制（最多3次）
   - 失败任务人工干预接口
   - 失败原因分析与告警

2. **队列积压处理**
   - 监控队列长度告警
   - 动态增加Worker数量
   - 临时降低数据采集频率

3. **数据库连接失败**
   - 连接池健康检查
   - 自动重连机制
   - 降级处理（缓存数据）

### 数据恢复策略
1. **定期备份**
   ```bash
   # 数据库备份脚本
   mysqldump -u root -p gin_biz_web_api > backup/$(date +%Y%m%d).sql

   # Redis数据备份
   redis-cli BGSAVE
   ```

2. **灾难恢复**
   - 全量备份 + 增量备份
   - 备份文件异地存储
   - 定期恢复演练

## 总结

本架构设计针对单服务器环境，提供了完整的数据采集、接收、处理和查询能力。主要特点：

1. **轻量高效**：基于现有技术栈，无复杂外部依赖
2. **易于部署**：单服务器部署，配置简单
3. **功能完整**：支持数据全生命周期管理
4. **可扩展**：架构设计支持未来水平扩展
5. **易于维护**：清晰的代码结构和监控体系

该架构完全基于 `gin-biz-web-api` 项目现有能力构建，最大化复用现有组件，最小化开发工作量，可快速实施并投入生产使用。