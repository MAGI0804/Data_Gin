package data_svc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
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
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
		return result, errors.New("mall weather feishu rows: invalid columns")
	}
	result.Rows = make([][]feishu.SheetCell, len(dataRows))
	var previousCursor uint
	for rowIndex, dataRow := range dataRows {
		if dataRow.CursorID == 0 || dataRow.CursorID <= previousCursor || len(dataRow.Values) != len(columns) {
			return mallWeatherFeishuRenderedBatch{}, errors.New("mall weather feishu rows: invalid data row")
		}
		previousCursor = dataRow.CursorID
		result.Rows[rowIndex] = make([]feishu.SheetCell, len(columns))
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
			cell, err := mallWeatherFeishuCellFromRenderedValue(excelCell.Value)
			if err != nil {
				return mallWeatherFeishuRenderedBatch{}, err
			}
			result.Rows[rowIndex][columnIndex] = cell
		}
	}
	result.Checksum, err = checksumMallWeatherFeishuRows(result.Rows, len(result.Rows), len(columns))
	if err != nil {
		return mallWeatherFeishuRenderedBatch{}, err
	}
	result.FirstCursor = dataRows[0].CursorID
	result.LastCursor = dataRows[len(dataRows)-1].CursorID
	return result, nil
}

func mallWeatherFeishuCellFromRenderedValue(
	value interface{},
) (feishu.SheetCell, error) {
	switch typed := value.(type) {
	case nil:
		return feishu.SheetCell{Type: feishu.SheetCellBlank}, nil
	case string:
		return feishu.SheetCell{Type: feishu.SheetCellString, Text: typed}, nil
	case bool:
		return feishu.SheetCell{Type: feishu.SheetCellBoolean, Boolean: typed}, nil
	case float64:
		number := strconv.FormatFloat(typed, 'g', -1, 64)
		return feishu.SheetCell{Type: feishu.SheetCellNumber, Number: json.Number(number)}, nil
	default:
		return feishu.SheetCell{}, errors.New("mall weather feishu rows: unsupported rendered value")
	}
}

func checksumMallWeatherFeishuRows(rows [][]feishu.SheetCell, expectedRows int, expectedColumns int) (string, error) {
	if expectedRows < 1 || expectedRows > maxMallWeatherFeishuBatchRows || expectedColumns < 1 ||
		expectedColumns > maxMallWeatherFeishuColumns || len(rows) > expectedRows {
		return "", errors.New("mall weather feishu rows: invalid checksum dimensions")
	}
	canonical := make([][]mallWeatherFeishuCanonicalCell, expectedRows)
	for rowIndex := 0; rowIndex < expectedRows; rowIndex++ {
		canonical[rowIndex] = make([]mallWeatherFeishuCanonicalCell, expectedColumns)
		for columnIndex := range canonical[rowIndex] {
			canonical[rowIndex][columnIndex].Type = feishu.SheetCellBlank
		}
		if rowIndex >= len(rows) {
			continue
		}
		if len(rows[rowIndex]) > expectedColumns {
			return "", errors.New("mall weather feishu rows: invalid checksum dimensions")
		}
		for columnIndex, cell := range rows[rowIndex] {
			normalized, err := canonicalMallWeatherFeishuCell(cell)
			if err != nil {
				return "", err
			}
			canonical[rowIndex][columnIndex] = normalized
		}
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", errors.New("mall weather feishu rows: checksum encoding failed")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalMallWeatherFeishuCell(cell feishu.SheetCell) (mallWeatherFeishuCanonicalCell, error) {
	switch cell.Type {
	case feishu.SheetCellBlank:
		if cell.Text != "" || cell.Number != "" || cell.Boolean {
			return mallWeatherFeishuCanonicalCell{}, errors.New("mall weather feishu rows: invalid blank cell")
		}
		return mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellBlank}, nil
	case feishu.SheetCellString:
		if cell.Number != "" || cell.Boolean {
			return mallWeatherFeishuCanonicalCell{}, errors.New("mall weather feishu rows: invalid string cell")
		}
		if cell.Text == "" {
			return mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellBlank}, nil
		}
		return mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellString, Text: cell.Text}, nil
	case feishu.SheetCellBoolean:
		if cell.Text != "" || cell.Number != "" {
			return mallWeatherFeishuCanonicalCell{}, errors.New("mall weather feishu rows: invalid boolean cell")
		}
		return mallWeatherFeishuCanonicalCell{Type: feishu.SheetCellBoolean, Boolean: cell.Boolean}, nil
	case feishu.SheetCellNumber:
		if cell.Text != "" || cell.Number == "" || cell.Boolean {
			return mallWeatherFeishuCanonicalCell{}, errors.New("mall weather feishu rows: invalid number cell")
		}
		number, err := strconv.ParseFloat(cell.Number.String(), 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return mallWeatherFeishuCanonicalCell{}, errors.New("mall weather feishu rows: invalid number cell")
		}
		return mallWeatherFeishuCanonicalCell{
			Type: feishu.SheetCellNumber, Number: strconv.FormatFloat(number, 'g', -1, 64),
		}, nil
	default:
		return mallWeatherFeishuCanonicalCell{}, errors.New("mall weather feishu rows: unsupported cell type")
	}
}
