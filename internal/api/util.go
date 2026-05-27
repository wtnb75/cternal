package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/wtnb75/cternal/internal/recorder"
)

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func marshalCast(width, height int, events []recorder.Event) ([]byte, error) {
	hdr := recorder.Header{
		Width:         width,
		Height:        height,
		IdleTimeLimit: 1.0, // clamp idle gaps to 1 s for snappy playback
	}
	data, err := recorder.Marshal(hdr, events)
	if err != nil {
		return nil, fmt.Errorf("marshal cast: %w", err)
	}
	return data, nil
}
