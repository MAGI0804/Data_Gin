package cmd

import (
	"encoding/json"

	"gin-biz-web-api/internal/service/data_svc"

	"github.com/spf13/cobra"
)

// MallWeatherCmd provides internal mall weather operations.
var MallWeatherCmd = NewMallWeatherCmd()

func NewMallWeatherCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "mall-weather",
		Short: "商场天气内部运维命令",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(newMallWeatherCapacityPlanCmd())
	return command
}

func newMallWeatherCapacityPlanCmd() *cobra.Command {
	input := data_svc.MallWeatherCapacityPlanInput{
		HourlySteps:   360,
		DailySteps:    15,
		LifeIndexDays: 15,
	}
	command := &cobra.Command{
		Use:   "capacity-plan",
		Short: "估算商场天气容量、调用量和批次数",
		Example: "go run main.go mall-weather capacity-plan " +
			"--mall-count 1000 --provider-qps 20 --alerts-per-mall 3",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := data_svc.BuildMallWeatherCapacityPlan(input)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(plan)
		},
	}
	flags := command.Flags()
	flags.IntVar(&input.MallCount, "mall-count", 0, "启用天气的商场数量")
	flags.Float64Var(&input.ProviderQPS, "provider-qps", 0, "彩云控制台确认的实际可用 QPS")
	flags.IntVar(&input.HourlySteps, "hourly-steps", input.HourlySteps, "每次 full 采集的小时预报步数，最多 360")
	flags.IntVar(&input.DailySteps, "daily-steps", input.DailySteps, "每次 full 采集的日预报步数，最多 15")
	flags.IntVar(&input.LifeIndexDays, "life-index-days", input.LifeIndexDays, "每次 v3 生活指数采集天数，最多 15")
	flags.IntVar(&input.AlertsPerMall, "alerts-per-mall", 0, "压测估算使用的每商场预警条数")
	flags.IntVar(&input.FeishuBatchRows, "feishu-batch-rows", 0, "飞书批次行数，默认使用 Destination 默认值")
	_ = command.MarkFlagRequired("mall-count")
	_ = command.MarkFlagRequired("provider-qps")
	return command
}
