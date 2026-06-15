package app

import (
	"context"
	"strings"
)

func (s *SignalingService) UpdateRoomLimits(ctx context.Context, roomID string, extendSeconds int64, newMaxPeers int32) error {
	return nil
}

func (s *SignalingService) MutateSdpQuality(rawSdp string, lowBandwidth bool) string {
	lines := strings.Split(rawSdp, "\r\n")
	var mutatedLines []string
	for _, line := range lines {
		mutatedLines = append(mutatedLines, line)
		if strings.HasPrefix(line, "m=video") && lowBandwidth {
			mutatedLines = append(mutatedLines, "b=AS:100")
		}
	}
	return strings.Join(mutatedLines, "\r\n")
}
