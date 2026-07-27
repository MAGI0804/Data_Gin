package data_svc

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

const mallWeatherExportScopeSheetName = "导出说明"

type mallWeatherExportScopeRow struct {
	label string
	value string
}

func writeMallWeatherExportScopeSheet(
	file *excelize.File,
	namer *mallWeatherExportSheetNamer,
	request MallWeatherExportRenderRequest,
	styles mallWeatherExportExcelStyles,
) error {
	if file == nil || namer == nil || !reservedFixedMallWeatherExportProfileCode(request.ProfileCode) ||
		request.GeneratedAt.IsZero() || request.SnapshotAt.IsZero() {
		return fmt.Errorf("mall weather export scope sheet: invalid request")
	}
	location, err := time.LoadLocation(request.Config.TimeZone)
	if err != nil {
		return fmt.Errorf("mall weather export scope sheet: load time zone: %w", err)
	}
	name, err := namer.Name(mallWeatherExportScopeSheetName, "", 1)
	if err != nil {
		return fmt.Errorf("mall weather export scope sheet: name sheet: %w", err)
	}
	if err := file.SetSheetName("Sheet1", name); err != nil {
		return fmt.Errorf("mall weather export scope sheet: rename sheet: %w", err)
	}
	rows := []mallWeatherExportScopeRow{
		{label: "生成时间", value: request.GeneratedAt.In(location).Format(request.Config.DateTimeFormat)},
		{label: "数据快照截止时间", value: request.SnapshotAt.In(location).Format(request.Config.DateTimeFormat)},
		{label: "时区", value: request.Config.TimeZone},
		{label: "代表点", value: "商场中心点"},
		{label: "业务半径", value: "1 km"},
		{label: "数据供应商", value: "彩云天气"},
		{label: "实况空间分辨率", value: "商场中心点约 1 km 级，局地结果受站点覆盖影响"},
		{label: "分钟降水空间分辨率", value: "商场中心点未来两小时约 1 km 级"},
		{label: "小时与逐日空间分辨率", value: "商场中心点所在约 9～13 km 预报网格"},
		{label: "近时降水说明", value: "小时预报前两小时的降水相关结果可能细化至约 1 km"},
		{label: "预警范围", value: "按商场所在行政区划发布"},
		{label: "口径声明", value: "1 km 业务半径不代表所有天气字段均为 1 km 精度"},
	}
	header := []mallWeatherExportExcelCell{
		{StyleID: styles.Header, Value: "项目"},
		{StyleID: styles.Header, Value: "说明"},
	}
	if err := setMallWeatherExportRegularRow(file, name, 1, header); err != nil {
		return fmt.Errorf("mall weather export scope sheet: write header: %w", err)
	}
	for index, row := range rows {
		cells := []mallWeatherExportExcelCell{
			{StyleID: styles.Text, Value: row.label},
			{StyleID: styles.Text, Value: row.value},
		}
		if err := setMallWeatherExportRegularRow(file, name, index+2, cells); err != nil {
			return fmt.Errorf("mall weather export scope sheet: write row: %w", err)
		}
	}
	if err := file.SetColWidth(name, "A", "A", 24); err != nil {
		return fmt.Errorf("mall weather export scope sheet: set label width: %w", err)
	}
	if err := file.SetColWidth(name, "B", "B", 72); err != nil {
		return fmt.Errorf("mall weather export scope sheet: set value width: %w", err)
	}
	if err := file.SetPanes(name, mallWeatherExportHeaderPanes()); err != nil {
		return fmt.Errorf("mall weather export scope sheet: freeze header: %w", err)
	}
	return nil
}
