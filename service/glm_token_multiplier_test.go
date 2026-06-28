package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyGLMTokenBillingMultiplier(t *testing.T) {
	require.Equal(t, 99, applyGLMTokenBillingMultiplier("glm-5.2", 99))
	require.Equal(t, 1000, applyGLMTokenBillingMultiplier("glm-5.2", 1000))
	require.Equal(t, 2002, applyGLMTokenBillingMultiplier("glm-5.2", 1001))
	require.Equal(t, 2500, applyGLMTokenBillingMultiplier("openrouter/z-ai/glm-5.1-air", 1250))
	require.Equal(t, 1250, applyGLMTokenBillingMultiplier("glm-4.6", 1250))
}
