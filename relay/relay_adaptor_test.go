package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	tasksora "github.com/QuantumNous/new-api/relay/channel/task/sora"
	"github.com/stretchr/testify/require"
)

func TestJimengOpenAIVideoChannelTypeUsesSoraTaskAdaptor(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("59"))
	require.IsType(t, &tasksora.TaskAdaptor{}, adaptor)

	_, ok := adaptor.(channel.OpenAIVideoConverter)
	require.True(t, ok)
}
