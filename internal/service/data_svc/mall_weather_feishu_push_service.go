package data_svc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	projectredis "gin-biz-web-api/pkg/redis"

	"gorm.io/gorm"
)

var (
	ErrMallWeatherFeishuInvalid             = errors.New("mall weather feishu push: invalid input")
	ErrMallWeatherFeishuDestinationNotFound = errors.New("mall weather feishu push: destination not found")
)

type mallWeatherFeishuDestinationReader interface {
	FindByID(context.Context, uint) (*model.DestinationDefinition, error)
}

type mallWeatherFeishuSheetsReader interface {
	Inspect(context.Context, string, []string) (*feishu.SpreadsheetMetadata, error)
	ReadRange(context.Context, string, feishu.SheetRange) (*feishu.SheetValues, error)
}

type mallWeatherFeishuSheetsFactory func() (mallWeatherFeishuSheetsReader, error)

type mallWeatherFeishuPushDependencies struct {
	destinations mallWeatherFeishuDestinationReader
	profiles     mallWeatherExportProfileReader
	permissions  mallPermissionChecker
	estimator    mallWeatherExportEstimator
	limits       mallWeatherExportLimitReader
	resources    mallWeatherFeishuResourceResolver
	newSheets    mallWeatherFeishuSheetsFactory
	now          func() time.Time
}

type MallWeatherFeishuPushService struct {
	destinations mallWeatherFeishuDestinationReader
	profiles     mallWeatherExportProfileReader
	permissions  mallPermissionChecker
	estimator    mallWeatherExportEstimator
	limits       mallWeatherExportLimitReader
	resources    mallWeatherFeishuResourceResolver
	newSheets    mallWeatherFeishuSheetsFactory
	now          func() time.Time
}

func NewMallWeatherFeishuPushService() *MallWeatherFeishuPushService {
	var once sync.Once
	var sheets mallWeatherFeishuSheetsReader
	var sheetsErr error
	newSheets := func() (mallWeatherFeishuSheetsReader, error) {
		once.Do(func() {
			sheets, sheetsErr = newRuntimeMallWeatherFeishuSheets()
		})
		return sheets, sheetsErr
	}
	service, err := newMallWeatherFeishuPushService(mallWeatherFeishuPushDependencies{
		destinations: data_dao.NewDestinationDefinitionDAO(),
		profiles:     data_dao.NewMallWeatherExportProfileDAO(database.DB),
		permissions:  data_dao.NewMallWeatherPermissionDAO(database.DB),
		estimator:    data_dao.NewMallWeatherExportJobDAO(database.DB),
		limits:       data_dao.NewRuntimeConfigDAO(),
		resources:    global.Credentials,
		newSheets:    newSheets,
		now:          time.Now,
	})
	if err != nil {
		panic(err)
	}
	return service
}

func newMallWeatherFeishuPushService(
	dependencies mallWeatherFeishuPushDependencies,
) (*MallWeatherFeishuPushService, error) {
	if dependencies.destinations == nil || dependencies.profiles == nil || dependencies.permissions == nil ||
		dependencies.estimator == nil || dependencies.limits == nil || dependencies.resources == nil ||
		dependencies.newSheets == nil || dependencies.now == nil {
		return nil, errors.New("mall weather feishu push: invalid service configuration")
	}
	return &MallWeatherFeishuPushService{
		destinations: dependencies.destinations,
		profiles:     dependencies.profiles,
		permissions:  dependencies.permissions,
		estimator:    dependencies.estimator,
		limits:       dependencies.limits,
		resources:    dependencies.resources,
		newSheets:    dependencies.newSheets,
		now:          dependencies.now,
	}, nil
}

func (service *MallWeatherFeishuPushService) DryRun(
	ctx context.Context,
	actorUserID uint,
	request requestbody.MallWeatherFeishuPushRequest,
) (*MallWeatherFeishuDryRunResult, error) {
	if service == nil || ctx == nil || actorUserID == 0 || request.DestinationID == 0 || request.ProfileID == 0 ||
		(request.ExpectedProfileVersion != nil && *request.ExpectedProfileVersion == 0) {
		return nil, ErrMallWeatherFeishuInvalid
	}
	allowed, err := service.permissions.HasPermission(
		ctx,
		actorUserID,
		PermissionWeatherFeishuPush,
		service.now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu push: authorize: %w", err)
	}
	if !allowed {
		return nil, ErrMallForbidden
	}
	destination, err := service.destinations.FindByID(ctx, request.DestinationID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMallWeatherFeishuDestinationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu push: load destination: %w", err)
	}
	resolved, err := resolveMallWeatherFeishuDestination(destination, service.resources)
	if err != nil {
		return nil, fmt.Errorf("%w: destination configuration", ErrMallWeatherFeishuInvalid)
	}
	profile, err := service.profiles.FindByID(ctx, request.ProfileID)
	if err != nil {
		return nil, err
	}
	if profile == nil || !profile.Enabled ||
		(request.ExpectedProfileVersion != nil && *request.ExpectedProfileVersion != profile.Version) {
		return nil, ErrMallWeatherExportProfileConflict
	}
	profileDTO, err := mallWeatherExportProfileDTO(profile)
	if err != nil {
		return nil, err
	}
	filters := profileDTO.Filters
	if request.Filters != nil {
		filters, err = normalizeMallWeatherExportFilters(*request.Filters)
		if err != nil {
			return nil, ErrMallWeatherFeishuInvalid
		}
	}
	limits, err := loadMallWeatherExportLimits(ctx, service.limits)
	if err != nil {
		return nil, err
	}
	if err := validateMallWeatherExportJobRange(profileDTO.Datasets, filters, profileDTO.TimeZone, limits); err != nil {
		return nil, ErrMallWeatherFeishuInvalid
	}
	estimatedRows, err := service.estimateDryRunRows(ctx, profileDTO, filters, limits)
	if err != nil {
		return nil, err
	}
	sheets, err := service.newSheets()
	if err != nil || sheets == nil {
		return nil, errors.New("mall weather feishu push: sheets client is unavailable")
	}
	return service.inspectAndPlan(ctx, resolved, profileDTO, estimatedRows, sheets)
}

func (service *MallWeatherFeishuPushService) estimateDryRunRows(
	ctx context.Context,
	profile MallWeatherExportProfileDTO,
	filters requestbody.MallWeatherExportFilters,
	limits mallWeatherExportLimits,
) (map[string]int64, error) {
	estimateCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(limits.EstimateTimeoutSeconds)*time.Second,
	)
	defer cancel()
	result := make(map[string]int64, len(profile.Datasets))
	var total int64
	for _, dataset := range profile.Datasets {
		single := profile
		single.Datasets = []requestbody.MallWeatherExportDataset{dataset}
		remaining := limits.MaxEstimatedRows - total
		stopAfter := remaining
		if stopAfter < 1 {
			stopAfter = 1
		}
		estimateRequest, err := mallWeatherExportEstimateRequest(single, filters, stopAfter)
		if err != nil {
			return nil, err
		}
		rows, err := service.estimator.EstimateRows(estimateCtx, estimateRequest)
		if err != nil {
			return nil, fmt.Errorf("mall weather feishu push: estimate %s: %w", dataset.Kind, err)
		}
		if rows < 0 || rows > remaining {
			return nil, ErrMallWeatherExportTooLarge
		}
		result[dataset.Kind] = rows
		total += rows
	}
	return result, nil
}

func (service *MallWeatherFeishuPushService) inspectAndPlan(
	ctx context.Context,
	destination *MallWeatherFeishuResolvedDestination,
	profile MallWeatherExportProfileDTO,
	estimatedRows map[string]int64,
	sheets mallWeatherFeishuSheetsReader,
) (*MallWeatherFeishuDryRunResult, error) {
	inspectCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(destination.Config.TimeoutSeconds)*time.Second,
	)
	defer cancel()
	requiredSheetIDs := make([]string, 0, len(destination.SheetIDs))
	for _, sheetID := range destination.SheetIDs {
		requiredSheetIDs = append(requiredSheetIDs, sheetID)
	}
	metadata, err := sheets.Inspect(inspectCtx, destination.SpreadsheetToken, requiredSheetIDs)
	if err != nil {
		return nil, err
	}
	metadataByID := make(map[string]feishu.SheetMetadata, len(metadata.Sheets))
	for _, sheet := range metadata.Sheets {
		metadataByID[sheet.SheetID] = sheet
	}
	headers := make(map[string]*feishu.SheetValues, len(profile.Datasets))
	for _, dataset := range profile.Datasets {
		columns, err := mallWeatherExportRenderColumns(dataset)
		if err != nil || len(columns) == 0 {
			return nil, ErrMallWeatherFeishuInvalid
		}
		sheetID, exists := destination.SheetIDs[dataset.Kind]
		metadata, hasMetadata := metadataByID[sheetID]
		if !exists || !hasMetadata {
			return nil, ErrMallWeatherFeishuInvalid
		}
		endColumn := int64(len(columns))
		if metadata.GridProperties.ColumnCount < endColumn {
			endColumn = metadata.GridProperties.ColumnCount
		}
		headers[dataset.Kind], err = sheets.ReadRange(inspectCtx, destination.SpreadsheetToken, feishu.SheetRange{
			SheetID: sheetID, StartRow: 1, EndRow: 1, StartColumn: 1, EndColumn: endColumn,
		})
		if err != nil {
			return nil, err
		}
	}
	return buildMallWeatherFeishuDryRunPlan(mallWeatherFeishuDryRunPlanInput{
		Destination: destination, Profile: profile, Metadata: metadata, Headers: headers, EstimatedRows: estimatedRows,
	})
}

func newRuntimeMallWeatherFeishuSheets() (mallWeatherFeishuSheetsReader, error) {
	redisInstance := projectredis.Instance()
	if redisInstance == nil || redisInstance.Client == nil {
		return nil, errors.New("mall weather feishu push: redis is unavailable")
	}
	tokens, err := feishu.NewTenantTokenProvider(
		global.Credentials.FeishuAppID(),
		global.Credentials.FeishuAppSecret(),
		nil,
		redisInstance.Client,
	)
	if err != nil {
		return nil, err
	}
	return feishu.NewSheetsClient(tokens, nil)
}
