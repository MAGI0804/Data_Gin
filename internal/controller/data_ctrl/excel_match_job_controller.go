package data_ctrl

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/msg"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/model"
)

type excelUploadSessionRequest struct {
	FileName    string `json:"fileName"`
	TotalChunks int    `json:"totalChunks"`
}

type excelUploadCompleteRequest struct {
	TotalChunks int `json:"totalChunks"`
}

type excelMatchSchemeRequest struct {
	Name      string          `json:"name"`
	Operation string          `json:"operation"`
	Config    json.RawMessage `json:"config"`
}

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
	config := c.PostForm("config")
	if config == "" {
		c.JSON(400, msg.ErrResponseStr("匹配配置不能为空"))
		return
	}

	uploadID := c.PostForm("uploadId")
	var matchJob *model.ExcelMatchJob
	if err == nil {
		matchJob, err = ctrl.service.CreateJob(c.Request.Context(), fileHeader, config)
	} else if uploadID != "" {
		matchJob, err = ctrl.service.CreateJobFromUpload(c.Request.Context(), uploadID, config)
	} else {
		c.JSON(400, msg.ErrResponse("读取 Excel 文件失败", err))
		return
	}
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

func (ctrl *ExcelMatchJobController) Preview(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	config := c.PostForm("config")
	if config == "" {
		c.JSON(400, msg.ErrResponseStr("匹配配置不能为空"))
		return
	}

	uploadID := c.PostForm("uploadId")
	var preview *data_svc.ExcelMatchPreviewResult
	if err == nil {
		preview, err = ctrl.service.Preview(c.Request.Context(), fileHeader, config)
	} else if uploadID != "" {
		preview, err = ctrl.service.PreviewUploaded(c.Request.Context(), uploadID, config)
	} else {
		c.JSON(400, msg.ErrResponse("读取 Excel 文件失败", err))
		return
	}
	if err != nil {
		c.JSON(400, msg.ErrResponse("预览 Excel 匹配失败", err))
		return
	}

	c.JSON(200, msg.SuccessResponse("预览 Excel 匹配成功", &map[string]any{
		"preview": preview,
	}))
}

func (ctrl *ExcelMatchJobController) CreateUploadSession(c *gin.Context) {
	var req excelUploadSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("读取上传会话参数失败", err))
		return
	}
	session, err := ctrl.service.CreateUploadSession(c.Request.Context(), req.FileName, req.TotalChunks)
	if err != nil {
		c.JSON(400, msg.ErrResponse("创建 Excel 分片上传会话失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("Excel 分片上传会话已创建", &map[string]any{
		"upload": session,
	}))
}

func (ctrl *ExcelMatchJobController) UploadChunk(c *gin.Context) {
	uploadID := c.Param("upload_id")
	index, err := strconv.Atoi(c.PostForm("index"))
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的分片序号", err))
		return
	}
	totalChunks, err := strconv.Atoi(c.PostForm("totalChunks"))
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的分片总数", err))
		return
	}
	fileHeader, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(400, msg.ErrResponse("读取 Excel 分片失败", err))
		return
	}

	session, err := ctrl.service.SaveUploadChunk(c.Request.Context(), uploadID, index, totalChunks, fileHeader)
	if err != nil {
		c.JSON(400, msg.ErrResponse("保存 Excel 分片失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("Excel 分片已保存", &map[string]any{
		"upload": session,
	}))
}

func (ctrl *ExcelMatchJobController) CompleteUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")
	var req excelUploadCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("读取上传合并参数失败", err))
		return
	}
	session, err := ctrl.service.CompleteUpload(c.Request.Context(), uploadID, req.TotalChunks)
	if err != nil {
		c.JSON(400, msg.ErrResponse("合并 Excel 分片失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("Excel 分片已合并", &map[string]any{
		"upload": session,
	}))
}

func (ctrl *ExcelMatchJobController) ListSchemes(c *gin.Context) {
	schemes, err := ctrl.service.ListSchemes(c.Request.Context(), c.Query("operation"))
	if err != nil {
		c.JSON(400, msg.ErrResponse("查询 Excel 匹配方案失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询 Excel 匹配方案成功", &map[string]any{
		"schemes": schemes,
	}))
}

func (ctrl *ExcelMatchJobController) SaveScheme(c *gin.Context) {
	var req excelMatchSchemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, msg.ErrResponse("读取 Excel 匹配方案失败", err))
		return
	}
	scheme, err := ctrl.service.SaveScheme(c.Request.Context(), req.Name, req.Operation, string(req.Config))
	if err != nil {
		c.JSON(400, msg.ErrResponse("保存 Excel 匹配方案失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("保存 Excel 匹配方案成功", &map[string]any{
		"scheme": scheme,
	}))
}

func (ctrl *ExcelMatchJobController) DeleteScheme(c *gin.Context) {
	id, err := parseUintParam(c, "scheme_id")
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的 Excel 匹配方案ID", err))
		return
	}
	if err := ctrl.service.DeleteScheme(c.Request.Context(), id); err != nil {
		c.JSON(400, msg.ErrResponse("删除 Excel 匹配方案失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponseStr("删除 Excel 匹配方案成功"))
}

func (ctrl *ExcelMatchJobController) ListJobs(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if err != nil {
		c.JSON(400, msg.ErrResponse("无效的 Excel 匹配任务数量", err))
		return
	}
	jobs, err := ctrl.service.ListJobs(c.Request.Context(), limit)
	if err != nil {
		c.JSON(500, msg.ErrResponse("查询 Excel 匹配任务列表失败", err))
		return
	}
	c.JSON(200, msg.SuccessResponse("查询 Excel 匹配任务列表成功", &map[string]any{
		"jobs": jobs,
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

	matchJob, resultPath, filename, err := ctrl.service.Download(c.Request.Context(), id)
	if err != nil {
		c.JSON(400, msg.ErrResponse("下载 Excel 匹配结果失败", err))
		return
	}
	if strings.TrimSpace(matchJob.ResultURL) != "" {
		c.Redirect(http.StatusFound, matchJob.ResultURL)
		return
	}

	c.FileAttachment(resultPath, filename)
}
