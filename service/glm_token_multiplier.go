package service

import (
	"math"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const tokenBillingMultiplierConfigPathEnv = "TOKEN_BILLING_CONFIG_PATH"

type tokenBillingMultiplierConfig struct {
	Rules []tokenBillingMultiplierRule `json:"rules"`
}

type tokenBillingMultiplierRule struct {
	ModelContains []string                     `json:"model_contains"`
	Tiers         []tokenBillingMultiplierTier `json:"tiers"`
}

type tokenBillingMultiplierTier struct {
	AboveTokens int     `json:"above_tokens"`
	Multiplier  float64 `json:"multiplier"`
}

var (
	tokenBillingMultiplierConfigMu     sync.RWMutex
	tokenBillingMultiplierConfigLoaded bool
	tokenBillingMultiplierRules        []tokenBillingMultiplierRule
)

func applyTokenBillingMultiplier(modelName string, tokenCount int) int {
	return scaleTokenCount(tokenCount, tokenBillingMultiplierFor(modelName, tokenCount))
}

func tokenBillingMultiplierFor(modelName string, tokenCount int) float64 {
	if tokenCount <= 0 {
		return 1
	}

	modelName = strings.ToLower(modelName)
	multiplier := 1.0
	for _, rule := range getTokenBillingMultiplierRules() {
		if !tokenBillingRuleMatchesModel(rule, modelName) {
			continue
		}
		for _, tier := range rule.Tiers {
			if tokenCount > tier.AboveTokens && tier.Multiplier > 0 && tier.Multiplier > multiplier {
				multiplier = tier.Multiplier
			}
		}
	}
	return multiplier
}

func getTokenBillingMultiplierRules() []tokenBillingMultiplierRule {
	tokenBillingMultiplierConfigMu.RLock()
	if tokenBillingMultiplierConfigLoaded {
		rules := cloneTokenBillingMultiplierRules(tokenBillingMultiplierRules)
		tokenBillingMultiplierConfigMu.RUnlock()
		return rules
	}
	tokenBillingMultiplierConfigMu.RUnlock()

	tokenBillingMultiplierConfigMu.Lock()
	defer tokenBillingMultiplierConfigMu.Unlock()
	if !tokenBillingMultiplierConfigLoaded {
		tokenBillingMultiplierRules = loadTokenBillingMultiplierRules()
		tokenBillingMultiplierConfigLoaded = true
	}
	return cloneTokenBillingMultiplierRules(tokenBillingMultiplierRules)
}

func loadTokenBillingMultiplierRules() []tokenBillingMultiplierRule {
	configPath := os.Getenv(tokenBillingMultiplierConfigPathEnv)
	if strings.TrimSpace(configPath) == "" {
		configPath = "token_billing_tiers.json"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			common.SysError("failed to read token billing multiplier config: " + err.Error())
		}
		return defaultTokenBillingMultiplierRules()
	}

	var cfg tokenBillingMultiplierConfig
	if err := common.Unmarshal(data, &cfg); err != nil {
		common.SysError("failed to parse token billing multiplier config: " + err.Error())
		return defaultTokenBillingMultiplierRules()
	}

	rules := normalizeTokenBillingMultiplierRules(cfg.Rules)
	if len(rules) == 0 {
		return defaultTokenBillingMultiplierRules()
	}
	return rules
}

func defaultTokenBillingMultiplierRules() []tokenBillingMultiplierRule {
	return []tokenBillingMultiplierRule{
		{
			ModelContains: []string{"glm-5.1", "glm-5.2"},
			Tiers: []tokenBillingMultiplierTier{
				{AboveTokens: 500, Multiplier: 1.2},
				{AboveTokens: 1000, Multiplier: 1.3},
				{AboveTokens: 10000, Multiplier: 1.4},
				{AboveTokens: 50000, Multiplier: 1.5},
				{AboveTokens: 100000, Multiplier: 1.6},
				{AboveTokens: 200000, Multiplier: 1.7},
			},
		},
	}
}

func normalizeTokenBillingMultiplierRules(rules []tokenBillingMultiplierRule) []tokenBillingMultiplierRule {
	normalized := make([]tokenBillingMultiplierRule, 0, len(rules))
	for _, rule := range rules {
		modelContains := make([]string, 0, len(rule.ModelContains))
		for _, pattern := range rule.ModelContains {
			pattern = strings.ToLower(strings.TrimSpace(pattern))
			if pattern != "" {
				modelContains = append(modelContains, pattern)
			}
		}
		if len(modelContains) == 0 {
			continue
		}

		tiers := make([]tokenBillingMultiplierTier, 0, len(rule.Tiers))
		for _, tier := range rule.Tiers {
			if tier.AboveTokens < 0 || tier.Multiplier <= 0 {
				continue
			}
			tiers = append(tiers, tier)
		}
		if len(tiers) == 0 {
			continue
		}

		normalized = append(normalized, tokenBillingMultiplierRule{
			ModelContains: modelContains,
			Tiers:         tiers,
		})
	}
	return normalized
}

func tokenBillingRuleMatchesModel(rule tokenBillingMultiplierRule, modelName string) bool {
	for _, pattern := range rule.ModelContains {
		if strings.Contains(modelName, pattern) {
			return true
		}
	}
	return false
}

func scaleTokenCount(tokenCount int, multiplier float64) int {
	if tokenCount <= 0 || multiplier <= 0 || multiplier == 1 {
		return tokenCount
	}
	return int(math.Round(float64(tokenCount) * multiplier))
}

func cloneTokenBillingMultiplierRules(rules []tokenBillingMultiplierRule) []tokenBillingMultiplierRule {
	cloned := make([]tokenBillingMultiplierRule, len(rules))
	for i, rule := range rules {
		cloned[i].ModelContains = append([]string(nil), rule.ModelContains...)
		cloned[i].Tiers = append([]tokenBillingMultiplierTier(nil), rule.Tiers...)
	}
	return cloned
}
