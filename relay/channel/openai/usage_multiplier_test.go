package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenaiHandlerReturnsAdjustedUsageToClientAndRawUsageForBilling(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	body := `{"id":"chatcmpl_1","object":"chat.completion","model":"glm-5.2","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":800,"completion_tokens":201,"total_tokens":1001}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{},
		OriginModelName: "glm-5.2",
		RelayFormat:     types.RelayFormatOpenAI,
	}

	usage, err := OpenaiHandler(c, info, resp)

	require.Nil(t, err)
	require.Equal(t, 800, usage.PromptTokens)
	require.Equal(t, 201, usage.CompletionTokens)
	require.Equal(t, 1001, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"usage":{"prompt_tokens":1080,"completion_tokens":271,"total_tokens":1351`)
}

func TestHandleFinalResponseReturnsAdjustedGeneratedStreamUsage(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{},
		OriginModelName:    "glm-5.1",
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
	}

	HandleFinalResponse(c, info, "", "chatcmpl_1", 1710000000, "glm-5.1", "", usagePtr(800, 201), false)

	got := recorder.Body.String()
	require.Contains(t, got, `"usage":{"prompt_tokens":1080,"completion_tokens":271,"total_tokens":1351`)
	require.Contains(t, got, `data: [DONE]`)
}

func usagePtr(promptTokens, completionTokens int) *dto.Usage {
	return &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}
