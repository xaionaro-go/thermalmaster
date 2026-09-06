package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
)

type closeResultTransport struct {
	err   error
	calls int
}

func (t *closeResultTransport) Control(uint8, uint8, uint16, uint16, []byte) (int, error) {
	return 0, nil
}

func (t *closeResultTransport) Close() error {
	t.calls++
	return t.err
}

func TestJoinDeviceCloseErrorPreservesPhotoAndCleanupFailures(t *testing.T) {
	primaryErr := errors.New("encoding photo failed")
	cleanupErr := errors.New("camera restoration failed")
	tests := []struct {
		name       string
		primaryErr error
		cleanupErr error
	}{
		{name: "both succeed"},
		{name: "primary only", primaryErr: primaryErr},
		{name: "cleanup only", cleanupErr: cleanupErr},
		{name: "primary and cleanup", primaryErr: primaryErr, cleanupErr: cleanupErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &closeResultTransport{err: tt.cleanupErr}
			device := thermalmaster.NewDeviceWithTransport(transport, thermalmaster.ConfigP3)
			resultErr := tt.primaryErr

			joinDeviceCloseError(&resultErr, device)

			if tt.primaryErr != nil {
				assert.ErrorIs(t, resultErr, tt.primaryErr)
			}
			if tt.cleanupErr != nil {
				assert.ErrorIs(t, resultErr, tt.cleanupErr)
			}
			if tt.primaryErr == nil && tt.cleanupErr == nil {
				assert.NoError(t, resultErr)
			}
			assert.Equal(t, 1, transport.calls)
		})
	}
}
