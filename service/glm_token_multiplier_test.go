package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestApplyTokenBillingMultiplierDefaultGLMTiers(t *testing.T) {
	setTokenBillingMultiplierRulesForTest(t, defaultTokenBillingMultiplierRules())

	require.Equal(t, 200, applyTokenBillingMultiplier("glm-5.2", 200))
	require.Equal(t, 241, applyTokenBillingMultiplier("glm-5.2", 201))
	require.Equal(t, 600, applyTokenBillingMultiplier("glm-5.2", 500))
	require.Equal(t, 601, applyTokenBillingMultiplier("glm-5.2", 501))
	require.Equal(t, 1200, applyTokenBillingMultiplier("glm-5.2", 1000))
	require.Equal(t, 1351, applyTokenBillingMultiplier("glm-5.2", 1001))
	require.Equal(t, 212500, applyTokenBillingMultiplier("openrouter/z-ai/glm-5.1-air", 125000))
	require.Equal(t, 1250, applyTokenBillingMultiplier("glm-4.6", 1250))
}

func TestApplyTokenBillingMultiplierLoadsConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "token_billing_tiers.json")
	err := os.WriteFile(configPath, []byte(`{
  "rules": [
    {
      "model_contains": ["glm-5.2"],
      "tiers": [
        { "above_tokens": 10, "multiplier": 1.5 },
        { "above_tokens": 20, "multiplier": 2.0 }
      ]
    }
  ]
}`), 0o600)
	require.NoError(t, err)

	resetTokenBillingMultiplierConfigForTest(t)
	t.Setenv(tokenBillingMultiplierConfigPathEnv, configPath)

	require.Equal(t, 10, applyTokenBillingMultiplier("glm-5.2", 10))
	require.Equal(t, 17, applyTokenBillingMultiplier("glm-5.2", 11))
	require.Equal(t, 42, applyTokenBillingMultiplier("glm-5.2", 21))
	require.Equal(t, 21, applyTokenBillingMultiplier("glm-5.1", 21))
}

func TestApplyTokenBillingMultiplierConfigCanDisableRules(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "token_billing_tiers.json")
	err := os.WriteFile(configPath, []byte(`{
  "enabled": false,
  "rules": [
    {
      "model_contains": ["glm-5.2"],
      "tiers": [
        { "above_tokens": 10, "multiplier": 2.0 }
      ]
    }
  ]
}`), 0o600)
	require.NoError(t, err)

	resetTokenBillingMultiplierConfigForTest(t)
	t.Setenv(tokenBillingMultiplierConfigPathEnv, configPath)

	require.Equal(t, 100, applyTokenBillingMultiplier("glm-5.2", 100))
}

func TestApplyTokenBillingMultiplierHotReloadsConfigToggle(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "token_billing_tiers.json")
	enabledConfig := `{
  "enabled": true,
  "reload_interval_seconds": 1,
  "rules": [
    {
      "model_contains": ["glm-5.2"],
      "tiers": [
        { "above_tokens": 10, "multiplier": 2.0 }
      ]
    }
  ]
}`
	disabledConfig := `{
  "enabled": false,
  "reload_interval_seconds": 1,
  "rules": [
    {
      "model_contains": ["glm-5.2"],
      "tiers": [
        { "above_tokens": 10, "multiplier": 2.0 }
      ]
    }
  ]
}`
	require.NoError(t, os.WriteFile(configPath, []byte(enabledConfig), 0o600))

	resetTokenBillingMultiplierConfigForTest(t)
	t.Setenv(tokenBillingMultiplierConfigPathEnv, configPath)

	require.Equal(t, 40, applyTokenBillingMultiplier("glm-5.2", 20))

	require.NoError(t, os.WriteFile(configPath, []byte(disabledConfig), 0o600))
	forceTokenBillingMultiplierReloadForTest(t)

	require.Equal(t, 20, applyTokenBillingMultiplier("glm-5.2", 20))
}

func TestApplyTokenBillingMultiplierToUsageReturnsAdjustedCopy(t *testing.T) {
	setTokenBillingMultiplierRulesForTest(t, defaultTokenBillingMultiplierRules())

	inputDetails := &dto.InputTokenDetails{CachedTokens: 100, TextTokens: 700}
	usage := &dto.Usage{
		PromptTokens:     800,
		CompletionTokens: 201,
		TotalTokens:      1001,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 20,
			TextTokens:           700,
		},
		InputTokens:        800,
		OutputTokens:       201,
		InputTokensDetails: inputDetails,
	}

	adjusted, ok := ApplyTokenBillingMultiplierToUsage("glm-5.2", usage)

	require.True(t, ok)
	require.Equal(t, 940, adjusted.PromptTokens)
	require.Equal(t, 241, adjusted.CompletionTokens)
	require.Equal(t, 1181, adjusted.TotalTokens)
	require.Equal(t, 100, adjusted.PromptTokensDetails.CachedTokens)
	require.Equal(t, 24, adjusted.PromptTokensDetails.CachedCreationTokens)
	require.Equal(t, 840, adjusted.PromptTokensDetails.TextTokens)
	require.Equal(t, 940, adjusted.InputTokens)
	require.Equal(t, 241, adjusted.OutputTokens)
	require.NotSame(t, usage.InputTokensDetails, adjusted.InputTokensDetails)
	require.Equal(t, 100, adjusted.InputTokensDetails.CachedTokens)
	require.Equal(t, 800, usage.PromptTokens)
	require.Equal(t, 100, usage.InputTokensDetails.CachedTokens)
}

func TestApplyTokenBillingMultiplierToUsageUsesNonCacheTokensForTier(t *testing.T) {
	setTokenBillingMultiplierRulesForTest(t, defaultTokenBillingMultiplierRules())

	usage := &dto.Usage{
		PromptTokens:     16077,
		CompletionTokens: 221,
		TotalTokens:      16298,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 16028,
		},
	}

	adjusted, ok := ApplyTokenBillingMultiplierToUsage("glm-5.2", usage)

	require.True(t, ok)
	require.Equal(t, 16087, adjusted.PromptTokens)
	require.Equal(t, 265, adjusted.CompletionTokens)
	require.Equal(t, 16352, adjusted.TotalTokens)
	require.Equal(t, 16028, adjusted.PromptTokensDetails.CachedTokens)
}

func setTokenBillingMultiplierRulesForTest(t *testing.T, rules []tokenBillingMultiplierRule) {
	t.Helper()

	tokenBillingMultiplierConfigMu.Lock()
	previousRules := cloneTokenBillingMultiplierRules(tokenBillingMultiplierRules)
	previousEnabled := tokenBillingMultiplierEnabled
	previousLoaded := tokenBillingMultiplierConfigLoaded
	previousPath := tokenBillingMultiplierConfigPath
	previousModAt := tokenBillingMultiplierConfigModAt
	previousLastCheckAt := tokenBillingMultiplierLastCheckAt
	previousReloadEvery := tokenBillingMultiplierReloadEvery
	tokenBillingMultiplierEnabled = true
	tokenBillingMultiplierRules = cloneTokenBillingMultiplierRules(rules)
	tokenBillingMultiplierConfigLoaded = true
	tokenBillingMultiplierConfigMu.Unlock()

	t.Cleanup(func() {
		tokenBillingMultiplierConfigMu.Lock()
		tokenBillingMultiplierEnabled = previousEnabled
		tokenBillingMultiplierRules = previousRules
		tokenBillingMultiplierConfigLoaded = previousLoaded
		tokenBillingMultiplierConfigPath = previousPath
		tokenBillingMultiplierConfigModAt = previousModAt
		tokenBillingMultiplierLastCheckAt = previousLastCheckAt
		tokenBillingMultiplierReloadEvery = previousReloadEvery
		tokenBillingMultiplierConfigMu.Unlock()
	})
}

func resetTokenBillingMultiplierConfigForTest(t *testing.T) {
	t.Helper()

	tokenBillingMultiplierConfigMu.Lock()
	previousRules := cloneTokenBillingMultiplierRules(tokenBillingMultiplierRules)
	previousEnabled := tokenBillingMultiplierEnabled
	previousLoaded := tokenBillingMultiplierConfigLoaded
	previousPath := tokenBillingMultiplierConfigPath
	previousModAt := tokenBillingMultiplierConfigModAt
	previousLastCheckAt := tokenBillingMultiplierLastCheckAt
	previousReloadEvery := tokenBillingMultiplierReloadEvery
	tokenBillingMultiplierRules = nil
	tokenBillingMultiplierEnabled = false
	tokenBillingMultiplierConfigLoaded = false
	tokenBillingMultiplierConfigPath = ""
	tokenBillingMultiplierConfigModAt = time.Time{}
	tokenBillingMultiplierLastCheckAt = time.Time{}
	tokenBillingMultiplierReloadEvery = 0
	tokenBillingMultiplierConfigMu.Unlock()

	t.Cleanup(func() {
		tokenBillingMultiplierConfigMu.Lock()
		tokenBillingMultiplierRules = previousRules
		tokenBillingMultiplierEnabled = previousEnabled
		tokenBillingMultiplierConfigLoaded = previousLoaded
		tokenBillingMultiplierConfigPath = previousPath
		tokenBillingMultiplierConfigModAt = previousModAt
		tokenBillingMultiplierLastCheckAt = previousLastCheckAt
		tokenBillingMultiplierReloadEvery = previousReloadEvery
		tokenBillingMultiplierConfigMu.Unlock()
	})
}

func forceTokenBillingMultiplierReloadForTest(t *testing.T) {
	t.Helper()

	tokenBillingMultiplierConfigMu.Lock()
	tokenBillingMultiplierLastCheckAt = time.Now().Add(-time.Hour)
	tokenBillingMultiplierConfigModAt = time.Time{}
	tokenBillingMultiplierConfigMu.Unlock()
}
