package service

import "strings"

const glmTokenBillingMultiplierThreshold = 1000

func applyGLMTokenBillingMultiplier(modelName string, tokenCount int) int {
	if tokenCount <= glmTokenBillingMultiplierThreshold {
		return tokenCount
	}

	modelName = strings.ToLower(modelName)
	if strings.Contains(modelName, "glm-5.1") || strings.Contains(modelName, "glm-5.2") {
		return tokenCount * 2
	}

	return tokenCount
}
