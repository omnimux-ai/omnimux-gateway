package elevenlabs

import (
	"fmt"
	"net/url"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
)

func GetRequestURL(info *relaycommon.RelayInfo, voiceID string, outputFormat string) (string, error) {
	baseUrl := ""
	if info != nil && info.ChannelMeta != nil {
		baseUrl = info.ChannelBaseUrl
	}
	if baseUrl == "" {
		baseUrl = channelconstant.GetChannelBaseURL(channelconstant.ChannelTypeElevenLabs)
	}
	if baseUrl == "" {
		baseUrl = "https://api.elevenlabs.io"
	}

	if voiceID == "" {
		voiceID = DefaultVoice
	}

	if info != nil && info.RelayMode != constant.RelayModeAudioSpeech {
		return "", fmt.Errorf("elevenlabs: unsupported relay mode: %d", info.RelayMode)
	}

	targetURL := fmt.Sprintf("%s/v1/text-to-speech/%s", baseUrl, url.PathEscape(voiceID))
	if outputFormat != "" {
		targetURL = fmt.Sprintf("%s?output_format=%s", targetURL, url.QueryEscape(outputFormat))
	}
	return targetURL, nil
}
