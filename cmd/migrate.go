package cmd

import (
	"gin-biz-web-api/bootstrap"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/database"
	"github.com/spf13/cobra"
)

var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "运行数据库迁移",
}

var migrateDataTablesCmd = &cobra.Command{
	Use:     "data-tables",
	Short:   "创建/更新数据存储相关表",
	Example: "go run main.go migrate data-tables",
	Run: func(cmd *cobra.Command, args []string) {
		console.Info("开始迁移数据存储表...")

		db := database.DB
		err := db.AutoMigrate(
			&model.DataSource{},
			&model.RawData{},
			&model.ProcessedData{},
			&model.DataStatistics{},
			&model.BojunRetailOrder{},
			&model.SourceDefinition{},
			&model.RawRecord{},
			&model.CleanTableDefinition{},
			&model.CleanRecord{},
			&model.TransformRule{},
			&model.DestinationDefinition{},
			&model.DeliveryTask{},
			&model.PipelineRun{},
			&model.DeliveryLog{},
			&model.ExcelMatchJob{},
			&model.ExcelMatchJobLog{},
		)

		console.ExitIf(err)
		console.Success("数据存储表迁移完成")
	},
}

var migrateQueryIndexesCmd = &cobra.Command{
	Use:     "query-indexes",
	Short:   "低峰在线创建开放查询复合索引",
	Example: "go run main.go migrate query-indexes",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		console.Info("开始迁移开放查询复合索引...")
		if err := bootstrap.ApplyQueryIndexes(cmd.Context(), database.DB); err != nil {
			return err
		}
		console.Success("开放查询复合索引迁移完成")
		return nil
	},
}

func init() {
	MigrateCmd.AddCommand(migrateDataTablesCmd)
	MigrateCmd.AddCommand(migrateQueryIndexesCmd)
}
