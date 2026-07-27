package bootstrap

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

type mallWeatherVersionIndexSpec struct {
	Model     interface{}
	TableName string
	IndexName string
	Columns   []string
}

type mallWeatherMySQLIndexColumn struct {
	IndexName  string `gorm:"column:index_name"`
	ColumnName string `gorm:"column:column_name"`
	Sequence   int    `gorm:"column:seq_in_index"`
}

var mallWeatherIndexNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func mallWeatherVersionIndexSpecs() []mallWeatherVersionIndexSpec {
	return []mallWeatherVersionIndexSpec{
		{
			Model: &model.MallWeatherDaily{}, TableName: "mall_weather_daily", IndexName: "uk_daily_version",
			Columns: []string{"mall_id", "provider", "forecast_date_local", "issued_at_utc"},
		},
		{
			Model: &model.MallWeatherLifeIndex{}, TableName: "mall_weather_life_indices", IndexName: "uk_life_version",
			Columns: []string{"mall_id", "provider", "source_api", "forecast_date_local", "index_type", "issued_at_utc"},
		},
	}
}

func prepareMallWeatherVersionIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("mall weather index repair: database is unavailable")
	}
	for _, spec := range mallWeatherVersionIndexSpecs() {
		if !db.Migrator().HasTable(spec.Model) {
			continue
		}
		indexes, err := loadMallWeatherUniqueIndexes(db, spec.TableName)
		if err != nil {
			return err
		}
		canonicalColumns, canonicalExists := mallWeatherVersionIndexColumns(indexes, spec.IndexName)
		if !mallWeatherVersionIndexColumnsMatch(canonicalColumns, spec.Columns) {
			statement, err := replaceMallWeatherVersionIndexSQL(spec, canonicalExists)
			if err != nil {
				return err
			}
			execErr := db.Exec(statement).Error
			indexes, err = loadMallWeatherUniqueIndexes(db, spec.TableName)
			if err != nil {
				return err
			}
			canonicalColumns, _ = mallWeatherVersionIndexColumns(indexes, spec.IndexName)
			if !mallWeatherVersionIndexColumnsMatch(canonicalColumns, spec.Columns) {
				if execErr != nil {
					return fmt.Errorf("mall weather index repair: replace %s: %w", spec.IndexName, execErr)
				}
				return fmt.Errorf("mall weather index repair: %s replacement was not applied", spec.IndexName)
			}
		}
		for _, indexName := range restrictiveMallWeatherVersionIndexes(indexes, spec) {
			if !mallWeatherIndexNamePattern.MatchString(indexName) {
				return fmt.Errorf("mall weather index repair: unsafe index name")
			}
			if dropErr := db.Migrator().DropIndex(spec.Model, indexName); dropErr != nil {
				current, err := loadMallWeatherUniqueIndexes(db, spec.TableName)
				if err != nil {
					return err
				}
				if _, exists := mallWeatherVersionIndexColumns(current, indexName); exists {
					return fmt.Errorf("mall weather index repair: drop %s: %w", indexName, dropErr)
				}
			}
		}
	}
	return nil
}

func replaceMallWeatherVersionIndexSQL(spec mallWeatherVersionIndexSpec, existing bool) (string, error) {
	if !mallWeatherIndexNamePattern.MatchString(spec.TableName) || !mallWeatherIndexNamePattern.MatchString(spec.IndexName) || len(spec.Columns) == 0 {
		return "", fmt.Errorf("mall weather index repair: unsafe version index specification")
	}
	quotedColumns := make([]string, len(spec.Columns))
	for index, column := range spec.Columns {
		if !mallWeatherIndexNamePattern.MatchString(column) {
			return "", fmt.Errorf("mall weather index repair: unsafe version index column")
		}
		quotedColumns[index] = "`" + column + "`"
	}
	operations := make([]string, 0, 2)
	if existing {
		operations = append(operations, "DROP INDEX `"+spec.IndexName+"`")
	}
	operations = append(operations, "ADD UNIQUE INDEX `"+spec.IndexName+"` ("+strings.Join(quotedColumns, ", ")+")")
	return "ALTER TABLE `" + spec.TableName + "` " + strings.Join(operations, ", "), nil
}

func verifyMallWeatherVersionIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("mall weather index verification: database is unavailable")
	}
	for _, spec := range mallWeatherVersionIndexSpecs() {
		indexes, err := loadMallWeatherUniqueIndexes(db, spec.TableName)
		if err != nil {
			return err
		}
		columns, _ := mallWeatherVersionIndexColumns(indexes, spec.IndexName)
		if !mallWeatherVersionIndexColumnsMatch(columns, spec.Columns) {
			return fmt.Errorf(
				"mall weather index verification: %s has columns [%s], want [%s]",
				spec.IndexName, strings.Join(columns, ","), strings.Join(spec.Columns, ","),
			)
		}
		if restrictive := restrictiveMallWeatherVersionIndexes(indexes, spec); len(restrictive) > 0 {
			return fmt.Errorf("mall weather index verification: restrictive indexes remain [%s]", strings.Join(restrictive, ","))
		}
	}
	return nil
}

func loadMallWeatherUniqueIndexes(db *gorm.DB, tableName string) (map[string][]string, error) {
	var rows []mallWeatherMySQLIndexColumn
	err := db.Raw(
		`SELECT INDEX_NAME AS index_name, COLUMN_NAME AS column_name, SEQ_IN_INDEX AS seq_in_index
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND NON_UNIQUE = 0
ORDER BY INDEX_NAME ASC, SEQ_IN_INDEX ASC`,
		tableName,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mall weather index inspection: load %s: %w", tableName, err)
	}
	indexes := make(map[string][]string)
	for index := range rows {
		name := strings.TrimSpace(rows[index].IndexName)
		if name == "" {
			return nil, fmt.Errorf("mall weather index inspection: empty index name")
		}
		indexes[name] = append(indexes[name], strings.ToLower(strings.TrimSpace(rows[index].ColumnName)))
	}
	return indexes, nil
}

func mallWeatherVersionIndexColumns(indexes map[string][]string, name string) ([]string, bool) {
	for indexName, columns := range indexes {
		if strings.EqualFold(indexName, name) {
			return columns, true
		}
	}
	return nil, false
}

func restrictiveMallWeatherVersionIndexes(indexes map[string][]string, spec mallWeatherVersionIndexSpec) []string {
	result := make([]string, 0)
	for name, columns := range indexes {
		if strings.EqualFold(name, "PRIMARY") || mallWeatherVersionIndexColumnsMatch(columns, spec.Columns) {
			continue
		}
		if strings.EqualFold(name, spec.IndexName) || mallWeatherVersionIndexColumnsAreStrictSubset(columns, spec.Columns) {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func mallWeatherVersionIndexColumnsAreStrictSubset(actual, expected []string) bool {
	if len(actual) == 0 || len(actual) >= len(expected) {
		return false
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, column := range expected {
		expectedSet[strings.ToLower(strings.TrimSpace(column))] = struct{}{}
	}
	for _, column := range actual {
		if _, exists := expectedSet[strings.ToLower(strings.TrimSpace(column))]; !exists {
			return false
		}
	}
	return true
}

func mallWeatherVersionIndexColumnsMatch(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if !strings.EqualFold(strings.TrimSpace(actual[index]), strings.TrimSpace(expected[index])) {
			return false
		}
	}
	return true
}
