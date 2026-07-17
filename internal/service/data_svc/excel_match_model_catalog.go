package data_svc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"

	"gorm.io/gorm/schema"
)

// ExcelMatchModel is a selectable application model and its database mapping.
type ExcelMatchModel struct {
	Name        string                 `json:"name"`
	ModelName   string                 `json:"modelName"`
	TableName   string                 `json:"tableName"`
	Description string                 `json:"description"`
	Mapping     string                 `json:"mapping"`
	Fields      []ExcelMatchModelField `json:"fields"`
}

// ExcelMatchModelField is a selectable model field and its database mapping.
type ExcelMatchModelField struct {
	Name        string `json:"name"`
	ModelField  string `json:"modelField"`
	ColumnName  string `json:"columnName"`
	DataType    string `json:"dataType"`
	Description string `json:"description"`
	Mapping     string `json:"mapping"`
	Nullable    bool   `json:"nullable"`
	position    int
}

type excelMatchRegisteredModel struct {
	value       any
	name        string
	description string
}

type excelMatchModelDefinition struct {
	modelName   string
	name        string
	description string
	fields      map[string]excelMatchRegisteredField
}

type excelMatchRegisteredField struct {
	modelField string
	comment    string
}

type excelMatchFieldLabel struct {
	name        string
	description string
}

var excelMatchRegisteredModels = []excelMatchRegisteredModel{
	{value: &model.BojunRetailOrder{}, name: "伯俊零售订单", description: "伯俊零售单头数据与订单匹配结果"},
	{value: &model.QIMAI_ORDER_DATA{}, name: "企迈订单", description: "企迈渠道订单数据"},
	{value: &model.YOUZAN_ORDER_DATA{}, name: "有赞订单", description: "有赞交易订单数据"},
	{value: &model.YOUZAN_RETURN_DATA{}, name: "有赞退款订单", description: "有赞退款订单数据"},
	{value: &model.YouzanDistributionOrder{}, name: "有赞分销订单", description: "有赞分销账号拉取的订单数据"},
	{value: &model.DataSource{}, name: "旧版数据源", description: "旧版外部数据源配置"},
	{value: &model.RawData{}, name: "旧版原始数据", description: "旧版外部接口原始数据"},
	{value: &model.ProcessedData{}, name: "处理结果", description: "清洗和处理后的数据"},
	{value: &model.DataStatistics{}, name: "数据统计", description: "按日期与数据源汇总的统计数据"},
	{value: &model.SourceDefinition{}, name: "数据源定义", description: "数据仓库通用数据源配置"},
	{value: &model.RawRecord{}, name: "原始记录", description: "数据仓库通用原始记录"},
	{value: &model.CleanTableDefinition{}, name: "清洗表定义", description: "清洗结果表的字段与索引配置"},
	{value: &model.CleanRecord{}, name: "清洗记录", description: "数据仓库通用清洗结果"},
	{value: &model.TransformRule{}, name: "转换规则", description: "数据清洗与转换规则"},
	{value: &model.DestinationDefinition{}, name: "推送目标定义", description: "数据推送目标配置"},
	{value: &model.DeliveryTask{}, name: "推送任务", description: "数据推送任务配置"},
	{value: &model.PipelineRun{}, name: "流水线运行", description: "拉取、清洗与推送运行记录"},
	{value: &model.DeliveryLog{}, name: "推送日志", description: "单条数据推送日志"},
	{value: &model.RuntimeConfig{}, name: "运行时配置", description: "可在线编辑的运行时配置"},
	{value: &model.ExcelMatchJob{}, name: "Excel 匹配任务", description: "Excel 匹配与导入任务记录"},
	{value: &model.ExcelMatchJobLog{}, name: "Excel 匹配日志", description: "Excel 任务执行日志"},
	{value: &model.ExcelMatchScheme{}, name: "Excel 匹配方案", description: "可复用的 Excel 配置方案"},
	{value: &model.PipelineDefinition{}, name: "方法流水线", description: "可配置的业务处理流水线"},
	{value: &model.PipelineStage{}, name: "流水线阶段", description: "流水线中的业务阶段"},
	{value: &model.MethodStep{}, name: "方法步骤", description: "流水线阶段中的可执行方法"},
	{value: &model.MethodParam{}, name: "方法参数", description: "方法步骤的输入参数"},
	{value: &model.MethodOutput{}, name: "方法输出", description: "方法步骤产出的字段"},
	{value: &model.StageGeneratedConfig{}, name: "阶段生成配置", description: "阶段拼接生成的运行配置"},
	{value: &model.StepRun{}, name: "步骤运行", description: "单个方法步骤的执行结果"},
	{value: &model.User{}, name: "用户", description: "后台用户账号"},
	{value: &model.TokenData{}, name: "平台令牌", description: "外部平台令牌信息"},
}

var excelMatchFieldLabels = map[string]excelMatchFieldLabel{
	"bojun_retail_orders.docno":            {name: "伯俊零售单号", description: "伯俊系统生成的零售单号"},
	"bojun_retail_orders.otherdocno":       {name: "外部订单号", description: "伯俊订单关联的外部平台订单号"},
	"bojun_retail_orders.matched_docno":    {name: "已匹配订单号", description: "回填或关联得到的匹配单号"},
	"bojun_retail_orders.c_store_code":     {name: "伯俊门店编码", description: "伯俊订单所属门店编码"},
	"bojun_retail_orders.c_store_name":     {name: "伯俊门店名称", description: "伯俊订单所属门店名称"},
	"bojun_retail_orders.o2o_so_docno":     {name: "O2O 订单号", description: "伯俊订单关联的 O2O 销售订单号"},
	"qimai_order_data.order_no":            {name: "业务订单号", description: "企迈业务订单号"},
	"qimai_order_data.store_order_no":      {name: "取餐号", description: "企迈门店侧取餐号"},
	"qimai_order_data.shop_code":           {name: "企迈门店编码", description: "企迈订单所属门店编码"},
	"qimai_order_data.shop_name":           {name: "企迈门店名称", description: "企迈订单所属门店名称"},
	"youzan_order_data.tid":                {name: "有赞订单号", description: "有赞交易订单编号"},
	"youzan_order_data.shop_name":          {name: "有赞店铺名称", description: "有赞订单所属店铺名称"},
	"youzan_distribution_orders.tid":       {name: "有赞分销订单号", description: "有赞分销交易订单编号"},
	"youzan_distribution_orders.shop_name": {name: "有赞分销店铺名称", description: "有赞分销订单所属店铺名称"},
}

var excelMatchCommonFieldLabels = map[string]excelMatchFieldLabel{
	"id":            {name: "主键 ID", description: "数据库记录主键"},
	"created_at":    {name: "创建时间", description: "数据库记录创建时间"},
	"updated_at":    {name: "更新时间", description: "数据库记录最后更新时间"},
	"deleted_at":    {name: "删除时间", description: "数据库记录软删除时间"},
	"raw_data_id":   {name: "原始数据 ID", description: "关联的原始数据记录 ID"},
	"status":        {name: "状态", description: "当前记录状态"},
	"synced":        {name: "同步状态", description: "记录是否已完成下游同步"},
	"error_message": {name: "错误信息", description: "最近一次处理失败的错误说明"},
}

var excelMatchModelDefinitions = buildExcelMatchModelDefinitions()

// ListModels returns every current database table and column with its Go model
// mapping when one exists. Database-only custom tables receive explicit
// fallback names and explanations instead of disappearing from the selector.
func (s *ExcelMatchJobService) ListModels(ctx context.Context) ([]ExcelMatchModel, error) {
	columns, err := s.jobDAO.ListModelColumns(ctx)
	if err != nil {
		return nil, err
	}
	return buildExcelMatchModelCatalog(columns), nil
}

func buildExcelMatchModelDefinitions() map[string]excelMatchModelDefinition {
	definitions := make(map[string]excelMatchModelDefinition, len(excelMatchRegisteredModels))
	for _, registered := range excelMatchRegisteredModels {
		parsed, err := schema.Parse(registered.value, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			continue
		}
		fields := make(map[string]excelMatchRegisteredField, len(parsed.Fields))
		for _, field := range parsed.Fields {
			if field.DBName == "" {
				continue
			}
			fields[field.DBName] = excelMatchRegisteredField{
				modelField: field.Name,
				comment:    strings.TrimSpace(field.Comment),
			}
		}
		definitions[parsed.Table] = excelMatchModelDefinition{
			modelName:   parsed.Name,
			name:        registered.name,
			description: registered.description,
			fields:      fields,
		}
	}
	return definitions
}

func buildExcelMatchModelCatalog(columns []data_dao.ExcelMatchModelColumn) []ExcelMatchModel {
	modelByTable := make(map[string]*ExcelMatchModel)
	for _, column := range columns {
		tableName := strings.TrimSpace(column.TableName)
		columnName := strings.TrimSpace(column.ColumnName)
		if tableName == "" || columnName == "" {
			continue
		}

		catalogModel, ok := modelByTable[tableName]
		if !ok {
			definition, registered := excelMatchModelDefinitions[tableName]
			modelName := tableName
			name := strings.TrimSpace(column.TableComment)
			description := name
			if registered {
				modelName = definition.modelName
				name = definition.name
				description = definition.description
			}
			if name == "" {
				name = tableName
			}
			if description == "" {
				description = fmt.Sprintf("数据库自定义模型，对应数据表 %s", tableName)
			}
			catalogModel = &ExcelMatchModel{
				Name:        name,
				ModelName:   modelName,
				TableName:   tableName,
				Description: description,
				Mapping:     fmt.Sprintf("%s（%s） → 数据库表 %s", name, modelName, tableName),
				Fields:      make([]ExcelMatchModelField, 0),
			}
			modelByTable[tableName] = catalogModel
		}

		definition, registered := excelMatchModelDefinitions[tableName]
		registeredField, fieldRegistered := definition.fields[columnName]
		modelField := columnName
		if registered && fieldRegistered {
			modelField = registeredField.modelField
		}
		label, labeled := excelMatchFieldLabels[tableName+"."+columnName]
		if !labeled {
			label, labeled = excelMatchCommonFieldLabels[columnName]
		}
		name := strings.TrimSpace(column.ColumnComment)
		baseDescription := name
		if labeled {
			name = label.name
			baseDescription = label.description
		} else if name == "" && registeredField.comment != "" {
			name = registeredField.comment
			baseDescription = registeredField.comment
		} else if name == "" && fieldRegistered {
			name = modelField
		}
		if name == "" {
			name = columnName
		}

		dataType := strings.TrimSpace(column.ColumnType)
		if dataType == "" {
			dataType = strings.TrimSpace(column.DataType)
		}
		if dataType == "" {
			dataType = "未知类型"
		}
		mapping := fmt.Sprintf("%s.%s → %s.%s", catalogModel.ModelName, modelField, tableName, columnName)
		description := fmt.Sprintf("模型字段 %s；数据库列 %s.%s；类型 %s", modelField, tableName, columnName, dataType)
		if baseDescription != "" {
			description = baseDescription + "；" + description
		}

		catalogModel.Fields = append(catalogModel.Fields, ExcelMatchModelField{
			Name:        name,
			ModelField:  modelField,
			ColumnName:  columnName,
			DataType:    dataType,
			Description: description,
			Mapping:     mapping,
			Nullable:    strings.EqualFold(strings.TrimSpace(column.IsNullable), "YES"),
			position:    column.OrdinalPosition,
		})
	}

	models := make([]ExcelMatchModel, 0, len(modelByTable))
	for _, catalogModel := range modelByTable {
		sort.SliceStable(catalogModel.Fields, func(i, j int) bool {
			if catalogModel.Fields[i].position == catalogModel.Fields[j].position {
				return catalogModel.Fields[i].ColumnName < catalogModel.Fields[j].ColumnName
			}
			return catalogModel.Fields[i].position < catalogModel.Fields[j].position
		})
		models = append(models, *catalogModel)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].TableName < models[j].TableName
	})
	return models
}
