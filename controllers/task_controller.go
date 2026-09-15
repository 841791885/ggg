package controllers

import (
	"net/http"

	model "ggg/models"
	"ggg/repositories"

	"github.com/gin-gonic/gin"
)

// AdminListTasks 管理端分页查询后台任务，支持 status 和 task_type 过滤。
func (c *TradeController) AdminListTasks(ctx *gin.Context) {
	page, size := parsePageQuery(ctx)
	total, tasks, err := c.taskService.List(ctx.Request.Context(), repositories.ListTasksQuery{
		Status: model.TaskStatus(ctx.Query("status")), TaskType: ctx.Query("task_type"), Page: page, PageSize: size,
	})
	if err != nil {
		respondTradeError(ctx, err, "查询任务列表")
		return
	}
	items := make([]TaskResponse, len(tasks))
	for i := range tasks {
		items[i] = newTaskResponse(&tasks[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": items})
}

// AdminRetryTask 运营重试失败任务；非 failed 状态返回 409。
func (c *TradeController) AdminRetryTask(ctx *gin.Context) {
	operatorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	taskID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	task, err := c.taskService.Retry(ctx.Request.Context(), operatorID, taskID)
	if err != nil {
		respondTradeError(ctx, err, "重试任务")
		return
	}
	respondSuccess(ctx, http.StatusOK, newTaskResponse(&task))
}
