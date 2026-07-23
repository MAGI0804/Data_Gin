package data_svc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
)

type mallWeatherFeishuUpsertRenderedRow struct {
	BusinessKey string
	Checksum    string
	Cells       []feishu.SheetCell
}

type mallWeatherFeishuCanonicalBusinessKey struct {
	Fields []string                         `json:"fields"`
	Values []mallWeatherFeishuCanonicalCell `json:"values"`
}

func buildMallWeatherFeishuUpsertRows(
	columns []requestbody.MallWeatherExportColumn,
	uniqueKeyFields []string,
	batch mallWeatherFeishuRenderedBatch,
) ([]mallWeatherFeishuUpsertRenderedRow, error) {
	if len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns || len(uniqueKeyFields) == 0 ||
		len(uniqueKeyFields) > 8 || len(batch.Rows) == 0 || len(batch.Rows) > maxMallWeatherFeishuBatchRows {
		return nil, errors.New("mall weather feishu upsert keys: invalid input")
	}
	columnIndex := make(map[string]int, len(columns))
	for index, column := range columns {
		if column.Field == "" {
			return nil, errors.New("mall weather feishu upsert keys: invalid column")
		}
		if _, duplicate := columnIndex[column.Field]; duplicate {
			return nil, errors.New("mall weather feishu upsert keys: duplicate column")
		}
		columnIndex[column.Field] = index
	}
	keyIndexes := make([]int, len(uniqueKeyFields))
	seenFields := make(map[string]struct{}, len(uniqueKeyFields))
	for index, field := range uniqueKeyFields {
		column, exists := columnIndex[field]
		if !exists || field == "" {
			return nil, errors.New("mall weather feishu upsert keys: unknown key field")
		}
		if _, duplicate := seenFields[field]; duplicate {
			return nil, errors.New("mall weather feishu upsert keys: duplicate key field")
		}
		seenFields[field] = struct{}{}
		keyIndexes[index] = column
	}
	result := make([]mallWeatherFeishuUpsertRenderedRow, len(batch.Rows))
	seenKeys := make(map[string]struct{}, len(batch.Rows))
	for rowIndex, row := range batch.Rows {
		if len(row) != len(columns) || mallWeatherFeishuFirstCellIsBlank(row) {
			return nil, errors.New("mall weather feishu upsert keys: invalid row")
		}
		key := mallWeatherFeishuCanonicalBusinessKey{
			Fields: append([]string(nil), uniqueKeyFields...),
			Values: make([]mallWeatherFeishuCanonicalCell, len(keyIndexes)),
		}
		for index, cellIndex := range keyIndexes {
			canonical, err := canonicalMallWeatherFeishuCell(row[cellIndex])
			if err != nil || canonical.Type == feishu.SheetCellBlank {
				return nil, errors.New("mall weather feishu upsert keys: blank or invalid key value")
			}
			key.Values[index] = canonical
		}
		encoded, err := json.Marshal(key)
		if err != nil {
			return nil, errors.New("mall weather feishu upsert keys: encode business key")
		}
		digest := sha256.Sum256(encoded)
		businessKey := "sha256:" + hex.EncodeToString(digest[:])
		if _, duplicate := seenKeys[businessKey]; duplicate {
			return nil, errors.New("mall weather feishu upsert keys: duplicate business key")
		}
		seenKeys[businessKey] = struct{}{}
		checksum, err := checksumMallWeatherFeishuRows([][]feishu.SheetCell{row}, 1, len(columns))
		if err != nil {
			return nil, errors.New("mall weather feishu upsert keys: checksum row")
		}
		result[rowIndex] = mallWeatherFeishuUpsertRenderedRow{
			BusinessKey: businessKey,
			Checksum:    checksum,
			Cells:       append([]feishu.SheetCell(nil), row...),
		}
	}
	return result, nil
}

func mallWeatherFeishuMappingSchemaChecksum(
	destination *MallWeatherFeishuResolvedDestination,
	datasetKind string,
	columns []requestbody.MallWeatherExportColumn,
) (string, error) {
	if destination == nil || destination.DestinationID == 0 || destination.Config.WriteMode != "upsert" ||
		datasetKind == "" || destination.SpreadsheetToken == "" || destination.SheetIDs[datasetKind] == "" ||
		destination.Config.SheetIDEnvMapping[datasetKind] == "" || len(columns) == 0 ||
		len(columns) > maxMallWeatherFeishuColumns || len(destination.Config.UniqueKeyFields[datasetKind]) == 0 {
		return "", errors.New("mall weather feishu upsert keys: invalid schema")
	}
	canonical := struct {
		Version         int                                   `json:"version"`
		DestinationID   uint                                  `json:"destinationId"`
		DatasetKind     string                                `json:"datasetKind"`
		Spreadsheet     string                                `json:"spreadsheet"`
		Sheet           string                                `json:"sheet"`
		SheetIDEnv      string                                `json:"sheetIdEnv"`
		Columns         []requestbody.MallWeatherExportColumn `json:"columns"`
		UniqueKeyFields []string                              `json:"uniqueKeyFields"`
	}{
		Version: 1, DestinationID: destination.DestinationID, DatasetKind: datasetKind,
		Spreadsheet: destination.SpreadsheetToken, Sheet: destination.SheetIDs[datasetKind],
		SheetIDEnv:      destination.Config.SheetIDEnvMapping[datasetKind],
		Columns:         append([]requestbody.MallWeatherExportColumn(nil), columns...),
		UniqueKeyFields: append([]string(nil), destination.Config.UniqueKeyFields[datasetKind]...),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", errors.New("mall weather feishu upsert keys: encode schema")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
