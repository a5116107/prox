package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAgentImageConfigRequiresChatOpsSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := operation_setting.GetAgentSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	cfg.ChatOpsWebhookSecret = "chatops-secret"
	cfg.QQAccessToken = "qq-connector-token"
	cfg.ImageAPIKey = "image-secret"
	cfg.ImageModel = "gpt-image-2"

	for name, request := range map[string]*http.Request{
		"missing secret": httptest.NewRequest(http.MethodGet, "/api/agent/chatops/image-config?source=qq", nil),
		"query secret":   httptest.NewRequest(http.MethodGet, "/api/agent/chatops/image-config?source=qq&secret=chatops-secret", nil),
		"connector token": func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/api/agent/chatops/image-config?source=qq", nil)
			req.Header.Set("Authorization", "Bearer qq-connector-token")
			return req
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			unauthorized := httptest.NewRecorder()
			unauthorizedContext, _ := gin.CreateTestContext(unauthorized)
			unauthorizedContext.Request = request
			GetAgentImageConfig(unauthorizedContext)
			require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
		})
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/agent/chatops/image-config?source=qq", nil)
	context.Request.Header.Set("Authorization", "Bearer chatops-secret")
	GetAgentImageConfig(context)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	var response struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "image-secret", response.Data["api_key"])
	require.Equal(t, "gpt-image-2", response.Data["model"])
}

func TestGetOptionsNeverIncludesImageAPIKey(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	mapWasNil := common.OptionMap == nil
	if mapWasNil {
		common.OptionMap = make(map[string]string)
	}
	originalKey, keyExisted := common.OptionMap["agent_setting.image_api_key"]
	originalURL, urlExisted := common.OptionMap["agent_setting.image_api_base_url"]
	common.OptionMap["agent_setting.image_api_key"] = "must-not-leak"
	common.OptionMap["agent_setting.image_api_base_url"] = "https://images.example.test/v1"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if mapWasNil {
			common.OptionMap = nil
			return
		}
		if keyExisted {
			common.OptionMap["agent_setting.image_api_key"] = originalKey
		} else {
			delete(common.OptionMap, "agent_setting.image_api_key")
		}
		if urlExisted {
			common.OptionMap["agent_setting.image_api_base_url"] = originalURL
		} else {
			delete(common.OptionMap, "agent_setting.image_api_base_url")
		}
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/option", nil)
	GetOptions(context)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	keys := make([]string, 0, len(response.Data))
	for _, option := range response.Data {
		keys = append(keys, option.Key)
	}
	require.NotContains(t, keys, "agent_setting.image_api_key")
	require.Contains(t, keys, "agent_setting.image_api_base_url")
}
