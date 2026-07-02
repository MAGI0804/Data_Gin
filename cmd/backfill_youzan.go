package cmd

import (
	"context"
	"time"

	"gin-biz-web-api/job"

	"github.com/spf13/cobra"
)

var (
	backfillStart string
	backfillEnd   string
	backfillDate  string
)

var BackfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "数据补拉",
	Long:  `用于补拉指定时间范围内的有赞订单和退款订单数据`,
}

var backfillYouzanCmd = &cobra.Command{
	Use:     "youzan",
	Short:   "补拉有赞订单数据",
	Long:    `从有赞API补拉指定时间范围内的订单和退款订单数据，并写入数据库。已存在的数据会被更新而非重复插入。`,
	Example: `  go run main.go backfill youzan --date 2026-06-15` + "\n" +
		`  go run main.go backfill youzan --start "2026-06-15 00:00:00" --end "2026-06-15 15:40:00"`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		var startTime, endTime string

		if backfillDate != "" {
			startTime = backfillDate + " 00:00:00"
			endTime = backfillDate + " 23:59:59"
		} else if backfillStart != "" && backfillEnd != "" {
			startTime = backfillStart
			endTime = backfillEnd
		} else {
			today := time.Now().Format("2006-01-02")
			startTime = today + " 00:00:00"
			endTime = time.Now().Format("2006-01-02 15:04:05")
		}

		cmd.Printf("开始补拉有赞数据: %s ~ %s\n", startTime, endTime)

		result, err := job.BackfillYouzanByRange(ctx, startTime, endTime)
		if err != nil {
			cmd.Printf("补拉失败: %v\n", err)
			return
		}

		cmd.Printf("补拉完成!\n")
		cmd.Printf("  时间范围: %s ~ %s\n", result.StartTime, result.EndTime)
		cmd.Printf("  订单数量: %d\n", result.OrderCount)
		cmd.Printf("  退款数量: %d\n", result.RefundCount)
		cmd.Printf("\n注意: 补拉写入的订单数据 synced=0，后续由定时任务的 youzan-sales-sync 任务推送到销售系统。\n")
	},
}

func init() {
	backfillYouzanCmd.Flags().StringVar(&backfillDate, "date", "", "按日期补拉（覆盖整天，格式：2026-06-15）")
	backfillYouzanCmd.Flags().StringVar(&backfillStart, "start", "", "起始时间（格式：2026-06-15 00:00:00）")
	backfillYouzanCmd.Flags().StringVar(&backfillEnd, "end", "", "结束时间（格式：2026-06-15 15:40:00）")

	BackfillCmd.AddCommand(backfillYouzanCmd)
}
