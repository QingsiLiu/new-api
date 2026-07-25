package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestCreditsSpecPricingIsAuditableInTaskLogs(t *testing.T) {
	fromRelay := map[string]interface{}{}
	injectSpecPricingOther(fromRelay, &types.SpecPricingInfo{
		Priced:        true,
		Kind:          "video",
		Model:         "seedance-2.0",
		SpecKey:       "720p_no_video_input",
		UnitCredits:   "33",
		TotalCredits:  "165",
		PricingSource: "kie",
	})
	require.Equal(t, "33", fromRelay["spec_unit_credits"])
	require.Equal(t, "165", fromRelay["spec_total_credits"])
	require.Equal(t, "kie", fromRelay["pricing_source"])

	fromTask := map[string]interface{}{}
	injectTaskSpecPricingOther(fromTask, &model.TaskSpecPricing{
		Priced:        true,
		Kind:          "image",
		Model:         "seedream-5.0",
		SpecKey:       "default",
		UnitCredits:   "5.5",
		TotalCredits:  "5.5",
		PricingSource: "geili",
	})
	require.Equal(t, "5.5", fromTask["spec_unit_credits"])
	require.Equal(t, "5.5", fromTask["spec_total_credits"])
	require.Equal(t, "geili", fromTask["pricing_source"])
}
