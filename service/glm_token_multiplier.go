package service

import (
	"math"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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

func ApplyTokenBillingMultiplierToUsage(modelName string, usage *dto.Usage) (dto.Usage, bool) {
	if usage == nil {
		return dto.Usage{}, false
	}

	adjusted := cloneUsage(usage)
	tokenCount := usage.PromptTokens + usage.CompletionTokens
	if tokenCount == 0 {
		tokenCount = usage.InputTokens + usage.OutputTokens
	}
	if tokenCount == 0 {
		tokenCount = usage.TotalTokens
	}

	multiplier := tokenBillingMultiplierFor(modelName, tokenCount)
	if multiplier == 1 {
		return adjusted, false
	}

	adjusted.PromptTokens = scaleTokenCount(adjusted.PromptTokens, multiplier)
	adjusted.CompletionTokens = scaleTokenCount(adjusted.CompletionTokens, multiplier)
	adjusted.PromptCacheHitTokens = scaleTokenCount(adjusted.PromptCacheHitTokens, multiplier)
	adjusted.PromptTokensDetails = scaleInputTokenDetails(adjusted.PromptTokensDetails, multiplier)
	adjusted.CompletionTokenDetails = scaleOutputTokenDetails(adjusted.CompletionTokenDetails, multiplier)
	adjusted.InputTokens = scaleTokenCount(adjusted.InputTokens, multiplier)
	adjusted.OutputTokens = scaleTokenCount(adjusted.OutputTokens, multiplier)
	if adjusted.InputTokensDetails != nil {
		details := scaleInputTokenDetails(*adjusted.InputTokensDetails, multiplier)
		adjusted.InputTokensDetails = &details
	}
	adjusted.ClaudeCacheCreation5mTokens = scaleTokenCount(adjusted.ClaudeCacheCreation5mTokens, multiplier)
	adjusted.ClaudeCacheCreation1hTokens = scaleTokenCount(adjusted.ClaudeCacheCreation1hTokens, multiplier)

	if adjusted.PromptTokens != 0 || adjusted.CompletionTokens != 0 {
		adjusted.TotalTokens = adjusted.PromptTokens + adjusted.CompletionTokens
	} else if adjusted.InputTokens != 0 || adjusted.OutputTokens != 0 {
		adjusted.TotalTokens = adjusted.InputTokens + adjusted.OutputTokens
	} else {
		adjusted.TotalTokens = scaleTokenCount(adjusted.TotalTokens, multiplier)
	}

	return adjusted, true
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
				{AboveTokens: 1000, Multiplier: 1.35},
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

func cloneUsage(usage *dto.Usage) dto.Usage {
	cloned := *usage
	if usage.InputTokensDetails != nil {
		details := *usage.InputTokensDetails
		cloned.InputTokensDetails = &details
	}
	return cloned
}

func scaleInputTokenDetails(details dto.InputTokenDetails, multiplier float64) dto.InputTokenDetails {
	details.CachedTokens = scaleTokenCount(details.CachedTokens, multiplier)
	details.CachedCreationTokens = scaleTokenCount(details.CachedCreationTokens, multiplier)
	details.TextTokens = scaleTokenCount(details.TextTokens, multiplier)
	details.AudioTokens = scaleTokenCount(details.AudioTokens, multiplier)
	details.ImageTokens = scaleTokenCount(details.ImageTokens, multiplier)
	return details
}

func scaleOutputTokenDetails(details dto.OutputTokenDetails, multiplier float64) dto.OutputTokenDetails {
	details.TextTokens = scaleTokenCount(details.TextTokens, multiplier)
	details.AudioTokens = scaleTokenCount(details.AudioTokens, multiplier)
	details.ImageTokens = scaleTokenCount(details.ImageTokens, multiplier)
	details.ReasoningTokens = scaleTokenCount(details.ReasoningTokens, multiplier)
	return details
}
