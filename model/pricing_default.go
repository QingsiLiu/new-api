package model

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
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

var defaultChannelVendorRules = map[int]string{
	constant.ChannelTypeOpenAI:            "OpenAI",
	constant.ChannelTypeOpenAIMax:         "OpenAI",
	constant.ChannelTypeCodex:             "OpenAI",
	constant.ChannelTypeSora:              "OpenAI",
	constant.ChannelTypeAzure:             "微软",
	constant.ChannelTypeAnthropic:         "Anthropic",
	constant.ChannelTypeGemini:            "Google",
	constant.ChannelTypeVertexAi:          "Google",
	constant.ChannelTypeMoonshot:          "Moonshot",
	constant.ChannelTypeZhipu:             "智谱",
	constant.ChannelTypeZhipu_v4:          "智谱",
	constant.ChannelTypeAli:               "阿里巴巴",
	constant.ChannelTypeDeepSeek:          "DeepSeek",
	constant.ChannelTypeMiniMax:           "MiniMax",
	constant.ChannelTypeBaidu:             "百度",
	constant.ChannelTypeBaiduV2:           "百度",
	constant.ChannelTypeXunfei:            "讯飞",
	constant.ChannelTypeTencent:           "腾讯",
	constant.ChannelTypeCohere:            "Cohere",
	constant.ChannelType360:               "360",
	constant.ChannelTypeLingYiWanWu:       "零一万物",
	constant.ChannelTypeJina:              "Jina",
	constant.ChannelTypeMistral:           "Mistral",
	constant.ChannelTypeXai:               "xAI",
	constant.ChannelTypeVolcEngine:        "字节跳动",
	constant.ChannelTypeDoubaoVideo:       "字节跳动",
	constant.ChannelTypeKling:             "快手",
	constant.ChannelTypeJimeng:            "即梦",
	constant.ChannelTypeJimengOpenAIVideo: "即梦",
	constant.ChannelTypeVidu:              "Vidu",
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	channelVendorByModel := defaultChannelVendorByModel(enableAbilities)
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		vendorID := 0
		if vendorName := channelVendorByModel[modelName]; vendorName != "" {
			vendorID = getOrCreateVendor(vendorName, vendorMap)
		}
		if vendorID == 0 {
			vendorID = inferDefaultVendorIDFromModelName(modelName, vendorMap)
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

func defaultChannelVendorByModel(enableAbilities []AbilityWithChannel) map[string]string {
	vendorByModel := make(map[string]string)
	conflictedModels := make(map[string]struct{})
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, conflicted := conflictedModels[modelName]; conflicted {
			continue
		}
		vendorName := defaultChannelVendorRules[ability.ChannelType]
		if vendorName == "" {
			continue
		}
		if existing, exists := vendorByModel[modelName]; exists && existing != vendorName {
			delete(vendorByModel, modelName)
			conflictedModels[modelName] = struct{}{}
			continue
		}
		vendorByModel[modelName] = vendorName
	}
	return vendorByModel
}

func inferDefaultVendorIDFromModelName(modelName string, vendorMap map[int]*Vendor) int {
	modelLower := strings.ToLower(modelName)
	for _, rule := range defaultVendorRules {
		if strings.Contains(modelLower, rule.Pattern) {
			return getOrCreateVendor(rule.Vendor, vendorMap)
		}
	}
	return 0
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
