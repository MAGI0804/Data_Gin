package data_svc

import (
	"context"
	"errors"
	"sort"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/internal/requestbody"
)

var ErrMallWeatherFeishuHeaderConflict = errors.New("mall weather feishu headers: remote header conflicts with profile")

type mallWeatherFeishuHeaderSheets interface {
	ReadRange(context.Context, string, feishu.SheetRange) (*feishu.SheetValues, error)
	BatchUpdateValues(context.Context, string, []feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
}

type mallWeatherFeishuHeaderOutcome struct {
	DatasetKind string
	Action      string
}

// ensureMallWeatherFeishuHeaders must run while the caller owns the
// destination execution lock. Fixed Header writes are idempotent, and every
// acknowledged write is read back before dataset rows may be delivered.
func ensureMallWeatherFeishuHeaders(
	ctx context.Context,
	destination *MallWeatherFeishuResolvedDestination,
	profile MallWeatherExportProfileDTO,
	sheets mallWeatherFeishuHeaderSheets,
) ([]mallWeatherFeishuHeaderOutcome, error) {
	if err := validateMallWeatherFeishuHeaderInputs(ctx, destination, profile, sheets); err != nil {
		return nil, err
	}
	datasets := append([]requestbody.MallWeatherExportDataset(nil), profile.Datasets...)
	sort.Slice(datasets, func(left, right int) bool { return datasets[left].Kind < datasets[right].Kind })

	outcomes := make([]mallWeatherFeishuHeaderOutcome, 0, len(datasets))
	writes := make([]feishu.SheetWriteRange, 0, len(datasets))
	for _, dataset := range datasets {
		write, outcome, err := planMallWeatherFeishuHeaderWrite(ctx, destination, dataset, sheets)
		if err != nil {
			return nil, err
		}
		if write != nil {
			writes = append(writes, *write)
			outcomes = append(outcomes, outcome)
		}
	}
	if len(writes) == 0 {
		return outcomes, nil
	}
	acknowledgement, err := sheets.BatchUpdateValues(ctx, destination.SpreadsheetToken, writes)
	if err != nil {
		return nil, err
	}
	if acknowledgement == nil {
		return nil, errors.New("mall weather feishu headers: write acknowledgement is missing")
	}
	for _, write := range writes {
		readBack, err := sheets.ReadRange(ctx, destination.SpreadsheetToken, write.Range)
		if err != nil {
			return nil, err
		}
		if !mallWeatherFeishuHeaderMatchesWrite(readBack, write) {
			return nil, errors.New("mall weather feishu headers: write verification failed")
		}
	}
	return outcomes, nil
}

func validateMallWeatherFeishuHeaderInputs(
	ctx context.Context,
	destination *MallWeatherFeishuResolvedDestination,
	profile MallWeatherExportProfileDTO,
	sheets mallWeatherFeishuHeaderSheets,
) error {
	if ctx == nil || destination == nil || destination.DestinationID == 0 || destination.Code == "" ||
		destination.SpreadsheetToken == "" || sheets == nil || profile.ID == 0 || profile.Version == 0 ||
		!profile.Enabled || profile.Code == "" || profile.Code != destination.Config.ProfileCode ||
		len(profile.Datasets) == 0 || len(profile.Datasets) != len(destination.SheetIDs) ||
		len(profile.Datasets) != len(destination.Config.SheetIDEnvMapping) {
		return errors.New("mall weather feishu headers: invalid input")
	}
	seen := make(map[string]struct{}, len(profile.Datasets))
	for _, dataset := range profile.Datasets {
		if _, allowed := mallWeatherFeishuDatasetKinds[dataset.Kind]; !allowed || dataset.SplitBy != "" {
			return errors.New("mall weather feishu headers: invalid dataset")
		}
		if _, exists := seen[dataset.Kind]; exists {
			return errors.New("mall weather feishu headers: duplicate dataset")
		}
		if destination.SheetIDs[dataset.Kind] == "" || destination.Config.SheetIDEnvMapping[dataset.Kind] == "" {
			return errors.New("mall weather feishu headers: dataset mapping is incomplete")
		}
		seen[dataset.Kind] = struct{}{}
	}
	return nil
}

func planMallWeatherFeishuHeaderWrite(
	ctx context.Context,
	destination *MallWeatherFeishuResolvedDestination,
	dataset requestbody.MallWeatherExportDataset,
	sheets mallWeatherFeishuHeaderSheets,
) (*feishu.SheetWriteRange, mallWeatherFeishuHeaderOutcome, error) {
	columns, err := mallWeatherExportRenderColumns(dataset)
	if err != nil || len(columns) == 0 || len(columns) > maxMallWeatherFeishuColumns {
		return nil, mallWeatherFeishuHeaderOutcome{}, errors.New("mall weather feishu headers: invalid columns")
	}
	for _, column := range columns {
		if column.Title == "" || !validMallWeatherFeishuHeaderText(column.Title) {
			return nil, mallWeatherFeishuHeaderOutcome{}, errors.New("mall weather feishu headers: unsafe title")
		}
	}
	writeRange := feishu.SheetRange{
		SheetID: destination.SheetIDs[dataset.Kind], StartRow: 1, EndRow: 1,
		StartColumn: 1, EndColumn: int64(len(columns)),
	}
	current, err := sheets.ReadRange(ctx, destination.SpreadsheetToken, writeRange)
	if err != nil {
		return nil, mallWeatherFeishuHeaderOutcome{}, err
	}
	status, _, _, err := mallWeatherFeishuHeaderPlan(columns, current)
	if err != nil {
		return nil, mallWeatherFeishuHeaderOutcome{}, err
	}
	action := ""
	switch status {
	case "MATCHED":
		return nil, mallWeatherFeishuHeaderOutcome{}, nil
	case "EMPTY":
		action = "WRITE"
	case "MISMATCH":
		if !destination.Config.AllowHeaderRewrite {
			return nil, mallWeatherFeishuHeaderOutcome{}, ErrMallWeatherFeishuHeaderConflict
		}
		action = "REWRITE"
	default:
		return nil, mallWeatherFeishuHeaderOutcome{}, errors.New("mall weather feishu headers: invalid header status")
	}
	row := make([]feishu.SheetCell, len(columns))
	for index, column := range columns {
		row[index] = feishu.SheetCell{Type: feishu.SheetCellString, Text: column.Title}
	}
	return &feishu.SheetWriteRange{Range: writeRange, Rows: [][]feishu.SheetCell{row}},
		mallWeatherFeishuHeaderOutcome{DatasetKind: dataset.Kind, Action: action}, nil
}

func mallWeatherFeishuHeaderMatchesWrite(values *feishu.SheetValues, write feishu.SheetWriteRange) bool {
	if values == nil || len(write.Rows) != 1 || len(values.Rows) != 1 || len(values.Rows[0]) != len(write.Rows[0]) {
		return false
	}
	for index, expected := range write.Rows[0] {
		actual := values.Rows[0][index]
		if expected.Type != feishu.SheetCellString || actual.Type != feishu.SheetCellString || actual.Text != expected.Text {
			return false
		}
	}
	return true
}
