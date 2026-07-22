package data_svc

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
)

type MallWeatherFeishuColumnPlan struct {
	Index int    `json:"index"`
	Field string `json:"field"`
	Title string `json:"title"`
}

type MallWeatherFeishuHeaderDifference struct {
	Index      int    `json:"index"`
	Expected   string `json:"expected"`
	Actual     string `json:"actual,omitempty"`
	ActualType string `json:"actualType"`
}

type MallWeatherFeishuDatasetDryRunPlan struct {
	DatasetKind        string                              `json:"datasetKind"`
	SheetIDEnv         string                              `json:"sheetIdEnv"`
	SheetTitle         string                              `json:"sheetTitle"`
	WriteMode          string                              `json:"writeMode"`
	RangeStrategy      string                              `json:"rangeStrategy"`
	PlannedRange       string                              `json:"plannedRange,omitempty"`
	HeaderRange        string                              `json:"headerRange"`
	HeaderStatus       string                              `json:"headerStatus"`
	HeaderAction       string                              `json:"headerAction"`
	HeaderDifferences  []MallWeatherFeishuHeaderDifference `json:"headerDifferences"`
	Columns            []MallWeatherFeishuColumnPlan       `json:"columns"`
	UniqueKeyFields    []string                            `json:"uniqueKeyFields,omitempty"`
	EstimatedRows      int64                               `json:"estimatedRows"`
	EstimatedCells     int64                               `json:"estimatedCells"`
	GridRowCapacity    int64                               `json:"gridRowCapacity"`
	GridColumnCapacity int64                               `json:"gridColumnCapacity"`
	CanExecute         bool                                `json:"canExecute"`
	Warnings           []string                            `json:"warnings"`
}

type MallWeatherFeishuDryRunResult struct {
	DestinationID       uint                                 `json:"destinationId"`
	DestinationCode     string                               `json:"destinationCode"`
	ProfileID           uint                                 `json:"profileId"`
	ProfileCode         string                               `json:"profileCode"`
	ProfileVersion      uint64                               `json:"profileVersion"`
	SpreadsheetTokenEnv string                               `json:"spreadsheetTokenEnv"`
	WriteMode           string                               `json:"writeMode"`
	TotalEstimatedRows  int64                                `json:"totalEstimatedRows"`
	TotalEstimatedCells int64                                `json:"totalEstimatedCells"`
	CanExecute          bool                                 `json:"canExecute"`
	Datasets            []MallWeatherFeishuDatasetDryRunPlan `json:"datasets"`
	Warnings            []string                             `json:"warnings"`
}

type mallWeatherFeishuDryRunPlanInput struct {
	Destination   *MallWeatherFeishuResolvedDestination
	Profile       MallWeatherExportProfileDTO
	Metadata      *feishu.SpreadsheetMetadata
	Headers       map[string]*feishu.SheetValues
	EstimatedRows map[string]int64
}

func buildMallWeatherFeishuDryRunPlan(
	input mallWeatherFeishuDryRunPlanInput,
) (*MallWeatherFeishuDryRunResult, error) {
	if err := validateMallWeatherFeishuDryRunPlanInput(input); err != nil {
		return nil, err
	}
	metadataByID := make(map[string]feishu.SheetMetadata, len(input.Metadata.Sheets))
	for _, sheet := range input.Metadata.Sheets {
		metadataByID[sheet.SheetID] = sheet
	}
	result := &MallWeatherFeishuDryRunResult{
		DestinationID:       input.Destination.DestinationID,
		DestinationCode:     input.Destination.Code,
		ProfileID:           input.Profile.ID,
		ProfileCode:         input.Profile.Code,
		ProfileVersion:      input.Profile.Version,
		SpreadsheetTokenEnv: input.Destination.Config.SpreadsheetTokenEnv,
		WriteMode:           strings.ToUpper(input.Destination.Config.WriteMode),
		CanExecute:          true,
		Datasets:            make([]MallWeatherFeishuDatasetDryRunPlan, 0, len(input.Profile.Datasets)),
		Warnings:            []string{},
	}
	seenDatasets := make(map[string]struct{}, len(input.Profile.Datasets))
	for _, dataset := range input.Profile.Datasets {
		plan, err := buildMallWeatherFeishuDatasetDryRunPlan(input, dataset, metadataByID)
		if err != nil {
			return nil, err
		}
		if _, exists := seenDatasets[dataset.Kind]; exists {
			return nil, errors.New("mall weather feishu dry-run: duplicate profile dataset")
		}
		seenDatasets[dataset.Kind] = struct{}{}
		if result.TotalEstimatedRows > math.MaxInt64-plan.EstimatedRows ||
			result.TotalEstimatedCells > math.MaxInt64-plan.EstimatedCells {
			return nil, errors.New("mall weather feishu dry-run: estimate overflow")
		}
		result.TotalEstimatedRows += plan.EstimatedRows
		result.TotalEstimatedCells += plan.EstimatedCells
		result.CanExecute = result.CanExecute && plan.CanExecute
		result.Datasets = append(result.Datasets, plan)
	}
	if len(seenDatasets) != len(input.Destination.Config.SheetIDEnvMapping) {
		return nil, errors.New("mall weather feishu dry-run: profile and destination datasets differ")
	}
	return result, nil
}

func validateMallWeatherFeishuDryRunPlanInput(input mallWeatherFeishuDryRunPlanInput) error {
	if input.Destination == nil || input.Destination.DestinationID == 0 || input.Destination.Code == "" ||
		input.Profile.ID == 0 || input.Profile.Version == 0 || !input.Profile.Enabled ||
		input.Profile.Code != input.Destination.Config.ProfileCode || input.Metadata == nil ||
		input.Metadata.SpreadsheetToken != input.Destination.SpreadsheetToken ||
		len(input.Profile.Datasets) == 0 || len(input.Headers) == 0 || len(input.EstimatedRows) == 0 {
		return errors.New("mall weather feishu dry-run: invalid plan input")
	}
	return nil
}

func buildMallWeatherFeishuDatasetDryRunPlan(
	input mallWeatherFeishuDryRunPlanInput,
	dataset requestbody.MallWeatherExportDataset,
	metadataByID map[string]feishu.SheetMetadata,
) (MallWeatherFeishuDatasetDryRunPlan, error) {
	if dataset.SplitBy != "" {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: split datasets are unsupported")
	}
	sheetEnv, exists := input.Destination.Config.SheetIDEnvMapping[dataset.Kind]
	if !exists {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: dataset has no sheet mapping")
	}
	sheetID, exists := input.Destination.SheetIDs[dataset.Kind]
	if !exists {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: dataset resource is unavailable")
	}
	metadata, exists := metadataByID[sheetID]
	if !exists {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: dataset metadata is unavailable")
	}
	header, exists := input.Headers[dataset.Kind]
	if !exists || header == nil {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: dataset header is unavailable")
	}
	estimatedRows, exists := input.EstimatedRows[dataset.Kind]
	if !exists || estimatedRows < 0 || estimatedRows > maxMallWeatherExportConfiguredRows {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: dataset estimate is invalid")
	}
	columns, err := mallWeatherExportRenderColumns(dataset)
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: dataset columns are invalid")
	}
	uniqueKeys := input.Destination.Config.UniqueKeyFields[dataset.Kind]
	if err := validateMallWeatherFeishuPlannedUniqueKeys(input.Destination.Config.WriteMode, columns, uniqueKeys); err != nil {
		return MallWeatherFeishuDatasetDryRunPlan{}, err
	}
	if estimatedRows > math.MaxInt64/int64(len(columns)) {
		return MallWeatherFeishuDatasetDryRunPlan{}, errors.New("mall weather feishu dry-run: cell estimate overflow")
	}
	plan := MallWeatherFeishuDatasetDryRunPlan{
		DatasetKind:        dataset.Kind,
		SheetIDEnv:         sheetEnv,
		SheetTitle:         metadata.Title,
		WriteMode:          strings.ToUpper(input.Destination.Config.WriteMode),
		HeaderRange:        "A1:" + mallWeatherFeishuColumnName(len(columns)) + "1",
		Columns:            make([]MallWeatherFeishuColumnPlan, len(columns)),
		UniqueKeyFields:    append([]string(nil), uniqueKeys...),
		EstimatedRows:      estimatedRows,
		EstimatedCells:     estimatedRows * int64(len(columns)),
		GridRowCapacity:    metadata.GridProperties.RowCount,
		GridColumnCapacity: metadata.GridProperties.ColumnCount,
		CanExecute:         true,
		Warnings:           []string{},
	}
	for index, column := range columns {
		plan.Columns[index] = MallWeatherFeishuColumnPlan{Index: index + 1, Field: column.Field, Title: column.Title}
	}
	plan.RangeStrategy, plan.PlannedRange = mallWeatherFeishuPlannedRange(
		input.Destination.Config.WriteMode,
		len(columns),
		estimatedRows,
	)
	plan.HeaderStatus, plan.HeaderAction, plan.HeaderDifferences, err = mallWeatherFeishuHeaderPlan(columns, header)
	if err != nil {
		return MallWeatherFeishuDatasetDryRunPlan{}, err
	}
	if plan.HeaderStatus == "EMPTY" {
		plan.Warnings = append(plan.Warnings, "HEADER_WILL_BE_CREATED")
	}
	if plan.HeaderStatus == "MISMATCH" {
		if input.Destination.Config.AllowHeaderRewrite {
			plan.HeaderAction = "REWRITE_ON_EXECUTION"
			plan.Warnings = append(plan.Warnings, "HEADER_WILL_BE_REWRITTEN")
		} else {
			plan.HeaderAction = "BLOCKED"
			plan.CanExecute = false
			plan.Warnings = append(plan.Warnings, "HEADER_MISMATCH_REWRITE_DISABLED")
		}
	}
	if metadata.GridProperties.ColumnCount < int64(len(columns)) {
		plan.CanExecute = false
		plan.Warnings = append(plan.Warnings, "GRID_COLUMN_CAPACITY_INSUFFICIENT")
	}
	if metadata.GridProperties.RowCount < estimatedRows+1 {
		plan.CanExecute = false
		plan.Warnings = append(plan.Warnings, "GRID_ROW_CAPACITY_INSUFFICIENT")
	}
	if input.Destination.Config.WriteMode == "upsert" {
		plan.Warnings = append(plan.Warnings, "UPSERT_COUNTS_REQUIRE_ROW_MAPPING")
	}
	return plan, nil
}

func validateMallWeatherFeishuPlannedUniqueKeys(
	writeMode string,
	columns []requestbody.MallWeatherExportColumn,
	uniqueKeys []string,
) error {
	if writeMode != "upsert" {
		return nil
	}
	selected := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		selected[column.Field] = struct{}{}
	}
	for _, field := range uniqueKeys {
		if _, exists := selected[field]; !exists {
			return fmt.Errorf("mall weather feishu dry-run: unique key field %q is not selected", field)
		}
	}
	return nil
}

func mallWeatherFeishuHeaderPlan(
	columns []requestbody.MallWeatherExportColumn,
	header *feishu.SheetValues,
) (string, string, []MallWeatherFeishuHeaderDifference, error) {
	differences := make([]MallWeatherFeishuHeaderDifference, 0)
	if len(header.Rows) > 1 {
		return "", "", nil, errors.New("mall weather feishu dry-run: invalid header row count")
	}
	if len(header.Rows) == 0 || mallWeatherFeishuHeaderRowIsEmpty(header.Rows[0]) {
		return "EMPTY", "WRITE_ON_EXECUTION", differences, nil
	}
	for index, column := range columns {
		actual, actualType, err := mallWeatherFeishuHeaderCell(header.Rows, index)
		if err != nil {
			return "", "", nil, err
		}
		if actualType != "STRING" || actual != column.Title {
			differences = append(differences, MallWeatherFeishuHeaderDifference{
				Index: index + 1, Expected: column.Title, Actual: actual, ActualType: actualType,
			})
		}
	}
	if len(differences) > 0 {
		return "MISMATCH", "BLOCKED", differences, nil
	}
	return "MATCHED", "NONE", differences, nil
}

func mallWeatherFeishuHeaderRowIsEmpty(row []feishu.SheetCell) bool {
	for _, cell := range row {
		if cell.Type != feishu.SheetCellBlank && !(cell.Type == feishu.SheetCellString && cell.Text == "") {
			return false
		}
	}
	return true
}

func mallWeatherFeishuHeaderCell(rows [][]feishu.SheetCell, index int) (string, string, error) {
	if len(rows) == 0 || index >= len(rows[0]) {
		return "", "BLANK", nil
	}
	cell := rows[0][index]
	switch cell.Type {
	case feishu.SheetCellBlank:
		return "", "BLANK", nil
	case feishu.SheetCellString:
		if !validMallWeatherFeishuHeaderText(cell.Text) {
			return "", "", errors.New("mall weather feishu dry-run: unsafe remote header")
		}
		return cell.Text, "STRING", nil
	case feishu.SheetCellNumber:
		return "", "NUMBER", nil
	case feishu.SheetCellBoolean:
		return "", "BOOLEAN", nil
	default:
		return "", "", errors.New("mall weather feishu dry-run: invalid remote header type")
	}
}

func validMallWeatherFeishuHeaderText(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 255 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func mallWeatherFeishuPlannedRange(writeMode string, columnCount int, estimatedRows int64) (string, string) {
	endColumn := mallWeatherFeishuColumnName(columnCount)
	switch writeMode {
	case "append":
		return "APPEND", "A:" + endColumn
	case "upsert":
		return "UPSERT_MIXED", "A:" + endColumn
	case "overwrite_range":
		if estimatedRows == 0 {
			return "OVERWRITE", ""
		}
		return "OVERWRITE", fmt.Sprintf("A2:%s%d", endColumn, estimatedRows+1)
	default:
		return "", ""
	}
}

func mallWeatherFeishuColumnName(column int) string {
	result := make([]byte, 0, 3)
	for column > 0 {
		column--
		result = append(result, byte('A'+column%26))
		column /= 26
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}
