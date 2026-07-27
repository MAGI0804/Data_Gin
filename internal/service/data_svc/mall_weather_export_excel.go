package data_svc

import (
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const (
	maxMallWeatherExportWorkbookSheets = 256
	mallWeatherExportExcelMaxRows      = 1_048_576
)

var mallWeatherExportDateTokenPattern = regexp.MustCompile(`\{\{date:([^{}]{1,64})\}\}`)

var mallWeatherExportNumericFields = map[string]bool{
	"longitude": true, "latitude": true, "weather_longitude": true, "weather_latitude": true, "coverage_radius_m": true,
	"temperature_c": true, "apparent_temperature_c": true, "temperature_max_c": true,
	"temperature_min_c": true, "temperature_avg_c": true,
	"day_temperature_max_c": true, "day_temperature_min_c": true, "day_temperature_avg_c": true,
	"night_temperature_max_c": true, "night_temperature_min_c": true, "night_temperature_avg_c": true,
	"humidity_pct": true, "humidity_max_pct": true, "humidity_min_pct": true, "humidity_avg_pct": true,
	"cloudrate_ratio": true, "cloudrate_max_ratio": true, "cloudrate_min_ratio": true, "cloudrate_avg_ratio": true,
	"pressure_pa": true, "wind_speed_kph": true, "wind_direction_deg": true,
	"wind_max_speed_kph": true, "wind_max_direction_deg": true, "wind_min_speed_kph": true, "wind_min_direction_deg": true,
	"wind_avg_speed_kph": true, "wind_avg_direction_deg": true,
	"day_wind_max_speed_kph": true, "day_wind_max_direction_deg": true, "day_wind_min_speed_kph": true,
	"day_wind_min_direction_deg": true, "day_wind_avg_speed_kph": true, "day_wind_avg_direction_deg": true,
	"night_wind_max_speed_kph": true, "night_wind_max_direction_deg": true, "night_wind_min_speed_kph": true,
	"night_wind_min_direction_deg": true, "night_wind_avg_speed_kph": true, "night_wind_avg_direction_deg": true,
	"precipitation_mm_h": true, "precipitation_max_mm_h": true, "precipitation_min_mm_h": true,
	"precipitation_avg_mm_h": true, "precipitation_probability_pct": true,
	"day_precipitation_max_mm_h": true, "day_precipitation_min_mm_h": true, "day_precipitation_avg_mm_h": true,
	"day_precipitation_probability_pct": true, "night_precipitation_max_mm_h": true,
	"night_precipitation_min_mm_h": true, "night_precipitation_avg_mm_h": true,
	"night_precipitation_probability_pct": true, "probability_pct": true, "probability_window": true,
	"nearest_precip_distance_km": true, "nearest_precipitation_mm_h": true,
	"visibility_km": true, "visibility_max_km": true, "visibility_min_km": true, "visibility_avg_km": true,
	"dswrf_w_m2": true, "dswrf_max_w_m2": true, "dswrf_min_w_m2": true, "dswrf_avg_w_m2": true,
	"aqi_chn": true, "aqi_usa": true, "aqi_max_chn": true, "aqi_min_chn": true, "aqi_avg_chn": true,
	"aqi_max_usa": true, "aqi_min_usa": true, "aqi_avg_usa": true,
	"pm25_ug_m3": true, "pm10_ug_m3": true, "o3_ug_m3": true, "so2_ug_m3": true, "no2_ug_m3": true,
	"co_mg_m3": true, "pm25_max_ug_m3": true, "pm25_min_ug_m3": true, "pm25_avg_ug_m3": true,
	"comfort_index": true, "ultraviolet_index": true, "alert_latitude": true, "alert_longitude": true,
	"minute_offset": true, "index_type": true, "level": true, "attempt_count": true, "duration_ms": true, "http_status": true,
}

type mallWeatherExportExcelStyles struct {
	Header  int
	Text    int
	Integer int
	Decimal int
	Percent int
}

type mallWeatherExportExcelCell struct {
	StyleID int
	Value   interface{}
}

type mallWeatherExportSheetNamer struct {
	maxSheets  int
	byIdentity map[string]string
	owners     map[string]string
}

func newMallWeatherExportExcelStyles(file *excelize.File) (mallWeatherExportExcelStyles, error) {
	if file == nil {
		return mallWeatherExportExcelStyles{}, fmt.Errorf("mall weather export excel: nil workbook")
	}
	styles := mallWeatherExportExcelStyles{}
	definitions := []struct {
		destination *int
		style       *excelize.Style
	}{
		{&styles.Header, &excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E78"}, Pattern: 1},
			Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		}},
		{&styles.Text, &excelize.Style{NumFmt: 49}},
		{&styles.Integer, &excelize.Style{CustomNumFmt: stringPointer("0")}},
		{&styles.Decimal, &excelize.Style{CustomNumFmt: stringPointer("0.00")}},
		{&styles.Percent, &excelize.Style{CustomNumFmt: stringPointer("0.00%")}},
	}
	for _, definition := range definitions {
		styleID, err := file.NewStyle(definition.style)
		if err != nil {
			return mallWeatherExportExcelStyles{}, fmt.Errorf("mall weather export excel: create style: %w", err)
		}
		*definition.destination = styleID
	}
	return styles, nil
}

func mallWeatherExportExcelValue(
	field string,
	format string,
	unitSystem string,
	location *time.Location,
	dateFormat string,
	dateTimeFormat string,
	value interface{},
	styles mallWeatherExportExcelStyles,
) (mallWeatherExportExcelCell, error) {
	if location == nil || (unitSystem != "metric" && unitSystem != "imperial") {
		return mallWeatherExportExcelCell{}, fmt.Errorf("mall weather export excel: invalid value configuration")
	}
	if value == nil {
		return mallWeatherExportExcelCell{}, nil
	}
	if typed, ok := value.(time.Time); ok {
		layout := dateTimeFormat
		if format == "date" {
			layout = dateFormat
		}
		return mallWeatherExportExcelCell{StyleID: styles.Text, Value: typed.In(location).Format(layout)}, nil
	}
	if format == "date" || format == "datetime" {
		if text, ok := value.(string); ok {
			parsed, err := parseMallWeatherExportTime(text)
			if err != nil {
				return mallWeatherExportExcelCell{}, err
			}
			layout := dateTimeFormat
			if format == "date" {
				layout = dateFormat
			}
			return mallWeatherExportExcelCell{StyleID: styles.Text, Value: parsed.In(location).Format(layout)}, nil
		}
		return mallWeatherExportExcelCell{}, fmt.Errorf("mall weather export excel: invalid time value")
	}
	if typed, ok := value.(bool); ok {
		return mallWeatherExportExcelCell{Value: typed}, nil
	}
	converted, numeric, err := mallWeatherExportNumericValue(field, format, unitSystem, value)
	if err != nil {
		return mallWeatherExportExcelCell{}, err
	}
	if numeric {
		styleID := styles.Decimal
		switch format {
		case "integer":
			styleID = styles.Integer
		case "percent":
			converted /= 100
			styleID = styles.Percent
		case "general":
			styleID = 0
		}
		return mallWeatherExportExcelCell{StyleID: styleID, Value: converted}, nil
	}
	return mallWeatherExportExcelCell{StyleID: styles.Text, Value: fmt.Sprint(value)}, nil
}

func setMallWeatherExportRegularRow(
	file *excelize.File,
	sheet string,
	rowNumber int,
	cells []mallWeatherExportExcelCell,
) error {
	if file == nil || sheet == "" || rowNumber < 1 || len(cells) == 0 {
		return fmt.Errorf("mall weather export excel: invalid regular row")
	}
	for index, cell := range cells {
		coordinate, err := excelize.CoordinatesToCellName(index+1, rowNumber)
		if err != nil {
			return fmt.Errorf("mall weather export excel: cell coordinate: %w", err)
		}
		if err := file.SetCellValue(sheet, coordinate, cell.Value); err != nil {
			return fmt.Errorf("mall weather export excel: set cell: %w", err)
		}
		if cell.StyleID != 0 {
			if err := file.SetCellStyle(sheet, coordinate, coordinate, cell.StyleID); err != nil {
				return fmt.Errorf("mall weather export excel: style cell: %w", err)
			}
		}
	}
	return nil
}

func mallWeatherExportStreamRow(cells []mallWeatherExportExcelCell) []interface{} {
	row := make([]interface{}, len(cells))
	for index, cell := range cells {
		row[index] = excelize.Cell{StyleID: cell.StyleID, Value: cell.Value}
	}
	return row
}

func mallWeatherExportNumericValue(
	field string,
	format string,
	unitSystem string,
	value interface{},
) (float64, bool, error) {
	numericFormat := format == "integer" || format == "decimal" || format == "percent" ||
		(format == "general" && mallWeatherExportNumericFields[field])
	requiresUnitConversion := unitSystem == "imperial" && mallWeatherExportConvertibleField(field)
	if !numericFormat && !requiresUnitConversion {
		switch typed := value.(type) {
		case int:
			return float64(typed), true, nil
		case int64:
			return float64(typed), true, nil
		case uint:
			return float64(typed), true, nil
		case uint64:
			return float64(typed), true, nil
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) {
				return 0, false, fmt.Errorf("mall weather export excel: non-finite number")
			}
			return typed, true, nil
		default:
			return 0, false, nil
		}
	}
	number, err := mallWeatherExportFloat(value)
	if err != nil {
		return 0, false, err
	}
	if requiresUnitConversion {
		switch {
		case strings.HasSuffix(field, "_c"):
			number = number*9/5 + 32
		case strings.HasSuffix(field, "_kph"):
			number /= 1.609344
		case strings.Contains(field, "_mm_h"):
			number /= 25.4
		case strings.HasSuffix(field, "_pa"):
			number /= 3386.389
		}
	}
	return number, true, nil
}

func mallWeatherExportFloat(value interface{}) (float64, error) {
	var number float64
	switch typed := value.(type) {
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case uint:
		number = float64(typed)
	case uint64:
		number = float64(typed)
	case float32:
		number = float64(typed)
	case float64:
		number = typed
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, fmt.Errorf("mall weather export excel: invalid numeric value")
		}
		number = parsed
	default:
		return 0, fmt.Errorf("mall weather export excel: invalid numeric value")
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("mall weather export excel: non-finite number")
	}
	return number, nil
}

func mallWeatherExportConvertibleField(field string) bool {
	return strings.HasSuffix(field, "_c") || strings.HasSuffix(field, "_kph") ||
		strings.Contains(field, "_mm_h") || strings.HasSuffix(field, "_pa")
}

func parseMallWeatherExportTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999", "2006-01-02 15:04:05", time.DateOnly} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("mall weather export excel: invalid time value")
}

func newMallWeatherExportSheetNamer(maxSheets int) (*mallWeatherExportSheetNamer, error) {
	if maxSheets < 1 || maxSheets > maxMallWeatherExportWorkbookSheets {
		return nil, fmt.Errorf("mall weather export excel: invalid sheet limit")
	}
	return &mallWeatherExportSheetNamer{
		maxSheets:  maxSheets,
		byIdentity: make(map[string]string, maxSheets),
		owners:     make(map[string]string, maxSheets),
	}, nil
}

func (namer *mallWeatherExportSheetNamer) Name(base string, splitValue string, part int) (string, error) {
	if namer == nil || part < 1 || base == "" {
		return "", fmt.Errorf("mall weather export excel: invalid sheet identity")
	}
	identity := base + "\x1f" + splitValue + "\x1f" + strconv.Itoa(part)
	if existing := namer.byIdentity[identity]; existing != "" {
		return existing, nil
	}
	if len(namer.byIdentity) >= namer.maxSheets {
		return "", fmt.Errorf("mall weather export excel: sheet limit exceeded")
	}
	components := []string{base}
	if splitValue != "" {
		components = append(components, splitValue)
	}
	if part > 1 {
		components = append(components, strconv.Itoa(part))
	}
	candidate := sanitizeMallWeatherExportSheetName(strings.Join(components, "_"))
	ownerKey := strings.ToLower(candidate)
	if owner, collision := namer.owners[ownerKey]; collision && owner != identity {
		sum := sha256.Sum256([]byte(identity))
		suffix := fmt.Sprintf("_%x", sum[:4])
		candidate = truncateMallWeatherExportRunes(candidate, 31-utf8.RuneCountInString(suffix)) + suffix
		ownerKey = strings.ToLower(candidate)
		if owner, collision = namer.owners[ownerKey]; collision && owner != identity {
			return "", fmt.Errorf("mall weather export excel: sheet name collision")
		}
	}
	namer.byIdentity[identity] = candidate
	namer.owners[ownerKey] = identity
	return candidate, nil
}

func sanitizeMallWeatherExportSheetName(value string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(value) {
		switch {
		case unicode.IsControl(char), strings.ContainsRune(`[]:*?/\\`, char), char == '\'':
			builder.WriteRune('_')
		default:
			builder.WriteRune(char)
		}
	}
	value = strings.Trim(builder.String(), " _")
	if value == "" {
		value = "Sheet"
	}
	return truncateMallWeatherExportRunes(value, 31)
}

func truncateMallWeatherExportRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func renderMallWeatherExportFileName(
	template string,
	now time.Time,
	location *time.Location,
) (string, error) {
	if location == nil || now.IsZero() {
		return "", fmt.Errorf("mall weather export excel: invalid file name context")
	}
	rendered := mallWeatherExportDateTokenPattern.ReplaceAllStringFunc(template, func(token string) string {
		matches := mallWeatherExportDateTokenPattern.FindStringSubmatch(token)
		return now.In(location).Format(matches[1])
	})
	if strings.ContainsAny(rendered, "/\\\x00\r\n") || strings.Contains(rendered, "{{") ||
		strings.Contains(rendered, "}}") || !strings.HasSuffix(strings.ToLower(rendered), ".xlsx") ||
		utf8.RuneCountInString(rendered) > 255 || strings.TrimSpace(rendered) != rendered {
		return "", fmt.Errorf("mall weather export excel: invalid rendered file name")
	}
	return rendered, nil
}

func stringPointer(value string) *string {
	return &value
}
