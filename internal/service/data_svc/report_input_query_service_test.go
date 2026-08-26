package data_svc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	appConfig "gin-biz-web-api/config"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

func TestReportInputQueryServiceUsesPublishedBindingAndCachesOracle(t *testing.T) {
	store := &fakeReportInputQueryStore{published: &reportrepo.PublishedReport{Version: model.ReportVersion{
		InputSchemaJSON: model.JSONText(`{"store":{"type":"str","displayName":"门店","control":"SELECT","queryName":"stores"}}`),
	}}}
	connection := &fakeReportInputOracle{options: []reportoracle.InputOption{{ID: "S001", Name: "上海店"}}}
	openCalls := 0
	service := NewReportInputQueryServiceWithDependencies(store, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		openCalls++
		return connection, nil
	})
	for range 2 {
		result, err := service.Options(t.Context(), 17, 9, "store", "上海店")
		if err != nil || len(result.Items) != 1 || result.Items[0].ID != "S001" {
			t.Fatalf("Options() = %#v, %v", result, err)
		}
	}
	if openCalls != 1 || connection.queryName != "" || connection.limit != reportInputOptionLimit || connection.statement != "SELECT id, name FROM stores" {
		t.Fatalf("openCalls=%d connection=%#v", openCalls, connection)
	}
}

func TestReportInputQueryServiceSupportsListBindings(t *testing.T) {
	for _, valueType := range []string{"list[str]", "list[number]"} {
		t.Run(valueType, func(t *testing.T) {
			store := &fakeReportInputQueryStore{published: &reportrepo.PublishedReport{Version: model.ReportVersion{
				InputSchemaJSON: model.JSONText(fmt.Sprintf(`{"store":{"type":%q,"displayName":"门店","control":"SELECT","queryName":"stores"}}`, valueType)),
			}}}
			connection := &fakeReportInputOracle{options: []reportoracle.InputOption{{ID: "S001", Name: "上海店"}}}
			service := NewReportInputQueryServiceWithDependencies(store, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
				return connection, nil
			})

			result, err := service.Options(t.Context(), 17, 9, "store", "上海店")
			if err != nil || len(result.Items) != 1 || result.Items[0].ID != "S001" || result.Items[0].Name != "上海店" {
				t.Fatalf("Options() = %#v, %v", result, err)
			}
			if connection.queryName != "" || connection.limit != reportInputOptionLimit || connection.statement != "SELECT id, name FROM stores" {
				t.Fatalf("connection = %#v", connection)
			}
		})
	}
}

func TestReportInputQueryServiceEnforcesPublishedReportAccess(t *testing.T) {
	store := &fakeReportInputQueryStore{err: reportrepo.ErrReportActionDenied}
	service := NewReportInputQueryServiceWithDependencies(store, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		return nil, errors.New("must not open")
	})
	if _, err := service.Options(t.Context(), 17, 9, "store", ""); !errors.Is(err, ErrReportRunDenied) {
		t.Fatalf("Options() error = %v", err)
	}
}

func TestReportInputQueryServicePrefersDatabaseDefinition(t *testing.T) {
	store := &fakeReportInputQueryStore{published: &reportrepo.PublishedReport{Version: model.ReportVersion{
		InputSchemaJSON: model.JSONText(`{"store":{"type":"str","displayName":"门店","control":"SELECT","queryName":"门店查询-2026_华东"}}`),
	}}}
	definitions := &fakeReportInputQueryDefinitionStore{definitions: []model.ReportInputQueryDefinition{{Name: "门店查询-2026_华东", SelectSQL: "SELECT id, name FROM db_stores", Enabled: true}}}
	connection := &fakeReportInputOracle{}
	service := NewReportInputQueryServiceWithStores(store, definitions, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		return connection, nil
	})
	if _, err := service.Options(t.Context(), 17, 9, "store", ""); err != nil {
		t.Fatalf("Options() error = %v", err)
	}
	if connection.statement != "SELECT id, name FROM db_stores" {
		t.Fatalf("statement = %q", connection.statement)
	}
}

func TestReportInputQueryServiceDisabledDatabaseDefinitionSuppressesLegacyFallback(t *testing.T) {
	store := &fakeReportInputQueryStore{published: &reportrepo.PublishedReport{Version: model.ReportVersion{
		InputSchemaJSON: model.JSONText(`{"store":{"type":"str","displayName":"门店","control":"SELECT","queryName":"stores"}}`),
	}}}
	definitions := &fakeReportInputQueryDefinitionStore{definitions: []model.ReportInputQueryDefinition{{Name: "stores", Enabled: false}}}
	service := NewReportInputQueryServiceWithStores(store, definitions, reportInputQueryConfig(), func(context.Context, reportoracle.Config) (reportInputOracleConnection, error) {
		return nil, errors.New("must not open")
	})
	if _, err := service.Options(t.Context(), 17, 9, "store", ""); !errors.Is(err, ErrReportInputQueryUnavailable) {
		t.Fatalf("Options() error = %v", err)
	}
	list, err := service.List(t.Context(), 17)
	if err != nil || len(list.Items) != 0 {
		t.Fatalf("List() = %#v, %v", list, err)
	}
}

func reportInputQueryConfig() appConfig.ReportInputQueryConfig {
	return appConfig.ReportInputQueryConfig{
		Oracle:  appConfig.ReportInputOracleConfig{Host: "oracle", Port: 1521, ServiceName: "REPORT", Username: "user", Password: "secret", QueryTimeout: time.Second},
		Queries: map[string]appConfig.ReportInputQuery{"stores": {Name: "stores", Select: "SELECT id, name FROM stores"}},
	}
}

type fakeReportInputQueryStore struct {
	published *reportrepo.PublishedReport
	err       error
}

type fakeReportInputQueryDefinitionStore struct {
	definitions []model.ReportInputQueryDefinition
	created     *model.ReportInputQueryDefinition
	updated     *model.ReportInputQueryDefinition
	deletedID   uint
	testStatus  string
}

func (store *fakeReportInputQueryDefinitionStore) ListReportInputQueryDefinitions(context.Context) ([]model.ReportInputQueryDefinition, error) {
	return store.definitions, nil
}

func (store *fakeReportInputQueryDefinitionStore) GetReportInputQueryDefinition(_ context.Context, id uint) (*model.ReportInputQueryDefinition, error) {
	for _, definition := range store.definitions {
		if definition.ID == id {
			copy := definition
			return &copy, nil
		}
	}
	return nil, reportrepo.ErrInputQueryNotFound
}

func (store *fakeReportInputQueryDefinitionStore) FindReportInputQueryByName(_ context.Context, name string) (*model.ReportInputQueryDefinition, error) {
	for _, definition := range store.definitions {
		if definition.Name == name {
			copy := definition
			return &copy, nil
		}
	}
	return nil, reportrepo.ErrInputQueryNotFound
}

func (store *fakeReportInputQueryDefinitionStore) CreateReportInputQueryDefinition(_ context.Context, _ uint, definition *model.ReportInputQueryDefinition) error {
	definition.ID = 7
	definition.LockVersion = 1
	store.created = definition
	store.definitions = append(store.definitions, *definition)
	return nil
}

func (store *fakeReportInputQueryDefinitionStore) UpdateReportInputQueryDefinition(_ context.Context, _ uint, definition *model.ReportInputQueryDefinition, expected uint64) error {
	definition.LockVersion = expected + 1
	store.updated = definition
	for index := range store.definitions {
		if store.definitions[index].ID == definition.ID {
			store.definitions[index] = *definition
		}
	}
	return nil
}

func (store *fakeReportInputQueryDefinitionStore) DeleteReportInputQueryDefinition(_ context.Context, _ uint, definitionID uint, _ uint64) error {
	store.deletedID = definitionID
	return nil
}

func (store *fakeReportInputQueryDefinitionStore) RecordReportInputQueryTest(_ context.Context, _ uint, _ uint, status, _ string, _ time.Time) error {
	store.testStatus = status
	return nil
}

func (store *fakeReportInputQueryStore) FindPublishedReport(context.Context, uint, uint, string) (*reportrepo.PublishedReport, error) {
	return store.published, store.err
}

type fakeReportInputOracle struct {
	options   []reportoracle.InputOption
	statement string
	queryName string
	limit     int
}

func (oracle *fakeReportInputOracle) QueryInputOptions(_ context.Context, statement, name string, limit int) ([]reportoracle.InputOption, error) {
	oracle.statement, oracle.queryName, oracle.limit = statement, name, limit
	return oracle.options, nil
}

func (*fakeReportInputOracle) Close() error { return nil }
