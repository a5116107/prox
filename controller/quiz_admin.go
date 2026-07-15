package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ListOpsQuizBanks(c *gin.Context) {
	items, err := service.ListOpsQuizBanks(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "count": len(items)})
}

func CreateOpsQuizBank(c *gin.Context) {
	var input service.OpsQuizBankInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	bank, err := service.CreateOpsQuizBank(c.Param("site_id"), c.GetInt("id"), input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.quiz_bank.create", map[string]any{"site_id": c.Param("site_id"), "bank_id": bank.Id})
	common.ApiSuccess(c, bank)
}

func UpdateOpsQuizBank(c *gin.Context) {
	bankID := quizAdminParamInt(c, "bank_id")
	var input service.OpsQuizBankInput
	if bankID <= 0 || c.ShouldBindJSON(&input) != nil {
		common.ApiErrorMsg(c, "无效的题库参数")
		return
	}
	bank, err := service.UpdateOpsQuizBank(c.Param("site_id"), bankID, input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.quiz_bank.update", map[string]any{"site_id": c.Param("site_id"), "bank_id": bankID})
	common.ApiSuccess(c, bank)
}

func PublishOpsQuizBank(c *gin.Context) {
	bankID := quizAdminParamInt(c, "bank_id")
	bank, err := service.PublishOpsQuizBank(c.Param("site_id"), bankID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.quiz_bank.publish", map[string]any{"site_id": c.Param("site_id"), "bank_id": bankID})
	common.ApiSuccess(c, bank)
}

func ListOpsQuizQuestions(c *gin.Context) {
	bankID := quizAdminParamInt(c, "bank_id")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	result, err := service.ListOpsQuizQuestions(c.Param("site_id"), bankID, c.Query("status"), c.Query("search"), offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func CreateOpsQuizQuestion(c *gin.Context) {
	saveOpsQuizQuestion(c, 0)
}

func UpdateOpsQuizQuestion(c *gin.Context) {
	saveOpsQuizQuestion(c, quizAdminParamInt(c, "question_id"))
}

func saveOpsQuizQuestion(c *gin.Context, questionID int) {
	bankID := quizAdminParamInt(c, "bank_id")
	var input service.OpsQuizQuestionInput
	if bankID <= 0 || c.ShouldBindJSON(&input) != nil {
		common.ApiErrorMsg(c, "无效的题目参数")
		return
	}
	question, err := service.SaveOpsQuizQuestion(c.Param("site_id"), c.GetInt("id"), bankID, questionID, input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	action := "ops.quiz_question.create"
	if questionID > 0 {
		action = "ops.quiz_question.update"
	}
	recordManageAudit(c, action, map[string]any{"site_id": c.Param("site_id"), "bank_id": bankID, "question_id": question.Id})
	common.ApiSuccess(c, question)
}

func SetOpsQuizQuestionStatus(c *gin.Context) {
	bankID := quizAdminParamInt(c, "bank_id")
	questionID := quizAdminParamInt(c, "question_id")
	var input struct {
		Status string `json:"status"`
	}
	if bankID <= 0 || questionID <= 0 || c.ShouldBindJSON(&input) != nil {
		common.ApiErrorMsg(c, "无效的题目状态参数")
		return
	}
	if err := service.SetOpsQuizQuestionStatus(c.Param("site_id"), bankID, questionID, input.Status); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.quiz_question.status", map[string]any{"site_id": c.Param("site_id"), "bank_id": bankID, "question_id": questionID, "status": input.Status})
	common.ApiSuccess(c, gin.H{"question_id": questionID, "status": input.Status})
}

func ImportOpsQuizQuestions(c *gin.Context) {
	bankID := quizAdminParamInt(c, "bank_id")
	var request service.OpsQuizImportRequest
	if bankID <= 0 || c.ShouldBindJSON(&request) != nil {
		common.ApiErrorMsg(c, "无效的题库导入参数")
		return
	}
	result, err := service.ImportOpsQuizQuestions(c.Param("site_id"), c.GetInt("id"), bankID, request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !request.DryRun {
		recordManageAudit(c, "ops.quiz_question.import", map[string]any{"site_id": c.Param("site_id"), "bank_id": bankID, "result": result})
	}
	common.ApiSuccess(c, result)
}

func ListOpsQuizBindings(c *gin.Context) {
	items, err := service.ListOpsQuizBindings(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "count": len(items)})
}

func SaveOpsQuizBinding(c *gin.Context) {
	var input service.OpsQuizBindingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := service.SaveOpsQuizBinding(c.Param("site_id"), input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.quiz_binding.save", map[string]any{"site_id": c.Param("site_id"), "binding_id": binding.Id, "bank_id": binding.BankId})
	common.ApiSuccess(c, binding)
}

func DeleteOpsQuizBinding(c *gin.Context) {
	bindingID := quizAdminParamInt(c, "binding_id")
	if bindingID <= 0 {
		common.ApiErrorMsg(c, "无效的题库绑定参数")
		return
	}
	if err := service.DeleteOpsQuizBinding(c.Param("site_id"), bindingID); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.quiz_binding.delete", map[string]any{"site_id": c.Param("site_id"), "binding_id": bindingID})
	common.ApiSuccess(c, gin.H{"binding_id": bindingID, "deleted": true})
}

func GetOpsQuizStats(c *gin.Context) {
	result, err := service.GetOpsQuizStats(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func quizAdminParamInt(c *gin.Context, name string) int {
	value, _ := strconv.Atoi(c.Param(name))
	return value
}
