package model

import (
	"strings"
)

// 简化的供应商映射规则
type defaultVendorRule struct {
	Pattern string
	Vendor  string
}

var defaultVendorRules = []defaultVendorRule{
	{Pattern: "openai", Vendor: "OpenAI"},
	{Pattern: "gpt", Vendor: "OpenAI"},
	{Pattern: "dall-e", Vendor: "OpenAI"},
	{Pattern: "whisper", Vendor: "OpenAI"},
	{Pattern: "o1", Vendor: "OpenAI"},
	{Pattern: "o3", Vendor: "OpenAI"},
	{Pattern: "claude", Vendor: "Anthropic"},
	{Pattern: "gemini", Vendor: "Google"},
	{Pattern: "moonshot", Vendor: "Moonshot"},
	{Pattern: "kimi", Vendor: "Moonshot"},
	{Pattern: "chatglm", Vendor: "智谱"},
	{Pattern: "glm-", Vendor: "智谱"},
	{Pattern: "qwen", Vendor: "阿里巴巴"},
	{Pattern: "deepseek", Vendor: "DeepSeek"},
	{Pattern: "abab", Vendor: "MiniMax"},
	{Pattern: "ernie", Vendor: "百度"},
	{Pattern: "hunyuan", Vendor: "腾讯"},
	{Pattern: "command", Vendor: "Cohere"},
	{Pattern: "@cf/", Vendor: "Cloudflare"},
	{Pattern: "360", Vendor: "360"},
	{Pattern: "yi", Vendor: "零一万物"},
	{Pattern: "spark", Vendor: "讯飞"},
	{Pattern: "jina", Vendor: "Jina"},
	{Pattern: "mistral", Vendor: "Mistral"},
	{Pattern: "grok", Vendor: "xAI"},
	{Pattern: "llama", Vendor: "Meta"},
	{Pattern: "doubao", Vendor: "字节跳动"},
	{Pattern: "kling", Vendor: "快手"},
	{Pattern: "jimeng", Vendor: "即梦"},
	{Pattern: "vidu", Vendor: "Vidu"},
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"零一万物":       "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Ollama",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"Vidu":       "Vidu",
	"微软":         "AzureAI",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		// 匹配供应商
		vendorID := 0
		modelLower := strings.ToLower(modelName)
		for _, rule := range defaultVendorRules {
			if strings.Contains(modelLower, rule.Pattern) {
				vendorID = getOrCreateVendor(rule.Vendor, vendorMap)
				break
			}
		}

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
