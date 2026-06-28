package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyTokenBillingMultiplierDefaultGLMTiers(t *testing.T) {
	setTokenBillingMultiplierRulesForTest(t, defaultTokenBillingMultiplierRules())

	require.Equal(t, 500, applyTokenBillingMultiplier("glm-5.2", 500))
	require.Equal(t, 551, applyTokenBillingMultiplier("glm-5.2", 501))
	require.Equal(t, 1100, applyTokenBillingMultiplier("glm-5.2", 1000))
	require.Equal(t, 1201, applyTokenBillingMultiplier("glm-5.2", 1001))
	require.Equal(t, 187500, applyTokenBillingMultiplier("openrouter/z-ai/glm-5.1-air", 125000))
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

func setTokenBillingMultiplierRulesForTest(t *testing.T, rules []tokenBillingMultiplierRule) {
	t.Helper()

	tokenBillingMultiplierConfigMu.Lock()
	previousRules := cloneTokenBillingMultiplierRules(tokenBillingMultiplierRules)
	previousLoaded := tokenBillingMultiplierConfigLoaded
	tokenBillingMultiplierRules = cloneTokenBillingMultiplierRules(rules)
	tokenBillingMultiplierConfigLoaded = true
	tokenBillingMultiplierConfigMu.Unlock()

	t.Cleanup(func() {
		tokenBillingMultiplierConfigMu.Lock()
		tokenBillingMultiplierRules = previousRules
		tokenBillingMultiplierConfigLoaded = previousLoaded
		tokenBillingMultiplierConfigMu.Unlock()
	})
}

func resetTokenBillingMultiplierConfigForTest(t *testing.T) {
	t.Helper()

	tokenBillingMultiplierConfigMu.Lock()
	previousRules := cloneTokenBillingMultiplierRules(tokenBillingMultiplierRules)
	previousLoaded := tokenBillingMultiplierConfigLoaded
	tokenBillingMultiplierRules = nil
	tokenBillingMultiplierConfigLoaded = false
	tokenBillingMultiplierConfigMu.Unlock()

	t.Cleanup(func() {
		tokenBillingMultiplierConfigMu.Lock()
		tokenBillingMultiplierRules = previousRules
		tokenBillingMultiplierConfigLoaded = previousLoaded
		tokenBillingMultiplierConfigMu.Unlock()
	})
}
