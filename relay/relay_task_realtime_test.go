package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRealtimeFetchFinalizesFailureBillingExactlyOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"error":{"message":"upstream failed"}}`))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousSQLite := common.UsingSQLite
	previousRedis := common.RedisEnabled
	previousBatch := common.BatchUpdateEnabled
	previousLogConsume := common.LogConsumeEnabled
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.UsingSQLite = previousSQLite
		common.RedisEnabled = previousRedis
		common.BatchUpdateEnabled = previousBatch
		common.LogConsumeEnabled = previousLogConsume
	})
	model.DB, model.LOG_DB = db, db
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	service.InitHttpClient()
	require.NoError(t, db.AutoMigrate(
		&model.Task{}, &model.TaskBillingOperation{}, &model.User{}, &model.Token{},
		&model.Log{}, &model.Channel{}, &model.UserQuotaTransaction{},
	))

	const userID, tokenID, channelID = 801, 802, 803
	const userQuota, tokenRemain, preConsumed = 10000, 5000, 1800
	require.NoError(t, db.Create(&model.User{
		Id: userID, Username: "realtime-user", Quota: userQuota,
		UsedQuota: preConsumed, Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: tokenID, UserId: userID, Key: "realtime-token", Name: "realtime-token",
		RemainQuota: tokenRemain, UsedQuota: preConsumed, Status: common.TokenStatusEnabled,
	}).Error)
	baseURL := server.URL
	require.NoError(t, db.Create(&model.Channel{
		Id: channelID, Name: "realtime-gemini", Key: "gemini-key",
		BaseURL: &baseURL, Type: constant.ChannelTypeGemini,
		Status: common.ChannelStatusEnabled, UsedQuota: preConsumed,
	}).Error)

	task := &model.Task{
		TaskID: "task_realtime_failure", Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeGemini)),
		UserId: userID, Group: "default", ChannelId: channelID, Quota: preConsumed,
		Status: model.TaskStatusInProgress, Progress: "50%", Data: json.RawMessage(`{}`),
		Properties: model.Properties{OriginModelName: "veo-test"},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: taskcommon.EncodeLocalTaskID("operations/realtime-test"),
			BillingSource:  "wallet", TokenId: tokenID,
		},
	}
	require.NoError(t, db.Create(task).Error)

	response := tryRealtimeFetch(context.Background(), task, false)
	require.NotEmpty(t, response)
	response = tryRealtimeFetch(context.Background(), task, false)
	require.NotEmpty(t, response)

	var storedTask model.Task
	require.NoError(t, db.First(&storedTask, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, storedTask.Status)
	assert.Equal(t, "upstream failed", storedTask.FailReason)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, userID).Error)
	assert.Equal(t, userQuota+preConsumed, storedUser.Quota)
	assert.Zero(t, storedUser.UsedQuota)
	var storedToken model.Token
	require.NoError(t, db.First(&storedToken, tokenID).Error)
	assert.Equal(t, tokenRemain+preConsumed, storedToken.RemainQuota)
	assert.Zero(t, storedToken.UsedQuota)
	var storedChannel model.Channel
	require.NoError(t, db.First(&storedChannel, channelID).Error)
	assert.Zero(t, storedChannel.UsedQuota)

	var operation model.TaskBillingOperation
	require.NoError(t, db.Where("task_record_id = ?", task.ID).First(&operation).Error)
	assert.Equal(t, model.TaskBillingOperationCompleted, operation.Status)
	var operationCount, logCount int64
	require.NoError(t, db.Model(&model.TaskBillingOperation{}).Count(&operationCount).Error)
	require.NoError(t, db.Model(&model.Log{}).Count(&logCount).Error)
	assert.Equal(t, int64(1), operationCount)
	assert.Equal(t, int64(1), logCount)
}
