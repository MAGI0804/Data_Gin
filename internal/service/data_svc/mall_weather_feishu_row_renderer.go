package data_svc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
)

type mallWeatherFeishuRenderedBatch struct {
	Rows        [][]feishu.SheetCell `json:"-"`
	Checksum    string               `json:"-"`
	FirstCursor uint                 `json:"-"`
	LastCursor  uint                 `json:"-"`
}

func (mallWeatherFeishuRenderedBatch) String() string {
	return "data_svc.mallWeatherFeishuRenderedBatch{redacted}"
}

func (mallWeatherFeishuRenderedBatch) GoString() string {
	return "data_svc.mallWeatherFeishuRenderedBatch{redacted}"
}

type mallWeatherFeishuCanonicalCell struct {
	Type    feishu.SheetCellType `json:"t"`
	Text    string               `json:"s,omitempty"`
	Number  string               `json:"n,omitempty"`
	Boolean bool                 `json:"b,omitempty"`
}

func renderMallWeatherFeishuBatch(
	profile MallWeatherExportProfileDTO,
	dataset requestbody.MallWeatherExportDataset,
	dataRows []data_dao.MallWeatherExportDataRow,
) (mallWeatherFeishuRenderedBatch, error) {
	var result mallWeatherFeishuRenderedBatch
	if profile.ID == 0 || profile.Version == 0 || !profile.Enabled || len(dataRows) == 0 ||
		len(dataRows) > maxMallWeatherFeishuBatchRows || dataset.Kind == "" || dataset.SplitBy != "" {
		return result, errors.New("mall weather feishu rows: invalid batch")
	}
	location, err := time.LoadLocation(profile.TimeZone)
	if err != nil || (profile.UnitSystem != "metric" && profile.UnitSystem != "imperial") ||
		profile.DateFormat == "" || profile.DateTimeFormat == "" {
		return result, errors.New("mall weather feishu rows: invalid profile format")
	}
	columns, err := mallWeatherExportRenderColumns(dataset)
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherExportColumns {
		return result, errors.New("mall weather feishu rows: invalid columns")
	}
	result.Rows = make([][]feishu.SheetCell, len(dataRows))
	canonical := make([][]mallWeatherFeishuCanonicalCell, len(dataRows))
	var previousCursor uint
	for rowIndex, dataRow := range dataRows {
		if dataRow.CursorID == 0 || dataRow.CursorID <= previousCursor || len(dataRow.Values) != len(columns) {
			return mallWeatherFeishuRenderedBatch{}, errors.New("mall weather feishu rows: invalid data row")
		}
		previousCursor = dataRow.CursorID
		result.Rows[rowIndex] = make([]feishu.SheetCell, len(columns))
		canonical[rowIndex] = make([]mallWeatherFeishuCanonicalCell, len(columns))
		for columnIndex, column := range columns {
			value, exists := dataRow.Values[column.Field]
			if !exists {
				return mallWeatherFeishuRenderedBatch{}, errors.New("mall weather feishu rows: incomplete data row")
			}
			excelCell, err := mallWeatherExportExcelValue(
				column.Field,
				column.Format,
				profile.UnitSystem,
				location,
				profile.DateFormat,
				profile.DateTimeFormat,
				value,
				mallWeatherExportExcelStyles{},
			)
			if err != nil {
				return mallWeatherFeishuRenderedBatch{}, errors.New("mall weather feishu rows: render value failed")
			}
			cell, canonicalCell, err := mallWeatherFeishuCellFromRenderedValue(excelCell.Value)
			if err != nil {
				return mallWeatherFeishuRenderedBatch{}, err
			}
			result.Rows[rowIndex][columnIndex] = cell
			canonical[rowIndex][columnIndex] = canonicalCell
		}
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return mallWeatherFeishuRenderedBatch{}, errors.New("mall weather feishu rows: checksum encoding failed")
	}
	digest := sha256.Sum256(encoded)
	result.Checksum = hex.EncodeToString(digest[:])
	result.FirstCursor = dataRows[0].CursorID
	result.LastCursor = dataRows[len(dataRows)-1].CursorID
	return result, nil
}

func mallWeatherFeishuCellFromRenderedValue(
	value interface{},
) (feishu.SheetCell, mallWeatherFeishuCanonicalCell, error) {
	switch typed := value.(type) {
	case nil:
		return feishu.SheetCell{Type: feishu.SheetCellBlank},
			mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellBlank}, nil
	case string:
		return feishu.SheetCell{Type: feishu.SheetCellString, Text: typed},
			mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellString, Text: typed}, nil
	case bool:
		return feishu.SheetCell{Type: feishu.SheetCellBoolean, Boolean: typed},
			mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellBoolean, Boolean: typed}, nil
	case float64:
		number := strconv.FormatFloat(typed, 'g', -1, 64)
		return feishu.SheetCell{Type: feishu.SheetCellNumber, Number: json.Number(number)},
			mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellNumber, Number: number}, nil
	default:
		return feishu.SheetCell{}, mallWeatherFeishuCanonicalCell{},
			errors.New("mall weather feishu rows: unsupported rendered value")
	}
}
