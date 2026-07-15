package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelStatusControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldSQLite := common.UsingSQLite
	oldMySQL := common.UsingMySQL
	oldPostgreSQL := common.UsingPostgreSQL
	oldRedis := common.RedisEnabled
	oldMemoryCache := common.MemoryCacheEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.UsingSQLite = oldSQLite
		common.UsingMySQL = oldMySQL
		common.UsingPostgreSQL = oldPostgreSQL
		common.RedisEnabled = oldRedis
		common.MemoryCacheEnabled = oldMemoryCache
	})

	return db
}

func runChannelStatusRequest(t *testing.T, channelID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: channelID}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/"+channelID+"/status", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateChannelStatus(ctx)
	return recorder
}

func TestUpdateChannelStatusCompatibilityEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupChannelStatusControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "root",
		Password: "password",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	autoBan := 1
	require.NoError(t, db.Create(&model.Channel{
		Id:      38,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "preserved-secret",
		Status:  common.ChannelStatusEnabled,
		Name:    "compat-channel",
		Models:  "gpt-4o",
		Group:   "default",
		AutoBan: &autoBan,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-4o",
		ChannelId: 38,
		Enabled:   true,
	}).Error)

	disableResponse := runChannelStatusRequest(t, "38", `{"status":2}`)
	require.Equal(t, http.StatusOK, disableResponse.Code)
	require.Contains(t, disableResponse.Body.String(), `"success":true`)

	var channel model.Channel
	require.NoError(t, db.First(&channel, 38).Error)
	require.Equal(t, common.ChannelStatusManuallyDisabled, channel.Status)
	require.Equal(t, "preserved-secret", channel.Key)
	var ability model.Ability
	require.NoError(t, db.Where("channel_id = ?", 38).First(&ability).Error)
	require.False(t, ability.Enabled)

	enableResponse := runChannelStatusRequest(t, "38", `{"status":1}`)
	require.Equal(t, http.StatusOK, enableResponse.Code)
	require.NoError(t, db.First(&channel, 38).Error)
	require.Equal(t, common.ChannelStatusEnabled, channel.Status)
	require.NoError(t, db.Where("channel_id = ?", 38).First(&ability).Error)
	require.True(t, ability.Enabled)

	var auditCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("action = ?", "channel.status_update").Count(&auditCount).Error)
	require.Equal(t, int64(2), auditCount)
}

func TestUpdateChannelStatusRejectsUnsupportedStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupChannelStatusControllerTestDB(t)

	response := runChannelStatusRequest(t, "38", `{"status":3}`)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "status must be 1 or 2")
}
