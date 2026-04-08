# 数据仓库架构设计

## 概述

本数据仓库架构设计基于 `gin-biz-web-api` Web 应用程序，旨在构建一个可扩展、高性能的数据分析平台，用于收集、处理和分析应用程序产生的各类数据，支撑业务决策、用户行为分析和系统监控。

### 设计目标
- **数据集成**：整合多源数据（MySQL、Redis、日志文件等）
- **实时性**：支持近实时数据分析和离线批处理
- **可扩展性**：支持数据量和计算能力的水平扩展
- **易用性**：提供标准化的数据访问接口和可视化工具

## 数据源分析

### 1. 业务数据库（MySQL）
- **用户表（users）**：用户基本信息、注册时间、最后登录时间等
- **其他业务表**：根据后续业务扩展可能增加的表
- **数据特点**：结构化数据，数据量中等，更新频率较低

### 2. 缓存数据库（Redis）
- **会话数据**：用户登录状态、临时令牌
- **缓存数据**：热点数据、验证码、配置信息
- **队列数据**：异步任务队列
- **数据特点**：非持久化、高并发访问、数据结构多样

### 3. 应用日志
- **访问日志**：API 请求记录，包含 URL、HTTP 方法、状态码、响应时间、客户端 IP 等
- **应用日志**：业务操作日志、错误日志、调试信息
- **数据特点**：半结构化、数据量大、增长快速

### 4. 文件存储
- **上传文件**：用户头像、文档等
- **数据特点**：非结构化数据，存储成本较高

## 数据采集层

### 数据采集架构
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  数据源         │───▶│  采集组件       │───▶│  消息队列       │
│  - MySQL        │    │  - Canal        │    │  - Kafka        │
│  - Redis        │    │  - Logstash     │    │  - RocketMQ     │
│  - 日志文件      │    │  - Filebeat     │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 采集方案
1. **MySQL 数据同步**
   - **工具选择**：Alibaba Canal / Debezium
   - **同步模式**：Binlog 实时同步
   - **输出目标**：Kafka 消息队列

2. **Redis 数据采集**
   - **工具选择**：自定义采集脚本
   - **采集方式**：定期扫描关键 Key 模式
   - **输出目标**：Kafka / 直接写入 ODS 层

3. **日志文件采集**
   - **工具选择**：Filebeat + Logstash
   - **采集方式**：实时监控日志文件变化
   - **数据处理**：日志解析、字段提取、格式标准化

4. **API 请求日志**
   - **方案选择**：中间件直接写入 Kafka
   - **优势**：减少文件 I/O，提高实时性

## 数据存储层

### 分层架构（四层模型）
```
┌─────────────────────────────────────────────────────────┐
│                    应用数据层（ADS）                      │
│  - 面向业务的数据集市                                  │
│  - 高度聚合的指标数据                                  │
├─────────────────────────────────────────────────────────┤
│                    数据汇总层（DWS）                      │
│  - 轻度汇总的主题宽表                                  │
│  - 跨事实表的关联数据                                  │
├─────────────────────────────────────────────────────────┤
│                    数据明细层（DWD）                      │
│  - 清洗后的明细数据                                    │
│  - 维度退化后的数据                                    │
├─────────────────────────────────────────────────────────┤
│                    操作数据层（ODS）                      │
│  - 原始数据的镜像                                      │
│  - 保持与源系统一致                                    │
└─────────────────────────────────────────────────────────┘
```

### 各层详细设计

#### ODS 层（操作数据层）
- **存储介质**：HDFS / 对象存储
- **数据格式**：Parquet / ORC
- **数据保留**：30-90 天原始数据
- **同步频率**：
  - MySQL 数据：实时同步
  - 日志数据：实时同步
  - Redis 数据：小时级同步

#### DWD 层（数据明细层）
- **数据清洗**：
  - 去除无效数据
  - 标准化数据格式
  - 数据脱敏处理
- **维度退化**：将常用维度字段冗余到事实表中
- **分区策略**：按日期分区，便于数据管理

#### DWS 层（数据汇总层）
- **汇总维度**：
  - 按时间（小时、天、周、月）
  - 按用户维度
  - 按业务维度
- **预计算**：常用业务指标预先计算
- **存储策略**：多层聚合，平衡查询性能与存储成本

#### ADS 层（应用数据层）
- **数据形态**：高度聚合的指标数据
- **服务对象**：报表系统、BI 工具、API 接口
- **存储选型**：MySQL / ClickHouse / Elasticsearch（根据查询需求选择）

## 数据处理层

### 批处理流程
```yaml
每日批处理作业流程：
1. 00:00 - 数据质量检查（ODS 层）
2. 01:00 - DWD 层 ETL（清洗、转换、维度退化）
3. 02:00 - DWS 层聚合计算
4. 03:00 - ADS 层数据生成
5. 04:00 - 数据质量校验
6. 05:00 - 数据同步到查询引擎
```

### 流处理流程
```yaml
实时数据处理：
1. 数据采集 → Kafka
2. 实时清洗 → Flink 流处理
3. 实时聚合 → Flink 窗口计算
4. 结果存储 → ClickHouse / Redis
5. 实时监控 → 监控告警系统
```

### ETL/ELT 策略
- **ETL（传统模式）**：适用于复杂的数据转换场景
- **ELT（现代模式）**：利用数据仓库的计算能力，适用于大规模数据
- **混合模式**：根据数据特点选择合适的处理方式

## 数据服务层

### 数据访问接口
1. **SQL 查询服务**
   - 支持标准 SQL 查询
   - 多租户隔离
   - 查询性能优化

2. **RESTful API**
   - 指标数据接口
   - 用户行为分析接口
   - 系统监控接口

3. **数据导出服务**
   - 支持 CSV、Excel、JSON 格式
   - 定时导出任务
   - 大文件分片导出

### 数据可视化
1. **BI 工具集成**
   - Superset：开源 BI 工具，支持丰富的数据可视化
   - Metabase：用户友好的数据探索工具

2. **自定义报表**
   - 固定格式报表
   - 自助分析报表
   - 实时监控大屏

## 技术选型建议

### 基础设施层
| 组件类型 | 推荐技术 | 备选方案 | 说明 |
|---------|---------|---------|------|
| 数据存储 | HDFS | S3/MinIO | 大规模数据存储 |
| 数据仓库 | Apache Hive | Apache Iceberg | SQL 查询引擎 |
| 实时计算 | Apache Flink | Apache Spark Streaming | 流处理引擎 |
| 消息队列 | Apache Kafka | RocketMQ | 数据缓冲和解耦 |
| 任务调度 | Apache Airflow | DolphinScheduler | 工作流编排 |

### 查询引擎层
| 场景 | 推荐技术 | 特点 |
|------|---------|------|
| 交互式查询 | Apache Druid | 亚秒级查询延迟 |
| OLAP 分析 | ClickHouse | 高性能列式存储 |
| 全文搜索 | Elasticsearch | 复杂的搜索聚合 |
| 关系型查询 | MySQL/PostgreSQL | 事务性查询 |

### 监控运维
| 功能 | 工具 | 说明 |
|------|------|------|
| 任务监控 | Prometheus + Grafana | 任务执行状态监控 |
| 数据质量 | Great Expectations | 数据质量校验 |
| 元数据管理 | Apache Atlas | 数据血缘、数据治理 |
| 权限管理 | Apache Ranger | 数据访问权限控制 |

## 数据模型设计

### 核心事实表

#### 1. 用户行为事实表
```sql
CREATE TABLE dwd_user_action_fact (
    action_id STRING COMMENT '行为ID',
    user_id BIGINT COMMENT '用户ID',
    session_id STRING COMMENT '会话ID',
    action_type STRING COMMENT '行为类型: view/click/api_call',
    action_target STRING COMMENT '行为目标: page_url/api_endpoint',
    action_time TIMESTAMP COMMENT '行为时间',
    action_duration INT COMMENT '行为时长(ms)',
    device_info STRUCT<...> COMMENT '设备信息',
    geo_info STRUCT<...> COMMENT '地理位置信息',
    -- 退化维度字段
    user_age_group STRING COMMENT '用户年龄段',
    user_gender STRING COMMENT '用户性别',
    user_register_date DATE COMMENT '用户注册日期'
) PARTITIONED BY (dt STRING);
```

#### 2. API 访问事实表
```sql
CREATE TABLE dwd_api_access_fact (
    request_id STRING COMMENT '请求ID',
    user_id BIGINT COMMENT '用户ID',
    api_path STRING COMMENT 'API路径',
    http_method STRING COMMENT 'HTTP方法',
    status_code INT COMMENT '状态码',
    response_time INT COMMENT '响应时间(ms)',
    request_size INT COMMENT '请求大小',
    response_size INT COMMENT '响应大小',
    client_ip STRING COMMENT '客户端IP',
    user_agent STRING COMMENT '用户代理',
    request_time TIMESTAMP COMMENT '请求时间',
    -- 业务维度
    api_category STRING COMMENT 'API分类',
    api_version STRING COMMENT 'API版本'
) PARTITIONED BY (dt STRING);
```

### 核心维度表

#### 1. 用户维度表
```sql
CREATE TABLE dim_user (
    user_id BIGINT COMMENT '用户ID',
    account STRING COMMENT '账号',
    email STRING COMMENT '邮箱',
    phone STRING COMMENT '手机号',
    nickname STRING COMMENT '昵称',
    avatar_url STRING COMMENT '头像URL',
    introduction STRING COMMENT '个人简介',
    register_time TIMESTAMP COMMENT '注册时间',
    last_login_time TIMESTAMP COMMENT '最后登录时间',
    status STRING COMMENT '状态: active/inactive',
    -- 缓慢变化维度处理
    start_date DATE COMMENT '维度生效日期',
    end_date DATE COMMENT '维度失效日期',
    is_current BOOLEAN COMMENT '是否当前版本'
);
```

#### 2. 时间维度表
```sql
CREATE TABLE dim_date (
    date_key INT COMMENT '日期键: YYYYMMDD',
    full_date DATE COMMENT '完整日期',
    year INT COMMENT '年',
    quarter INT COMMENT '季度',
    month INT COMMENT '月',
    week INT COMMENT '周',
    day INT COMMENT '日',
    day_of_week INT COMMENT '星期几',
    day_of_year INT COMMENT '年中第几天',
    is_weekend BOOLEAN COMMENT '是否周末',
    is_holiday BOOLEAN COMMENT '是否节假日'
);
```

### 数据汇总表示例

#### 1. 用户活跃度汇总表
```sql
CREATE TABLE dws_user_activity_daily (
    date_key INT COMMENT '日期键',
    user_id BIGINT COMMENT '用户ID',
    login_count INT COMMENT '登录次数',
    api_call_count INT COMMENT 'API调用次数',
    total_duration BIGINT COMMENT '总在线时长(ms)',
    last_active_time TIMESTAMP COMMENT '最后活跃时间',
    -- 页面访问统计
    page_view_count INT COMMENT '页面访问次数',
    unique_page_count INT COMMENT '访问独立页面数',
    -- API 性能统计
    avg_response_time DECIMAL(10,2) COMMENT '平均响应时间',
    error_rate DECIMAL(5,4) COMMENT '错误率'
) PARTITIONED BY (dt STRING);
```

#### 2. API 性能监控汇总表
```sql
CREATE TABLE dws_api_performance_hourly (
    date_key INT COMMENT '日期键',
    hour_key INT COMMENT '小时键',
    api_path STRING COMMENT 'API路径',
    api_version STRING COMMENT 'API版本',
    request_count BIGINT COMMENT '请求次数',
    success_count BIGINT COMMENT '成功次数',
    error_count BIGINT COMMENT '错误次数',
    avg_response_time DECIMAL(10,2) COMMENT '平均响应时间',
    p95_response_time DECIMAL(10,2) COMMENT '95分位响应时间',
    p99_response_time DECIMAL(10,2) COMMENT '99分位响应时间',
    max_response_time INT COMMENT '最大响应时间',
    total_request_size BIGINT COMMENT '总请求大小',
    total_response_size BIGINT COMMENT '总响应大小'
) PARTITIONED BY (dt STRING);
```

## 运维与监控

### 数据质量管理
1. **完整性检查**：关键字段非空校验
2. **准确性检查**：数值范围、枚举值校验
3. **一致性检查**：跨表数据一致性校验
4. **及时性检查**：数据同步延迟监控

### 任务调度监控
1. **任务依赖管理**：DAG 依赖关系可视化
2. **任务重试机制**：失败任务自动重试
3. **报警通知**：企业微信、钉钉、邮件通知
4. **性能监控**：任务执行时间、资源使用监控

### 数据安全
1. **数据脱敏**：敏感信息加密存储
2. **访问控制**：基于角色的权限管理
3. **审计日志**：所有数据访问操作记录
4. **数据备份**：定期备份，灾难恢复演练

## 实施路线图

### 第一阶段：基础数据平台搭建（1-2个月）
1. 搭建 Hadoop 集群（HDFS + YARN）
2. 部署 Hive 数据仓库
3. 部署 Airflow 任务调度
4. 部署 Kafka 消息队列
5. 基础数据采集通道建设

### 第二阶段：核心数据模型建设（2-3个月）
1. ODS 层数据同步
2. DWD 层数据清洗和建模
3. DWS 层数据聚合
4. ADS 层数据服务接口
5. 基础报表开发

### 第三阶段：高级功能完善（3-4个月）
1. 实时数据处理（Flink）
2. 数据质量监控体系
3. 数据治理平台
4. 自助分析平台
5. AI/ML 平台集成

### 第四阶段：优化和扩展（持续）
1. 性能优化：查询优化、存储优化
2. 功能扩展：新增数据源、新增分析场景
3. 平台完善：监控体系、运维体系

## 详细实施步骤

### 第一阶段：基础数据平台搭建（第1-2个月）

#### 第1周：环境准备与规划
1. **硬件资源评估**
   - 评估数据量：估算每日增量数据（MySQL binlog 约 100MB/天，日志约 1GB/天）
   - 计算资源需求：8核32GB 服务器 × 3台（测试环境）
   - 存储需求：初始 1TB HDFS，预留 3倍扩容空间

2. **技术选型确认**
   - Hadoop 发行版：Apache Hadoop 3.3.4
   - 数据格式：Parquet（列式存储，高压缩比）
   - 资源调度：YARN Capacity Scheduler
   - 部署方式：Ansible 自动化部署

3. **网络与安全规划**
   - 网络隔离：数据平台部署在独立 VLAN
   - 防火墙规则：开放必要端口（HDFS: 9000, 9870; YARN: 8088; Hive: 10000）
   - 访问控制：Kerberos 认证集成

#### 第2-3周：Hadoop 集群部署
1. **基础环境配置**
   ```bash
   # 所有节点：修改主机名、hosts文件
   hostnamectl set-hostname dn01
   echo "192.168.1.101 nn01" >> /etc/hosts
   echo "192.168.1.102 dn01" >> /etc/hosts
   echo "192.168.1.103 dn02" >> /etc/hosts

   # 创建 hadoop 用户
   useradd -m -s /bin/bash hadoop
   echo "hadoop ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

   # 配置 SSH 免密登录
   su - hadoop
   ssh-keygen -t rsa
   ssh-copy-id hadoop@nn01
   ssh-copy-id hadoop@dn01
   ssh-copy-id hadoop@dn02
   ```

2. **Hadoop 安装配置**
   ```bash
   # 下载安装包
   wget https://archive.apache.org/dist/hadoop/common/hadoop-3.3.4/hadoop-3.3.4.tar.gz
   tar -xzf hadoop-3.3.4.tar.gz -C /opt/
   ln -s /opt/hadoop-3.3.4 /opt/hadoop

   # 配置环境变量
   echo 'export HADOOP_HOME=/opt/hadoop' >> ~/.bashrc
   echo 'export PATH=$PATH:$HADOOP_HOME/bin:$HADOOP_HOME/sbin' >> ~/.bashrc
   source ~/.bashrc

   # 核心配置文件
   # core-site.xml
   <property>
     <name>fs.defaultFS</name>
     <value>hdfs://nn01:9000</value>
   </property>

   # hdfs-site.xml
   <property>
     <name>dfs.replication</name>
     <value>2</value>
   </property>

   # yarn-site.xml
   <property>
     <name>yarn.resourcemanager.hostname</name>
     <value>nn01</value>
   </property>
   ```

3. **集群初始化与验证**
   ```bash
   # 格式化 HDFS
   hdfs namenode -format

   # 启动集群
   start-dfs.sh
   start-yarn.sh

   # 验证集群状态
   hdfs dfsadmin -report
   yarn node -list

   # 测试 HDFS 操作
   hdfs dfs -mkdir -p /user/hadoop
   hdfs dfs -put test.txt /user/hadoop/
   hdfs dfs -ls /user/hadoop
   ```

#### 第4周：Hive 数据仓库部署
1. **Hive 安装配置**
   ```bash
   # 下载安装包
   wget https://downloads.apache.org/hive/hive-3.1.3/apache-hive-3.1.3-bin.tar.gz
   tar -xzf apache-hive-3.1.3-bin.tar.gz -C /opt/
   ln -s /opt/apache-hive-3.1.3-bin /opt/hive

   # 配置环境变量
   echo 'export HIVE_HOME=/opt/hive' >> ~/.bashrc
   echo 'export PATH=$PATH:$HIVE_HOME/bin' >> ~/.bashrc
   source ~/.bashrc

   # 配置 MySQL 作为元数据存储
   mysql> CREATE DATABASE hive_metastore;
   mysql> CREATE USER 'hive'@'%' IDENTIFIED BY 'Hive@123456';
   mysql> GRANT ALL ON hive_metastore.* TO 'hive'@'%';

   # 配置 hive-site.xml
   <property>
     <name>javax.jdo.option.ConnectionURL</name>
     <value>jdbc:mysql://mysql-server:3306/hive_metastore</value>
   </property>
   ```

2. **初始化与测试**
   ```bash
   # 初始化元数据库
   schematool -initSchema -dbType mysql

   # 启动 Hive
   hive

   # 创建测试表
   CREATE TABLE test_table (
     id INT,
     name STRING,
     created_date DATE
   ) STORED AS PARQUET;

   # 加载测试数据
   LOAD DATA LOCAL INPATH '/tmp/test_data.csv' INTO TABLE test_table;

   # 查询验证
   SELECT * FROM test_table LIMIT 10;
   ```

#### 第5-6周：Kafka 消息队列部署
1. **Kafka 集群部署**
   ```bash
   # 下载安装包
   wget https://archive.apache.org/dist/kafka/3.4.0/kafka_2.13-3.4.0.tgz
   tar -xzf kafka_2.13-3.4.0.tgz -C /opt/
   ln -s /opt/kafka_2.13-3.4.0 /opt/kafka

   # 配置 server.properties
   broker.id=1
   listeners=PLAINTEXT://:9092
   advertised.listeners=PLAINTEXT://kafka01:9092
   log.dirs=/data/kafka-logs
   zookeeper.connect=zk01:2181,zk02:2181,zk03:2181

   # 启动 Kafka
   bin/kafka-server-start.sh config/server.properties &
   ```

2. **Topic 规划与创建**
   ```bash
   # 创建数据采集 Topic
   bin/kafka-topics.sh --create \
     --topic ods_mysql_binlog \
     --partitions 3 \
     --replication-factor 2 \
     --bootstrap-server localhost:9092

   bin/kafka-topics.sh --create \
     --topic ods_app_logs \
     --partitions 6 \
     --replication-factor 2 \
     --bootstrap-server localhost:9092

   # 查看 Topic 列表
   bin/kafka-topics.sh --list --bootstrap-server localhost:9092
   ```

#### 第7-8周：Airflow 任务调度部署
1. **Airflow 安装配置**
   ```bash
   # 使用 Docker Compose 部署
   wget https://airflow.apache.org/docs/apache-airflow/2.6.3/docker-compose.yaml

   # 修改配置
   AIRFLOW__CORE__EXECUTOR: CeleryExecutor
   AIRFLOW__CORE__LOAD_EXAMPLES: 'false'
   AIRFLOW__DATABASE__SQL_ALCHEMY_CONN: postgresql+psycopg2://airflow:airflow@postgres/airflow

   # 初始化数据库
   docker-compose up airflow-init

   # 启动服务
   docker-compose up -d
   ```

2. **基础 DAG 开发**
   ```python
   # dags/ods_sync_dag.py
   from airflow import DAG
   from airflow.operators.bash import BashOperator
   from datetime import datetime, timedelta

   default_args = {
       'owner': 'data_team',
       'depends_on_past': False,
       'start_date': datetime(2024, 1, 1),
       'retries': 3,
       'retry_delay': timedelta(minutes=5),
   }

   dag = DAG(
       'ods_mysql_sync',
       default_args=default_args,
       description='MySQL数据同步到ODS层',
       schedule_interval='0 * * * *',  # 每小时执行
   )

   extract_task = BashOperator(
       task_id='extract_mysql_data',
       bash_command='python /scripts/extract_mysql_to_hdfs.py',
       dag=dag,
   )
   ```

### 第二阶段：核心数据模型建设（第3-5个月）

#### 第1-2周：数据采集通道建设
1. **MySQL 数据同步（Canal）**
   ```bash
   # Canal 部署
   wget https://github.com/alibaba/canal/releases/download/canal-1.1.7/canal.deployer-1.1.7.tar.gz
   tar -xzf canal.deployer-1.1.7.tar.gz -C /opt/canal

   # 配置 canal.properties
   canal.destinations = example
   canal.instance.master.address = mysql-server:3306
   canal.instance.dbUsername = canal
   canal.instance.dbPassword = Canal@123456
   canal.instance.filter.regex = gin_biz_web_api\\..*

   # 启动 Canal
   ./bin/startup.sh

   # Canal Adapter 配置（写入 Kafka）
   curl -X POST http://canal-server:8081/destination/example/canalAdapters/rabbitmq \
     -H "Content-Type: application/json" \
     -d '{
       "mode": "kafka",
       "servers": "kafka01:9092,kafka02:9092",
       "topic": "ods_mysql_binlog"
     }'
   ```

2. **应用日志采集**
   ```yaml
   # filebeat.yml 配置
   filebeat.inputs:
   - type: log
     paths:
       - /var/log/gin-biz-web-api/*.log
     fields:
       source: app_log
     fields_under_root: true

   output.kafka:
     hosts: ["kafka01:9092", "kafka02:9092"]
     topic: "ods_app_logs"
     partition.round_robin:
       reachable_only: false
   ```

3. **API 请求日志中间件改造**
   ```go
   // 在现有中间件中增加 Kafka 写入功能
   // internal/middleware/access_log_middleware.go
   package middleware

   import (
       "github.com/gin-gonic/gin"
       "github.com/segmentio/kafka-go"
       "time"
   )

   var kafkaWriter *kafka.Writer

   func init() {
       kafkaWriter = &kafka.Writer{
           Addr:     kafka.TCP("kafka01:9092", "kafka02:9092"),
           Topic:    "ods_api_access",
           Balancer: &kafka.LeastBytes{},
       }
   }

   func AccessLog() gin.HandlerFunc {
       return func(c *gin.Context) {
           startTime := time.Now()

           c.Next()

           // 收集日志数据
           logData := map[string]interface{}{
               "request_id":   c.GetString("request_id"),
               "method":       c.Request.Method,
               "path":         c.Request.URL.Path,
               "status_code":  c.Writer.Status(),
               "response_time": time.Since(startTime).Milliseconds(),
               "client_ip":    c.ClientIP(),
               "user_agent":   c.Request.UserAgent(),
               "timestamp":    time.Now().Format(time.RFC3339),
           }

           // 写入 Kafka
           go kafkaWriter.WriteMessages(c.Request.Context(),
               kafka.Message{
                   Key:   []byte(logData["request_id"].(string)),
                   Value: json.Marshal(logData),
               })
       }
   }
   ```

#### 第3-4周：ODS 层数据同步
1. **Kafka 到 HDFS 同步（Flume/Flink）**
   ```java
   // Flink 流处理作业
   public class KafkaToHdfsJob {
       public static void main(String[] args) throws Exception {
           StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();

           // 读取 Kafka 数据
           DataStream<String> kafkaStream = env
               .addSource(new FlinkKafkaConsumer<>(
                   "ods_mysql_binlog",
                   new SimpleStringSchema(),
                   properties))
               .name("kafka-source");

           // 转换数据格式
           DataStream<RowData> parsedStream = kafkaStream
               .map(new JsonToRowDataMapper())
               .returns(RowData.class);

           // 写入 HDFS
           StreamingFileSink<RowData> sink = StreamingFileSink
               .forRowFormat(new Path("hdfs://nn01:9000/ods/mysql"),
                   ParquetRowDataBuilder.createSchema())
               .withBucketCheckInterval(1000)
               .build();

           parsedStream.addSink(sink);
           env.execute("Kafka to HDFS Sync");
       }
   }
   ```

2. **ODS 层 Hive 表创建**
   ```sql
   -- ODS 用户表
   CREATE EXTERNAL TABLE ods_users (
       id BIGINT COMMENT '用户ID',
       account STRING COMMENT '账号',
       email STRING COMMENT '邮箱',
       phone STRING COMMENT '手机号',
       nickname STRING COMMENT '昵称',
       password STRING COMMENT '密码',
       avatar STRING COMMENT '头像',
       introduction STRING COMMENT '简介',
       created_at INT COMMENT '创建时间',
       updated_at INT COMMENT '更新时间',
       op_type STRING COMMENT '操作类型: INSERT/UPDATE/DELETE',
       op_time TIMESTAMP COMMENT '操作时间'
   ) PARTITIONED BY (dt STRING)
   STORED AS PARQUET
   LOCATION 'hdfs://nn01:9000/ods/mysql/users';

   -- ODS API 访问日志表
   CREATE EXTERNAL TABLE ods_api_access (
       request_id STRING COMMENT '请求ID',
       user_id BIGINT COMMENT '用户ID',
       api_path STRING COMMENT 'API路径',
       http_method STRING COMMENT 'HTTP方法',
       status_code INT COMMENT '状态码',
       response_time INT COMMENT '响应时间(ms)',
       client_ip STRING COMMENT '客户端IP',
       user_agent STRING COMMENT '用户代理',
       request_time TIMESTAMP COMMENT '请求时间',
       dt STRING COMMENT '日期分区'
   ) PARTITIONED BY (hour STRING)
   STORED AS PARQUET
   LOCATION 'hdfs://nn01:9000/ods/logs/api_access';
   ```

#### 第5-6周：DWD 层数据清洗与建模
1. **数据清洗 SQL 脚本**
   ```sql
   -- DWD 用户维度表
   CREATE TABLE dwd_user_dim (
       user_id BIGINT COMMENT '用户ID',
       account STRING COMMENT '账号',
       email STRING COMMENT '邮箱',
       phone STRING COMMENT '手机号',
       nickname STRING COMMENT '昵称',
       avatar_url STRING COMMENT '头像URL',
       introduction STRING COMMENT '个人简介',
       register_date DATE COMMENT '注册日期',
       register_time TIMESTAMP COMMENT '注册时间',
       last_login_time TIMESTAMP COMMENT '最后登录时间',
       status STRING COMMENT '状态',
       age_group STRING COMMENT '年龄段',
       gender STRING COMMENT '性别',
       start_date DATE COMMENT '生效日期',
       end_date DATE COMMENT '失效日期',
       is_current BOOLEAN COMMENT '是否当前版本'
   ) PARTITIONED BY (dt STRING)
   STORED AS PARQUET;

   -- 数据清洗转换
   INSERT OVERWRITE TABLE dwd_user_dim PARTITION (dt='${execution_date}')
   SELECT
       id AS user_id,
       account,
       email,
       phone,
       nickname,
       avatar AS avatar_url,
       introduction,
       FROM_UNIXTIME(created_at) AS register_time,
       DATE(FROM_UNIXTIME(created_at)) AS register_date,
       NULL AS last_login_time,  -- 需要从登录日志中获取
       'active' AS status,
       CASE
           WHEN TIMESTAMPDIFF(YEAR, FROM_UNIXTIME(created_at), CURRENT_DATE) < 18 THEN 'under_18'
           WHEN TIMESTAMPDIFF(YEAR, FROM_UNIXTIME(created_at), CURRENT_DATE) BETWEEN 18 AND 25 THEN '18_25'
           WHEN TIMESTAMPDIFF(YEAR, FROM_UNIXTIME(created_at), CURRENT_DATE) BETWEEN 26 AND 35 THEN '26_35'
           WHEN TIMESTAMPDIFF(YEAR, FROM_UNIXTIME(created_at), CURRENT_DATE) BETWEEN 36 AND 45 THEN '36_45'
           ELSE 'over_45'
       END AS age_group,
       'unknown' AS gender,  -- 需要从用户资料中获取
       DATE(FROM_UNIXTIME(created_at)) AS start_date,
       '9999-12-31' AS end_date,
       TRUE AS is_current
   FROM ods_users
   WHERE dt = '${execution_date}'
     AND op_type IN ('INSERT', 'UPDATE')
     AND id IS NOT NULL
     AND account IS NOT NULL;
   ```

2. **DWD 用户行为事实表**
   ```sql
   CREATE TABLE dwd_user_action_fact (
       action_id STRING COMMENT '行为ID',
       user_id BIGINT COMMENT '用户ID',
       session_id STRING COMMENT '会话ID',
       action_type STRING COMMENT '行为类型',
       action_target STRING COMMENT '行为目标',
       action_time TIMESTAMP COMMENT '行为时间',
       action_duration INT COMMENT '行为时长(ms)',
       device_type STRING COMMENT '设备类型',
       browser STRING COMMENT '浏览器',
       os STRING COMMENT '操作系统',
       ip_address STRING COMMENT 'IP地址',
       country STRING COMMENT '国家',
       city STRING COMMENT '城市',
       -- 退化维度字段
       user_register_date DATE COMMENT '用户注册日期',
       user_age_group STRING COMMENT '用户年龄段',
       dt STRING COMMENT '日期分区'
   ) PARTITIONED BY (hour STRING)
   STORED AS PARQUET;

   -- 从API访问日志生成用户行为数据
   INSERT OVERWRITE TABLE dwd_user_action_fact PARTITION (dt='${execution_date}', hour='${hour}')
   SELECT
       request_id AS action_id,
       COALESCE(user_id, 0) AS user_id,
       MD5(CONCAT(client_ip, user_agent, DATE(request_time))) AS session_id,
       'api_call' AS action_type,
       api_path AS action_target,
       request_time AS action_time,
       response_time AS action_duration,
       CASE
           WHEN user_agent LIKE '%Mobile%' THEN 'mobile'
           ELSE 'desktop'
       END AS device_type,
       REGEXP_EXTRACT(user_agent, '([^/\\s]+)(?:/([^\\s]+))?', 1) AS browser,
       REGEXP_EXTRACT(user_agent, '\\((.*?)\\)', 1) AS os,
       client_ip AS ip_address,
       NULL AS country,  -- 需要IP库查询
       NULL AS city,     -- 需要IP库查询
       ud.register_date AS user_register_date,
       ud.age_group AS user_age_group
   FROM ods_api_access oaa
   LEFT JOIN dwd_user_dim ud ON oaa.user_id = ud.user_id AND ud.is_current = TRUE
   WHERE oaa.dt = '${execution_date}'
     AND oaa.hour = '${hour}'
     AND oaa.request_time IS NOT NULL;
   ```

#### 第7-10周：DWS 层数据聚合与 ADS 层接口开发
1. **DWS 用户活跃度聚合**
   ```sql
   CREATE TABLE dws_user_activity_daily (
       date_key INT COMMENT '日期键',
       user_id BIGINT COMMENT '用户ID',
       login_count INT COMMENT '登录次数',
       api_call_count INT COMMENT 'API调用次数',
       total_duration BIGINT COMMENT '总在线时长(ms)',
       avg_response_time DECIMAL(10,2) COMMENT '平均响应时间',
       last_active_time TIMESTAMP COMMENT '最后活跃时间',
       active_days_count INT COMMENT '活跃天数',
       consecutive_active_days INT COMMENT '连续活跃天数',
       dt STRING COMMENT '日期分区'
   ) PARTITIONED BY (dt STRING)
   STORED AS PARQUET;

   -- 每日聚合计算
   INSERT OVERWRITE TABLE dws_user_activity_daily PARTITION (dt='${execution_date}')
   SELECT
       CAST(DATE_FORMAT(action_time, 'yyyyMMdd') AS INT) AS date_key,
       user_id,
       COUNT(DISTINCT CASE WHEN action_type = 'login' THEN session_id END) AS login_count,
       COUNT(CASE WHEN action_type = 'api_call' THEN 1 END) AS api_call_count,
       SUM(action_duration) AS total_duration,
       AVG(action_duration) AS avg_response_time,
       MAX(action_time) AS last_active_time,
       COUNT(DISTINCT DATE(action_time)) AS active_days_count,
       -- 连续活跃天数计算（需要窗口函数）
       0 AS consecutive_active_days
   FROM dwd_user_action_fact
   WHERE dt = '${execution_date}'
   GROUP BY user_id, CAST(DATE_FORMAT(action_time, 'yyyyMMdd') AS INT);
   ```

2. **ADS 层数据服务 API**
   ```go
   // internal/controller/dw_ctrl/data_warehouse_controller.go
   package dw_ctrl

   import (
       "github.com/gin-gonic/gin"
       "gin-biz-web-api/internal/service/dw_svc"
       "gin-biz-web-api/pkg/responses"
   )

   type DataWarehouseController struct {
       service *dw_svc.DataWarehouseService
   }

   func NewDataWarehouseController() *DataWarehouseController {
       return &DataWarehouseController{
           service: dw_svc.NewDataWarehouseService(),
       }
   }

   // GetUserActivity 获取用户活跃度数据
   func (ctrl *DataWarehouseController) GetUserActivity(c *gin.Context) {
       startDate := c.Query("start_date")
       endDate := c.Query("end_date")
       userId := c.Query("user_id")

       data, err := ctrl.service.GetUserActivity(startDate, endDate, userId)
       if err != nil {
           responses.New(c).ToErrorResponse(err)
           return
       }

       responses.New(c).ToResponse(gin.H{
           "data": data,
           "meta": gin.H{
               "total": len(data),
               "page":  1,
               "limit": 100,
           },
       })
   }

   // GetApiPerformance 获取API性能数据
   func (ctrl *DataWarehouseController) GetApiPerformance(c *gin.Context) {
       apiPath := c.Query("api_path")
       dateRange := c.Query("date_range")

       data, err := ctrl.service.GetApiPerformance(apiPath, dateRange)
       if err != nil {
           responses.New(c).ToErrorResponse(err)
           return
       }

       responses.New(c).ToResponse(gin.H{
           "metrics": data,
       })
   }
   ```

### 第三阶段：高级功能完善（第6-10个月）

#### 实时数据处理（Flink）
1. **Flink 实时计算作业部署**
   ```java
   // 实时用户活跃度计算
   public class RealtimeUserActivityJob {
       public static void main(String[] args) throws Exception {
           StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();
           env.setStreamTimeCharacteristic(TimeCharacteristic.EventTime);

           // 读取 Kafka 实时数据
           DataStream<UserAction> actions = env
               .addSource(new FlinkKafkaConsumer<>(
                   "ods_user_actions",
                   new JSONDeserializationSchema(),
                   properties))
               .assignTimestampsAndWatermarks(WatermarkStrategy
                   .<UserAction>forBoundedOutOfOrderness(Duration.ofSeconds(5))
                   .withTimestampAssigner((event, timestamp) -> event.getTimestamp()))
               .name("kafka-user-actions");

           // 实时窗口聚合
           DataStream<UserActivity> activityStream = actions
               .keyBy(UserAction::getUserId)
               .window(TumblingEventTimeWindows.of(Time.minutes(5)))
               .aggregate(new UserActivityAggregator())
               .name("user-activity-aggregation");

           // 写入 ClickHouse
           activityStream.addSink(new ClickHouseSink())
               .name("clickhouse-sink");

           env.execute("Realtime User Activity Analysis");
       }
   }
   ```

2. **ClickHouse 实时数据存储**
   ```sql
   -- 创建 ClickHouse 表
   CREATE TABLE dws_realtime_user_activity
   (
       user_id UInt64,
       date Date,
       hour DateTime,
       login_count UInt32,
       api_call_count UInt32,
       total_duration UInt64,
       avg_response_time Float32,
       last_active_time DateTime
   )
   ENGINE = MergeTree()
   PARTITION BY toYYYYMM(date)
   ORDER BY (user_id, date, hour)
   SETTINGS index_granularity = 8192;
   ```

#### 数据质量监控体系
1. **数据质量检查规则定义**
   ```python
   # data_quality_rules.py
   from great_expectations.core import ExpectationSuite, ExpectationConfiguration

   suite = ExpectationSuite(
       expectation_suite_name="ods_users_quality_suite"
   )

   # 完整性检查
   suite.add_expectation(
       ExpectationConfiguration(
           expectation_type="expect_column_values_to_not_be_null",
           kwargs={
               "column": "id",
               "mostly": 1.0
           }
       )
   )

   # 准确性检查
   suite.add_expectation(
       ExpectationConfiguration(
           expectation_type="expect_column_values_to_be_between",
           kwargs={
               "column": "created_at",
               "min_value": 1609459200,  # 2021-01-01
               "max_value": 1735689600   # 2024-12-31
           }
       )
   )

   # 一致性检查
   suite.add_expectation(
       ExpectationConfiguration(
           expectation_type="expect_column_pair_values_to_be_equal",
           kwargs={
               "column_A": "account",
               "column_B": "email",
               "ignore_row_if": "either_value_is_missing"
           }
       )
   )
   ```

2. **数据质量监控 Airflow DAG**
   ```python
   # dags/data_quality_check.py
   from airflow import DAG
   from airflow.operators.python import PythonOperator
   from great_expectations.checkpoint import SimpleCheckpoint

   def run_data_quality_check(**context):
       checkpoint_name = "ods_users_checkpoint"
       checkpoint = SimpleCheckpoint(
           f"{checkpoint_name}",
           data_context,
           checkpoint_config={
               "class_name": "SimpleCheckpoint",
               "validations": [
                   {
                       "batch_request": {
                           "datasource_name": "hdfs_datasource",
                           "data_connector_name": "default_inferred_data_connector_name",
                           "data_asset_name": "ods_users",
                           "data_connector_query": {
                               "partition_index": -1
                           }
                       },
                       "expectation_suite_name": "ods_users_quality_suite"
                   }
               ]
           }
       )

       results = checkpoint.run()
       return results.to_json_dict()

   # 定义 DAG
   dag = DAG(
       'data_quality_check',
       schedule_interval='0 2 * * *',  # 每天凌晨2点执行
       default_args=default_args
   )

   quality_task = PythonOperator(
       task_id='check_ods_users_quality',
       python_callable=run_data_quality_check,
       dag=dag
   )
   ```

### 第四阶段：优化和扩展（持续）

#### 性能优化策略
1. **查询优化**
   ```sql
   -- 1. 分区优化
   ALTER TABLE dwd_user_action_fact DROP PARTITION dt < '2024-01-01';

   -- 2. 索引优化
   CREATE INDEX idx_user_id ON dwd_user_action_fact(user_id) AS 'BITMAP';
   CREATE INDEX idx_action_time ON dwd_user_action_fact(action_time) AS 'BLOOMFILTER';

   -- 3. 数据压缩优化
   SET hive.exec.compress.output=true;
   SET mapred.output.compression.codec=org.apache.hadoop.io.compress.SnappyCodec;
   SET hive.exec.orc.compression.strategy=COMPRESSION;

   -- 4. 小文件合并
   SET hive.merge.mapfiles=true;
   SET hive.merge.mapredfiles=true;
   SET hive.merge.size.per.task=256000000;
   SET hive.merge.smallfiles.avgsize=128000000;
   ```

2. **存储优化**
   ```bash
   # 冷热数据分层存储
   # 热数据：SSD存储，近30天数据
   # 温数据：HDD存储，30-90天数据
   # 冷数据：对象存储归档，90天以上数据

   # 配置 HDFS 存储策略
   hdfs storagepolicies -setStoragePolicy -path /ods/mysql -policy HOT
   hdfs storagepolicies -setStoragePolicy -path /dwd -policy WARM
   hdfs storagepolicies -setStoragePolicy -path /archive -policy COLD
   ```

#### 监控告警体系
1. **Prometheus 监控配置**
   ```yaml
   # prometheus.yml
   scrape_configs:
     - job_name: 'hadoop'
       static_configs:
         - targets: ['nn01:9870', 'rm01:8088']

     - job_name: 'hive'
       static_configs:
         - targets: ['hiveserver2:10000']

     - job_name: 'airflow'
       static_configs:
         - targets: ['airflow-webserver:8080']

     - job_name: 'flink'
       static_configs:
         - targets: ['flink-jobmanager:8081']
   ```

2. **Grafana 监控面板**
   ```json
   // 数据同步延迟监控面板
   {
     "panels": [
       {
         "title": "MySQL同步延迟",
         "targets": [
           {
             "expr": "canal_instance_delay{instance=\"mysql\"}",
             "legendFormat": "{{instance}}"
           }
         ],
         "alert": {
           "conditions": [
             {
               "evaluator": {
                 "params": [300],
                 "type": "gt"
               },
               "operator": {
                 "type": "and"
               },
               "query": {
                 "params": ["A", "5m", "now"]
               },
               "reducer": {
                 "params": [],
                 "type": "avg"
               },
               "type": "query"
             }
           ]
         }
       }
     ]
   }
   ```

#### 运维自动化
1. **Ansible 自动化部署脚本**
   ```yaml
   # ansible/playbooks/deploy_hadoop.yml
   - hosts: hadoop_cluster
     become: yes
     vars:
       hadoop_version: "3.3.4"
       java_home: "/usr/lib/jvm/java-11-openjdk"

     tasks:
     - name: 安装 Java
       yum:
         name: java-11-openjdk-devel
         state: present

     - name: 创建 Hadoop 用户
       user:
         name: hadoop
         group: hadoop
         shell: /bin/bash

     - name: 下载 Hadoop
       get_url:
         url: "https://archive.apache.org/dist/hadoop/common/hadoop-{{ hadoop_version }}/hadoop-{{ hadoop_version }}.tar.gz"
         dest: "/tmp/hadoop-{{ hadoop_version }}.tar.gz"

     - name: 解压 Hadoop
       unarchive:
         src: "/tmp/hadoop-{{ hadoop_version }}.tar.gz"
         dest: "/opt"
         remote_src: yes

     - name: 配置环境变量
       template:
         src: templates/hadoop_env.j2
         dest: /etc/profile.d/hadoop.sh
   ```

### 关键成功因素

1. **团队技能培养**
   - Hadoop/Spark 开发技能培训（2周）
   - 数据建模方法论培训（1周）
   - SQL 优化技巧培训（1周）

2. **项目管理**
   - 每周进度评审会议
   - 每月业务价值汇报
   - 每季度架构评审

3. **质量控制**
   - 代码审查：所有 ETL 脚本必须经过审查
   - 测试覆盖：单元测试覆盖率达到80%
   - 文档完备：所有组件必须有详细文档

4. **风险管理**
   - 数据丢失风险：建立多级备份策略
   - 性能瓶颈风险：定期性能压测
   - 技术债务风险：技术债务跟踪与偿还计划

## 总结

本数据仓库架构设计为 `gin-biz-web-api` 应用程序提供了一个完整的数据分析解决方案，涵盖了从数据采集到数据服务的全链路流程。架构设计充分考虑了系统的可扩展性、实时性和易用性，为业务决策和系统优化提供了坚实的数据基础。

随着业务的发展，架构可以逐步演进，引入更多先进的技术和工具，满足不断增长的数据分析需求。