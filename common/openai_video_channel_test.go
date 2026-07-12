package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestJimengOpenAIVideoChannelTypeUsesOpenAIVideoEndpoint(t *testing.T) {
	require.Equal(t, 102, constant.ChannelTypeJimengOpenAIVideo) // 2026-07-12 改号避让上游(原59)
	require.Equal(t, "JimengOpenAIVideo", constant.GetChannelTypeName(constant.ChannelTypeJimengOpenAIVideo))
	require.True(t, IsOpenAIVideoChannelType(constant.ChannelTypeJimengOpenAIVideo))

	endpointTypes := GetEndpointTypesByChannelType(constant.ChannelTypeJimengOpenAIVideo, "video-ds-2.0-fast")
	require.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, endpointTypes)

	apiType, ok := ChannelType2APIType(constant.ChannelTypeJimengOpenAIVideo)
	require.True(t, ok)
	require.Equal(t, constant.APITypeOpenAI, apiType)
}

func TestSoraChannelTypeRemainsAvailableForSoraOnly(t *testing.T) {
	require.Equal(t, "Sora", constant.GetChannelTypeName(constant.ChannelTypeSora))
	require.True(t, IsOpenAIVideoChannelType(constant.ChannelTypeSora))
}
