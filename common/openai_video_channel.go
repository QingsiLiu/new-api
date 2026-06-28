package common

import "github.com/QuantumNous/new-api/constant"

func IsOpenAIVideoChannelType(channelType int) bool {
	switch channelType {
	case constant.ChannelTypeSora, constant.ChannelTypeJimengOpenAIVideo:
		return true
	default:
		return false
	}
}
