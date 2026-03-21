package thermalmaster

import (
	"context"
	"time"
)

// StartHeartbeat starts the heartbeat protocol by sending the initial
// heartbeat start command.
func (d *Device) StartHeartbeat() error {
	return d.SendCommandNoResponse(CmdHeartbeatStart)
}

// SendHeartbeat sends a single heartbeat keepalive to the camera.
func (d *Device) SendHeartbeat() error {
	return d.SendCommandNoResponse(CmdHeartbeatSend)
}

// RunHeartbeatLoop sends periodic heartbeat keepalives until ctx is cancelled.
// This should be called as a goroutine. It returns nil when the context is
// done, or an error if a heartbeat send fails.
func (d *Device) RunHeartbeatLoop(
	ctx context.Context,
	interval time.Duration,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := d.SendHeartbeat(); err != nil {
				return err
			}
		}
	}
}
