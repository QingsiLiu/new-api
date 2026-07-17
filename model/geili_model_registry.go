package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ModelRegistry Geili 自有：模型公开落地页（SLP）注册表。
// 设计原则：独立表、独立文件，不侵入上游 models 表（减少上游合并冲突面，
// 见主仓 docs/superpowers/specs/2026-07-13-kie-parity-master-plan.md §5 风险表）。
// 以 model_name 关联计费模型（= AsyncSpecPricing 的 key）；slug 供公开层 URL 使用。
type ModelRegistry struct {
	Id              int    `json:"id"`
	ModelName       string `json:"model_name" gorm:"size:128;not null;uniqueIndex"`
	Slug            string `json:"slug" gorm:"size:128;not null;uniqueIndex"`
	DisplayNameZh   string `json:"display_name_zh" gorm:"size:128"`
	DisplayNameEn   string `json:"display_name_en" gorm:"size:128"`
	Aliases         string `json:"aliases" gorm:"type:text"`    // JSON 数组：热门别名（SEO 关键词）
	Vendor          string `json:"vendor" gorm:"size:64;index"` // 厂商 slug：google/openai/bytedance…
	VendorDisplayZh string `json:"vendor_display_zh" gorm:"size:64"`
	VendorDisplayEn string `json:"vendor_display_en" gorm:"size:64"`
	Modality        string `json:"modality" gorm:"size:16;index"`      // image | video | text
	TextCategory    string `json:"text_category" gorm:"size:16;index"` // gpt | claude | gemini | grok
	CapabilityTags  string `json:"capability_tags" gorm:"type:text"`   // JSON 数组：text-to-image 等（market 能力页）
	OfficialPrice   string `json:"official_price" gorm:"type:text"`    // JSON：官方价（对比列/折扣百分比）
	ParamsSchema    string `json:"params_schema" gorm:"type:text"`     // JSON Schema：Playground 表单（M2 消费）
	ExampleParams   string `json:"example_params" gorm:"type:text"`    // JSON：示例参数
	FaqZh           string `json:"faq_zh" gorm:"type:text"`            // JSON 数组 [{q,a}]
	FaqEn           string `json:"faq_en" gorm:"type:text"`
	SeoZh           string `json:"seo_zh" gorm:"type:text"` // Markdown 长文
	SeoEn           string `json:"seo_en" gorm:"type:text"`
	// 注意：不加 gorm default 标签——否则 Create 时零值 false 会被 DB 默认值顶掉（GORM 行为）。
	// Enabled 一律由 Upsert/种子导入显式赋值。
	Enabled     bool  `json:"enabled" gorm:"index"`
	CreatedTime int64 `json:"created_time" gorm:"bigint"`
	UpdatedTime int64 `json:"updated_time" gorm:"bigint"`
}

func (ModelRegistry) TableName() string {
	return "model_registry"
}

func GetEnabledModelRegistries() ([]ModelRegistry, error) {
	var entries []ModelRegistry
	err := DB.Where("enabled = ?", true).Order("modality desc, model_name asc").Find(&entries).Error
	return entries, err
}

func GetAllModelRegistries() ([]ModelRegistry, error) {
	var entries []ModelRegistry
	err := DB.Order("modality desc, model_name asc").Find(&entries).Error
	return entries, err
}

func GetModelRegistryBySlug(slug string) (*ModelRegistry, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil, errors.New("slug 不能为空")
	}
	var entry ModelRegistry
	err := DB.Where("slug = ? AND enabled = ?", slug, true).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// UpsertModelRegistry 按 model_name 幂等写入（运营录入/种子导入共用）。
func UpsertModelRegistry(entry *ModelRegistry) error {
	entry.ModelName = strings.TrimSpace(entry.ModelName)
	entry.Slug = strings.TrimSpace(strings.ToLower(entry.Slug))
	if entry.ModelName == "" || entry.Slug == "" {
		return errors.New("model_name 与 slug 必填")
	}
	var existing ModelRegistry
	err := DB.Where("model_name = ?", entry.ModelName).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DB.Create(entry).Error
	}
	if err != nil {
		return err
	}
	entry.Id = existing.Id
	entry.CreatedTime = existing.CreatedTime
	return DB.Model(&existing).Select("*").Omit("id", "created_time").Updates(entry).Error
}

func DeleteModelRegistryByModelName(modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.New("model_name 不能为空")
	}
	return DB.Where("model_name = ?", modelName).Delete(&ModelRegistry{}).Error
}
