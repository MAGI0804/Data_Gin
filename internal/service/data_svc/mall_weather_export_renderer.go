package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"

	"github.com/xuri/excelize/v2"
)

const (
	mallWeatherExportStreamThreshold = int64(10_000)
	mallWeatherExportProgressRows    = int64(1_000)
	defaultMallWeatherExportPageSize = 1_000
)

type mallWeatherExportDataPager interface {
	Page(context.Context, data_dao.MallWeatherExportDataPageRequest) (*data_dao.MallWeatherExportDataPage, error)
}

type MallWeatherExportRenderRequest struct {
	ProfileCode   string
	Config        MallWeatherExportProfileConfig
	Filter        data_dao.MallWeatherExportEstimateFilter
	SnapshotAt    time.Time
	EstimatedRows int64
	OutputPath    string
}

type MallWeatherExportRenderProgress struct {
	ProcessedRows int64
	DatasetIndex  int
	SheetIndex    int
	RowsInSheet   int64
	CurrentSheet  string
	Cursor        json.RawMessage
}

type MallWeatherExportRenderResult struct {
	ProcessedRows int64
	SheetCount    int
	UsedStream    bool
}

type MallWeatherExportRenderer struct {
	pager     mallWeatherExportDataPager
	pageSize  int
	maxSheets int
}

type mallWeatherExportSheetState struct {
	name     string
	dataset  requestbody.MallWeatherExportDataset
	columns  []requestbody.MallWeatherExportColumn
	part     int
	dataRows int64
	nextRow  int
	stream   *excelize.StreamWriter
}

func NewMallWeatherExportRenderer() *MallWeatherExportRenderer {
	return &MallWeatherExportRenderer{
		pager:     data_dao.NewMallWeatherExportDataDAO(),
		pageSize:  defaultMallWeatherExportPageSize,
		maxSheets: maxMallWeatherExportWorkbookSheets,
	}
}

func newMallWeatherExportRenderer(
	pager mallWeatherExportDataPager,
	pageSize int,
	maxSheets int,
) (*MallWeatherExportRenderer, error) {
	if pager == nil || pageSize < 1 || pageSize > defaultMallWeatherExportPageSize ||
		maxSheets < 1 || maxSheets > maxMallWeatherExportWorkbookSheets {
		return nil, fmt.Errorf("mall weather export renderer: invalid configuration")
	}
	return &MallWeatherExportRenderer{
		pager:     pager,
		pageSize:  pageSize,
		maxSheets: maxSheets,
	}, nil
}

func (renderer *MallWeatherExportRenderer) Render(
	ctx context.Context,
	request MallWeatherExportRenderRequest,
	onProgress func(MallWeatherExportRenderProgress) error,
) (result MallWeatherExportRenderResult, returnErr error) {
	invalidConfig := request.Config.UnitSystem != "metric" && request.Config.UnitSystem != "imperial"
	invalidConfig = invalidConfig || request.Config.DateFormat == "" || request.Config.DateTimeFormat == ""
	if renderer == nil || renderer.pager == nil || ctx == nil || request.ProfileCode == "" || request.OutputPath == "" ||
		request.SnapshotAt.IsZero() || request.EstimatedRows < 0 || len(request.Config.Datasets) == 0 || invalidConfig {
		return result, fmt.Errorf("mall weather export renderer: invalid request")
	}
	location, err := time.LoadLocation(request.Config.TimeZone)
	if err != nil {
		return result, fmt.Errorf("mall weather export renderer: load time zone: %w", err)
	}
	if _, err := os.Lstat(request.OutputPath); err == nil {
		return result, fmt.Errorf("mall weather export renderer: output path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("mall weather export renderer: inspect output path: %w", err)
	}
	file := excelize.NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("mall weather export renderer: close workbook: %w", err))
		}
		if returnErr != nil {
			if err := os.Remove(request.OutputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("mall weather export renderer: remove partial workbook: %w", err))
			}
		}
	}()
	styles, err := newMallWeatherExportExcelStyles(file)
	if err != nil {
		return result, err
	}
	namer, err := newMallWeatherExportSheetNamer(renderer.maxSheets)
	if err != nil {
		return result, err
	}
	result.UsedStream = request.EstimatedRows >= mallWeatherExportStreamThreshold
	currentStates := make(map[string]*mallWeatherExportSheetState)
	allStates := make([]*mallWeatherExportSheetState, 0)
	firstSheet := true
	for datasetIndex, dataset := range request.Config.Datasets {
		datasetFilter, err := mallWeatherExportDatasetFilter(
			request.ProfileCode,
			dataset.Kind,
			request.Filter,
			request.SnapshotAt,
			request.Config.TimeZone,
		)
		if err != nil {
			return result, err
		}
		columns, err := mallWeatherExportRenderColumns(dataset)
		if err != nil {
			return result, err
		}
		fields := make([]string, len(columns))
		for index := range columns {
			fields[index] = columns[index].Field
		}
		var afterID uint
		wroteDataset := false
		for {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			pageRequest := data_dao.MallWeatherExportDataPageRequest{
				Kind:       dataset.Kind,
				Fields:     fields,
				Filter:     datasetFilter,
				AfterID:    afterID,
				Limit:      renderer.pageSize,
				SnapshotAt: request.SnapshotAt,
			}
			if dataset.Latest != nil {
				pageRequest.Latest = *dataset.Latest
			}
			if dataset.AsOf != "" {
				asOf, parseErr := time.Parse(time.RFC3339Nano, dataset.AsOf)
				if parseErr != nil {
					return result, fmt.Errorf("mall weather export renderer: invalid asOf")
				}
				pageRequest.AsOfUTC = &asOf
			}
			page, err := renderer.pager.Page(ctx, pageRequest)
			if err != nil {
				return result, fmt.Errorf("mall weather export renderer: read dataset page: %w", err)
			}
			nextAfterID, err := validateMallWeatherExportPage(page, afterID)
			if err != nil {
				return result, err
			}
			for _, row := range page.Rows {
				if err := ctx.Err(); err != nil {
					return result, err
				}
				splitValue, err := mallWeatherExportSplitValue(dataset.SplitBy, row, location)
				if err != nil {
					return result, err
				}
				stateKey := fmt.Sprintf("%d\x1f%s", datasetIndex, splitValue)
				state := currentStates[stateKey]
				maxDataRows := int64(dataset.MaxRows)
				if maxDataRows <= 0 || maxDataRows > mallWeatherExportExcelMaxRows-1 {
					maxDataRows = mallWeatherExportExcelMaxRows - 1
				}
				if state == nil || state.dataRows >= maxDataRows {
					part := 1
					if state != nil {
						part = state.part + 1
					}
					state, firstSheet, err = createMallWeatherExportSheet(
						file,
						namer,
						firstSheet,
						dataset,
						columns,
						splitValue,
						part,
						result.UsedStream,
						styles,
					)
					if err != nil {
						return result, err
					}
					currentStates[stateKey] = state
					allStates = append(allStates, state)
					result.SheetCount++
				}
				cells := make([]mallWeatherExportExcelCell, len(columns))
				for index, column := range columns {
					cells[index], err = mallWeatherExportExcelValue(
						column.Field,
						column.Format,
						request.Config.UnitSystem,
						location,
						request.Config.DateFormat,
						request.Config.DateTimeFormat,
						row.Values[column.Field],
						styles,
					)
					if err != nil {
						return result, err
					}
				}
				if err := writeMallWeatherExportRow(file, state, cells); err != nil {
					return result, err
				}
				state.dataRows++
				state.nextRow++
				result.ProcessedRows++
				wroteDataset = true
				if result.ProcessedRows%mallWeatherExportProgressRows == 0 && onProgress != nil {
					cursor, err := json.Marshal(map[string]uint{"afterId": row.CursorID})
					if err != nil {
						return result, fmt.Errorf("mall weather export renderer: encode cursor: %w", err)
					}
					if err := onProgress(MallWeatherExportRenderProgress{
						ProcessedRows: result.ProcessedRows, DatasetIndex: datasetIndex, SheetIndex: state.part - 1,
						RowsInSheet: state.dataRows, CurrentSheet: state.name, Cursor: cursor,
					}); err != nil {
						return result, fmt.Errorf("mall weather export renderer: update progress: %w", err)
					}
				}
			}
			if len(page.Rows) > 0 {
				afterID = nextAfterID
			}
			if !page.HasMore {
				break
			}
		}
		if !wroteDataset {
			state, nextFirst, err := createMallWeatherExportSheet(
				file,
				namer,
				firstSheet,
				dataset,
				columns,
				"",
				1,
				result.UsedStream,
				styles,
			)
			if err != nil {
				return result, err
			}
			firstSheet = nextFirst
			currentStates[fmt.Sprintf("%d\x1f", datasetIndex)] = state
			allStates = append(allStates, state)
			result.SheetCount++
		}
	}
	for _, state := range allStates {
		if err := finalizeMallWeatherExportSheet(file, state); err != nil {
			return result, err
		}
		if state.stream != nil {
			if err := state.stream.Flush(); err != nil {
				return result, fmt.Errorf("mall weather export renderer: flush sheet: %w", err)
			}
		}
	}
	if err := file.SaveAs(request.OutputPath); err != nil {
		return result, fmt.Errorf("mall weather export renderer: save workbook: %w", err)
	}
	return result, nil
}

func validateMallWeatherExportPage(page *data_dao.MallWeatherExportDataPage, afterID uint) (uint, error) {
	if page == nil {
		return 0, fmt.Errorf("mall weather export renderer: nil dataset page")
	}
	if len(page.Rows) == 0 {
		if page.HasMore {
			return 0, fmt.Errorf("mall weather export renderer: empty page has continuation")
		}
		return afterID, nil
	}
	cursor := afterID
	for _, row := range page.Rows {
		if row.CursorID <= cursor {
			return 0, fmt.Errorf("mall weather export renderer: cursor did not advance")
		}
		cursor = row.CursorID
	}
	if page.NextAfterID != cursor {
		return 0, fmt.Errorf("mall weather export renderer: next cursor does not match page")
	}
	return cursor, nil
}

func mallWeatherExportRenderColumns(dataset requestbody.MallWeatherExportDataset) ([]requestbody.MallWeatherExportColumn, error) {
	if len(dataset.Columns) > 0 {
		return append([]requestbody.MallWeatherExportColumn(nil), dataset.Columns...), nil
	}
	fields, ok := data_dao.MallWeatherExportDefaultFields(dataset.Kind)
	if !ok {
		return nil, fmt.Errorf("mall weather export renderer: unsupported dataset")
	}
	columns := make([]requestbody.MallWeatherExportColumn, len(fields))
	for index, field := range fields {
		format := "general"
		if strings.Contains(field, "_at") || strings.Contains(field, "_time") || field == "forecast_minute" {
			format = "datetime"
		} else if strings.Contains(field, "_date") {
			format = "date"
		}
		columns[index] = requestbody.MallWeatherExportColumn{
			Field:  field,
			Title:  field,
			Width:  18,
			Format: format,
		}
	}
	return columns, nil
}

func mallWeatherExportSplitValue(
	splitBy string,
	row data_dao.MallWeatherExportDataRow,
	location *time.Location,
) (string, error) {
	switch splitBy {
	case "":
		return "", nil
	case "city":
		return row.SplitCity, nil
	case "mall":
		return row.SplitMall, nil
	case "data_type":
		return row.SplitDataType, nil
	case "date":
		parsed, err := parseMallWeatherExportTime(row.SplitDate)
		if err != nil {
			return "", err
		}
		return parsed.In(location).Format(time.DateOnly), nil
	default:
		return "", fmt.Errorf("mall weather export renderer: invalid split strategy")
	}
}

func createMallWeatherExportSheet(
	file *excelize.File,
	namer *mallWeatherExportSheetNamer,
	firstSheet bool,
	dataset requestbody.MallWeatherExportDataset,
	columns []requestbody.MallWeatherExportColumn,
	splitValue string,
	part int,
	useStream bool,
	styles mallWeatherExportExcelStyles,
) (*mallWeatherExportSheetState, bool, error) {
	name, err := namer.Name(dataset.SheetName, splitValue, part)
	if err != nil {
		return nil, firstSheet, err
	}
	if firstSheet {
		if err := file.SetSheetName("Sheet1", name); err != nil {
			return nil, firstSheet, fmt.Errorf("mall weather export renderer: rename first sheet: %w", err)
		}
		firstSheet = false
	} else if _, err := file.NewSheet(name); err != nil {
		return nil, firstSheet, fmt.Errorf("mall weather export renderer: create sheet: %w", err)
	}
	state := &mallWeatherExportSheetState{
		name:    name,
		dataset: dataset,
		columns: columns,
		part:    part,
		nextRow: 2,
	}
	header := make([]mallWeatherExportExcelCell, len(columns))
	for index, column := range columns {
		header[index] = mallWeatherExportExcelCell{StyleID: styles.Header, Value: column.Title}
	}
	if useStream {
		state.stream, err = file.NewStreamWriter(name)
		if err != nil {
			return nil, firstSheet, fmt.Errorf("mall weather export renderer: create stream writer: %w", err)
		}
		for index, column := range columns {
			width := mallWeatherExportColumnWidth(column)
			if err = state.stream.SetColWidth(index+1, index+1, width); err != nil {
				return nil, firstSheet, fmt.Errorf("mall weather export renderer: set stream column width: %w", err)
			}
		}
		if dataset.FreezeHeader {
			if err := state.stream.SetPanes(mallWeatherExportHeaderPanes()); err != nil {
				return nil, firstSheet, fmt.Errorf("mall weather export renderer: freeze stream header: %w", err)
			}
		}
		err = state.stream.SetRow("A1", mallWeatherExportStreamRow(header))
	} else {
		err = setMallWeatherExportRegularRow(file, name, 1, header)
	}
	if err != nil {
		return nil, firstSheet, fmt.Errorf("mall weather export renderer: write sheet header: %w", err)
	}
	return state, firstSheet, nil
}

func writeMallWeatherExportRow(
	file *excelize.File,
	state *mallWeatherExportSheetState,
	cells []mallWeatherExportExcelCell,
) error {
	if state.stream != nil {
		coordinate, err := excelize.CoordinatesToCellName(1, state.nextRow)
		if err != nil {
			return fmt.Errorf("mall weather export renderer: row coordinate: %w", err)
		}
		if err := state.stream.SetRow(coordinate, mallWeatherExportStreamRow(cells)); err != nil {
			return fmt.Errorf("mall weather export renderer: write stream row: %w", err)
		}
		return nil
	}
	return setMallWeatherExportRegularRow(file, state.name, state.nextRow, cells)
}

func finalizeMallWeatherExportSheet(file *excelize.File, state *mallWeatherExportSheetState) error {
	lastColumn, err := excelize.ColumnNumberToName(len(state.columns))
	if err != nil {
		return fmt.Errorf("mall weather export renderer: last column name: %w", err)
	}
	if state.stream == nil {
		for index, column := range state.columns {
			name, err := excelize.ColumnNumberToName(index + 1)
			if err != nil {
				return fmt.Errorf("mall weather export renderer: column name: %w", err)
			}
			if err := file.SetColWidth(state.name, name, name, mallWeatherExportColumnWidth(column)); err != nil {
				return fmt.Errorf("mall weather export renderer: set column width: %w", err)
			}
		}
		if state.dataset.FreezeHeader {
			if err := file.SetPanes(state.name, mallWeatherExportHeaderPanes()); err != nil {
				return fmt.Errorf("mall weather export renderer: freeze header: %w", err)
			}
		}
	}
	lastRow := state.nextRow - 1
	if state.dataset.AutoFilter {
		if err := file.AutoFilter(state.name, fmt.Sprintf("A1:%s%d", lastColumn, lastRow), nil); err != nil {
			return fmt.Errorf("mall weather export renderer: set auto filter: %w", err)
		}
	}
	fieldColumns := make(map[string]int, len(state.columns))
	for index, column := range state.columns {
		fieldColumns[column.Field] = index + 1
	}
	criteria := map[string]string{
		"equal": "==", "not_equal": "!=", "less_than": "<", "less_than_or_equal": "<=",
		"greater_than": ">", "greater_than_or_equal": ">=", "between": "between", "not_between": "not between",
	}
	for _, rule := range state.dataset.ConditionalFormats {
		columnNumber := fieldColumns[rule.Field]
		if lastRow < 2 {
			continue
		}
		criterion, validOperator := criteria[rule.Operator]
		if columnNumber == 0 || rule.Value == nil || !validOperator ||
			(rule.FontColor == "" && rule.BackgroundColor == "") ||
			math.IsNaN(*rule.Value) || math.IsInf(*rule.Value, 0) {
			return fmt.Errorf("mall weather export renderer: invalid conditional format")
		}
		style := &excelize.Style{}
		if rule.FontColor != "" {
			style.Font = &excelize.Font{Color: rule.FontColor}
		}
		if rule.BackgroundColor != "" {
			style.Fill = excelize.Fill{Type: "pattern", Color: []string{rule.BackgroundColor}, Pattern: 1}
		}
		styleID, err := file.NewConditionalStyle(style)
		if err != nil {
			return fmt.Errorf("mall weather export renderer: create conditional style: %w", err)
		}
		columnName, err := excelize.ColumnNumberToName(columnNumber)
		if err != nil {
			return fmt.Errorf("mall weather export renderer: conditional format column: %w", err)
		}
		option := excelize.ConditionalFormatOptions{Type: "cell", Criteria: criterion, Format: &styleID}
		if rule.Operator == "between" || rule.Operator == "not_between" {
			if rule.SecondValue == nil || math.IsNaN(*rule.SecondValue) || math.IsInf(*rule.SecondValue, 0) {
				return fmt.Errorf("mall weather export renderer: invalid conditional format range")
			}
			option.MinValue = fmt.Sprint(*rule.Value)
			option.MaxValue = fmt.Sprint(*rule.SecondValue)
		} else {
			option.Value = fmt.Sprint(*rule.Value)
		}
		if err := file.SetConditionalFormat(
			state.name,
			fmt.Sprintf("%s2:%s%d", columnName, columnName, lastRow),
			[]excelize.ConditionalFormatOptions{option},
		); err != nil {
			return fmt.Errorf("mall weather export renderer: set conditional format: %w", err)
		}
	}
	return nil
}

func mallWeatherExportColumnWidth(column requestbody.MallWeatherExportColumn) float64 {
	if column.Width > 0 {
		return column.Width
	}
	return 18
}

func mallWeatherExportHeaderPanes() *excelize.Panes {
	return &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}
}
