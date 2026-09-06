package cliflags

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xaionaro-go/thermalmaster/pkg/thermalmaster"
)

func TestSetupOpenedCameraJoinsPrimaryAndCloseErrors(t *testing.T) {
	primaryErr := errors.New("reading device info failed")
	closeErr := errors.New("closing setup device failed")
	transport := &setupFailureTransport{
		controlErr: primaryErr,
		closeErr:   closeErr,
	}
	dev := thermalmaster.NewDeviceWithTransport(transport, thermalmaster.ConfigP3)
	cfg := Config{}

	gotDev, _, err := cfg.setupOpenedCamera(dev)

	assert.Nil(t, gotDev)
	assert.ErrorIs(t, err, primaryErr)
	assert.ErrorIs(t, err, closeErr)
	assert.Equal(t, 1, transport.closeCalls)
}

type setupFailureTransport struct {
	controlErr error
	closeErr   error
	closeCalls int
}

func (t *setupFailureTransport) Control(
	uint8,
	uint8,
	uint16,
	uint16,
	[]byte,
) (int, error) {
	return 0, t.controlErr
}

func (t *setupFailureTransport) Close() error {
	t.closeCalls++
	return t.closeErr
}
