package setting

import "strings"

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

//var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// SensitiveWords 敏感词
// var SensitiveWords []string
var SensitiveWords = []string{
	"test_sensitive",
	"csam",
	"child pornography",
	"sexualized minor",
	"pornography",
	"nsfw",
	"non-consensual intimate",
	"graphic gore",
	"graphic violence",
	"beheading",
	"dismemberment",
	"torture",
	"hate speech",
	"terrorist propaganda",
	"deepfake porn",
	"impersonate a real person",
	"identity theft",
	"phishing kit",
	"credential theft",
	"ransomware",
	"malware payload",
	"bomb making",
	"doxxing",
	"copyright infringement",
	"trademark infringement",
	"remove watermark",
	"儿童色情",
	"未成年人色情",
	"色情内容",
	"裸露内容",
	"未经同意的私密影像",
	"暴力血腥",
	"肢解",
	"斩首",
	"酷刑",
	"仇恨言论",
	"恐怖主义宣传",
	"深度伪造色情",
	"冒充真实人物",
	"身份盗用",
	"钓鱼套件",
	"凭据窃取",
	"勒索软件",
	"恶意软件载荷",
	"炸弹制作",
	"人肉搜索",
	"侵犯版权",
	"侵犯商标",
	"去除水印",
}

func SensitiveWordsToString() string {
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsFromString(s string) {
	SensitiveWords = []string{}
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			SensitiveWords = append(SensitiveWords, w)
		}
	}
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

//func ShouldCheckCompletionSensitive() bool {
//	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
//}
