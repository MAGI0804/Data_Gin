package data_svc

import (
	"fmt"
	"strings"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
)

var mallWeatherExportChineseColumnTitles = map[string]string{
	"adcode": "行政区划代码", "address": "地址", "alert_id": "预警编号",
	"alert_latitude": "预警纬度", "alert_level_code": "预警级别代码", "alert_level_name": "预警级别",
	"alert_longitude": "预警经度", "alert_type_code": "预警类型代码", "alert_type_name": "预警类型",
	"apparent_temperature_c": "体感温度（℃）", "aqi_avg_chn": "中国空气质量指数平均值", "aqi_avg_usa": "美国空气质量指数平均值",
	"aqi_chn": "中国空气质量指数", "aqi_desc_chn": "中国空气质量描述", "aqi_desc_usa": "美国空气质量描述",
	"aqi_max_chn": "中国空气质量指数最高值", "aqi_max_usa": "美国空气质量指数最高值", "aqi_min_chn": "中国空气质量指数最低值",
	"aqi_min_usa": "美国空气质量指数最低值", "aqi_usa": "美国空气质量指数", "attempt_count": "尝试次数",
	"city": "城市", "cloudrate_avg_ratio": "平均云量比例", "cloudrate_max_ratio": "最高云量比例",
	"cloudrate_min_ratio": "最低云量比例", "cloudrate_ratio": "云量比例", "co_mg_m3": "一氧化碳（mg/m³）",
	"code": "代码", "comfort_desc": "舒适度描述", "comfort_index": "舒适度指数",
	"coordinate_system": "坐标系", "county": "区县", "coverage_radius_m": "覆盖半径（米）",
	"datasource": "数据源", "day_precipitation_avg_mm_h": "白天平均降水强度（mm/h）",
	"day_precipitation_max_mm_h": "白天最高降水强度（mm/h）", "day_precipitation_min_mm_h": "白天最低降水强度（mm/h）",
	"day_precipitation_probability_pct": "白天降水概率（%）", "day_skycon": "白天天气现象",
	"day_temperature_avg_c": "白天平均温度（℃）", "day_temperature_max_c": "白天最高温度（℃）",
	"day_temperature_min_c": "白天最低温度（℃）", "day_wind_avg_direction_deg": "白天平均风向（度）",
	"day_wind_avg_speed_kph": "白天平均风速（km/h）", "day_wind_max_direction_deg": "白天最大风风向（度）",
	"day_wind_max_speed_kph": "白天最大风速（km/h）", "day_wind_min_direction_deg": "白天最小风风向（度）",
	"day_wind_min_speed_kph": "白天最小风速（km/h）", "description": "描述", "detail": "详细说明",
	"district": "区县", "dswrf_avg_w_m2": "平均短波辐射（W/m²）", "dswrf_max_w_m2": "最高短波辐射（W/m²）",
	"dswrf_min_w_m2": "最低短波辐射（W/m²）", "dswrf_w_m2": "短波辐射（W/m²）", "duration_ms": "耗时（毫秒）",
	"ended_at": "结束时间", "endpoint_kind": "接口类型", "error_class": "错误类别", "error_code": "错误代码",
	"fetched_at": "采集时间", "finished_at": "完成时间", "first_seen_at": "首次发现时间",
	"forecast_date": "预报日期", "forecast_keypoint": "预报关键点", "forecast_minute": "预报分钟",
	"forecast_time": "预报时间", "hourly_description": "逐小时描述", "http_status": "HTTP状态码",
	"humidity_avg_pct": "平均湿度（%）", "humidity_max_pct": "最高湿度（%）", "humidity_min_pct": "最低湿度（%）",
	"humidity_pct": "湿度（%）", "index_code": "指数代码", "index_name": "指数名称", "index_type": "指数类型",
	"is_unknown_type": "是否未知类型", "issued_at": "发布时间", "last_seen_at": "最近发现时间",
	"latitude": "纬度", "level": "等级", "local_precip_datasource": "本地降水数据源",
	"local_precip_status": "本地降水状态", "location": "预警区域", "longitude": "经度",
	"mall_code": "商场编码", "minute_offset": "分钟偏移", "name_cn": "商场中文名", "name_en": "商场英文名",
	"nearest_precip_distance_km": "最近降水距离（km）", "nearest_precip_status": "最近降水状态",
	"nearest_precipitation_mm_h": "最近降水强度（mm/h）", "night_precipitation_avg_mm_h": "夜间平均降水强度（mm/h）",
	"night_precipitation_max_mm_h": "夜间最高降水强度（mm/h）", "night_precipitation_min_mm_h": "夜间最低降水强度（mm/h）",
	"night_precipitation_probability_pct": "夜间降水概率（%）", "night_skycon": "夜间天气现象",
	"night_temperature_avg_c": "夜间平均温度（℃）", "night_temperature_max_c": "夜间最高温度（℃）",
	"night_temperature_min_c": "夜间最低温度（℃）", "night_wind_avg_direction_deg": "夜间平均风向（度）",
	"night_wind_avg_speed_kph": "夜间平均风速（km/h）", "night_wind_max_direction_deg": "夜间最大风风向（度）",
	"night_wind_max_speed_kph": "夜间最大风速（km/h）", "night_wind_min_direction_deg": "夜间最小风风向（度）",
	"night_wind_min_speed_kph": "夜间最小风速（km/h）", "no2_ug_m3": "二氧化氮（μg/m³）",
	"o3_ug_m3": "臭氧（μg/m³）", "pm10_ug_m3": "可吸入颗粒物PM10（μg/m³）", "pm25_avg_ug_m3": "细颗粒物PM2.5平均值（μg/m³）",
	"pm25_max_ug_m3": "PM2.5最高值（μg/m³）", "pm25_min_ug_m3": "PM2.5最低值（μg/m³）",
	"pm25_ug_m3": "细颗粒物PM2.5（μg/m³）", "precipitation_avg_mm_h": "平均降水强度（mm/h）",
	"precipitation_max_mm_h": "最高降水强度（mm/h）", "precipitation_min_mm_h": "最低降水强度（mm/h）",
	"precipitation_mm_h": "降水强度（mm/h）", "precipitation_probability_pct": "降水概率（%）",
	"pressure_avg_pa": "平均气压（Pa）", "pressure_max_pa": "最高气压（Pa）", "pressure_min_pa": "最低气压（Pa）",
	"pressure_pa": "气压（Pa）", "probability_pct": "概率（%）", "probability_window": "概率窗口",
	"provider_status": "供应商状态", "province": "省份", "published_at": "发布时间", "quality_status": "质量状态",
	"region_id": "区域编号", "row_counts": "写入行数", "run_uuid": "运行编号", "sampling_mode": "采样模式",
	"short_desc": "简要说明", "skycon": "天气现象", "snapshot_at": "实况时间", "so2_ug_m3": "二氧化硫（μg/m³）",
	"source": "发布来源", "source_api": "来源接口", "started_at": "开始时间", "status": "状态",
	"sunrise": "日出时间", "sunset": "日落时间", "task_kind": "任务类型", "temperature_avg_c": "平均温度（℃）",
	"temperature_c": "温度（℃）", "temperature_max_c": "最高温度（℃）", "temperature_min_c": "最低温度（℃）",
	"title": "标题", "ultraviolet_desc": "紫外线描述", "ultraviolet_index": "紫外线指数",
	"visibility_avg_km": "平均能见度（km）", "visibility_km": "能见度（km）", "visibility_max_km": "最高能见度（km）",
	"visibility_min_km": "最低能见度（km）", "weather_coordinate_system": "天气坐标系", "weather_latitude": "天气纬度",
	"weather_longitude": "天气经度", "weather_provider": "天气供应商", "wind_avg_direction_deg": "平均风向（度）",
	"wind_avg_speed_kph": "平均风速（km/h）", "wind_direction_deg": "风向（度）", "wind_max_direction_deg": "最大风风向（度）",
	"wind_max_speed_kph": "最大风速（km/h）", "wind_min_direction_deg": "最小风风向（度）",
	"wind_min_speed_kph": "最小风速（km/h）", "wind_speed_kph": "风速（km/h）",
}

func mallWeatherExportChineseColumnTitle(field string) (string, error) {
	title, ok := mallWeatherExportChineseColumnTitles[field]
	if !ok || title == "" {
		return "", fmt.Errorf("mall weather export renderer: missing Chinese column title for %s", field)
	}
	return title, nil
}

func mallWeatherExportChineseColumns(kind string) ([]requestbody.MallWeatherExportColumn, error) {
	fields, ok := data_dao.MallWeatherExportDefaultFields(kind)
	if !ok {
		return nil, fmt.Errorf("mall weather export renderer: unsupported dataset")
	}
	columns := make([]requestbody.MallWeatherExportColumn, len(fields))
	for index, field := range fields {
		title, err := mallWeatherExportChineseColumnTitle(field)
		if err != nil {
			return nil, err
		}
		format := "general"
		if strings.Contains(field, "_at") || strings.Contains(field, "_time") || field == "forecast_minute" {
			format = "datetime"
		} else if strings.Contains(field, "_date") {
			format = "date"
		}
		columns[index] = requestbody.MallWeatherExportColumn{
			Field: field, Title: title, Width: 18, Format: format,
		}
	}
	return columns, nil
}
