package model

import "testing"

func TestDefaultVendorMappingPrefersOpenAIForCodexSparkOpenAIModel(t *testing.T) {
	const modelName = "gpt-5.3-codex-spark-openai-compact"
	vendorMap := map[int]*Vendor{
		1: {Id: 1, Name: "OpenAI"},
		2: {Id: 2, Name: "讯飞"},
	}

	for range 200 {
		metaMap := map[string]*Model{}
		initDefaultVendorMapping(metaMap, vendorMap, []AbilityWithChannel{
			{Ability: Ability{Model: modelName, Enabled: true}},
		})

		meta, ok := metaMap[modelName]
		if !ok {
			t.Fatalf("expected metadata for %s", modelName)
		}
		if meta.VendorID != 1 {
			t.Fatalf("expected %s to map to OpenAI vendor 1, got vendor %d", modelName, meta.VendorID)
		}
	}
}

func TestDefaultVendorMappingKeepsSparkDeskOnXunfei(t *testing.T) {
	const modelName = "SparkDesk-v4.0"
	vendorMap := map[int]*Vendor{
		1: {Id: 1, Name: "OpenAI"},
		2: {Id: 2, Name: "讯飞"},
	}
	metaMap := map[string]*Model{}

	initDefaultVendorMapping(metaMap, vendorMap, []AbilityWithChannel{
		{Ability: Ability{Model: modelName, Enabled: true}},
	})

	meta, ok := metaMap[modelName]
	if !ok {
		t.Fatalf("expected metadata for %s", modelName)
	}
	if meta.VendorID != 2 {
		t.Fatalf("expected %s to map to Xunfei vendor 2, got vendor %d", modelName, meta.VendorID)
	}
}
