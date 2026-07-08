package data_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
)

type ExcelMatchJobController struct {
	service *data_svc.ExcelMatchJobService
}

func NewExcelMatchJobController() *ExcelMatchJobController {
	return &ExcelMatchJobController{
		service: data_svc.NewExcelMatchJobService(),
	}
}

func (ctrl *ExcelMatchJobController) CreateJob(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, msg.ErrResponse("读取 Excel 文件失败", err))
		return
	}
	config := c.PostForm("config")
	if config == "" {
		c.JSON(400, msg.ErrResponseStr("匹配配置不能为空"))
		return
	}

	matchJob, err := ctrl.service.CreateJob(c.Request.Context(), fileHeader, config)
	if err != nil {
		c.JSON(400, msg.ErrResponse("创建 Excel 匹配任务失败", err))
		return
	}
	logs, _ := ctrl.service.GetJobLogs(c.Request.Context(), matchJob.ID)

	c.JSON(200, msg.SuccessResponse("Excel 匹配任务已创建", &map[string]any{
		"job":          matchJob,
		"logs":         logs,
		"downloadPath": "/api/v1/excel-match-jobs/" + strconv.FormatUint(uint64(matchJob.ID), 10) + "/download",
	}))
}

func (ctrl *ExcelMatchJobController) GetJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的 Excel 匹配任务ID", err))
		return
	}

	matchJob, err := ctrl.service.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, msg.ErrResponse("查询 Excel 匹配任务失败", err))
		return
	}
	logs, err := ctrl.service.GetJobLogs(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询 Excel 任务日志失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("查询 Excel 匹配任务成功", &map[string]any{
		"job":          matchJob,
		"logs":         logs,
		"downloadPath": "/api/v1/excel-match-jobs/" + strconv.FormatUint(uint64(matchJob.ID), 10) + "/download",
	}))
}

func (ctrl *ExcelMatchJobController) Download(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的 Excel 匹配任务ID", err))
		return
	}

	_, resultPath, filename, err := ctrl.service.Download(c.Request.Context(), id)
	if err != nil {
		c.JSON(400, msg.ErrResponse("下载 Excel 匹配结果失败", err))
		return
	}

	c.FileAttachment(resultPath, filename)
}
