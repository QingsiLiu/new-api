package setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSensitiveWordsCoverAIGCReviewCategories(t *testing.T) {
	for _, required := range []string{
		"csam",
		"pornography",
		"graphic violence",
		"hate speech",
		"deepfake porn",
		"copyright infringement",
		"儿童色情",
		"色情内容",
		"暴力血腥",
		"仇恨言论",
		"深度伪造色情",
		"侵犯版权",
	} {
		require.Contains(t, SensitiveWords, required)
	}
}
