// Package graphics implements the com.apple.instruments.server.services.graphics.opengl
// DVT channel. It streams per-frame GPU performance counters from the device.
package graphics

import (
	"fmt"

	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
)

const channelIdentifier = `com.apple.instruments.server.services.graphics.opengl`

// Sample holds one GPU performance counter snapshot.
// Counter names are device/driver-dependent; the most common ones are listed here
// but the Raw map always contains the full set returned by the device.
type Sample struct {
	// FrameTime is the per-frame render time in milliseconds (if reported).
	FrameTime float64
	// GPUUtilization is the overall GPU busy percentage [0,100] (if reported).
	GPUUtilization float64
	// TilerUtilization is the tiler stage busy percentage (if reported).
	TilerUtilization float64
	// RendererUtilization is the renderer stage busy percentage (if reported).
	RendererUtilization float64
	// Raw contains every counter key/value as returned by the device.
	Raw map[string]any
}

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(channelIdentifier)
	if err != nil {
		return nil, err
	}
	return &Service{ch: svr.GetChannel(id)}, nil
}

type Service struct {
	ch *remotesvr.DTXChannel
}

// Start begins GPU sampling at the given interval (seconds).
// Pass 0 to sample as fast as possible.
// Returns a channel of decoded samples and an error channel.
func (s *Service) Start(intervalSeconds float64) (<-chan *Sample, <-chan error) {
	sampleCh := make(chan *Sample, 64)
	errCh := make(chan error, 1)

	err := s.ch.Send(
		"startSamplingAtTimeInterval:",
		new(remotesvr.ArgumentAux).Obj(intervalSeconds),
		true,
	)
	if err != nil {
		errCh <- err
		close(sampleCh)
		close(errCh)
		return sampleCh, errCh
	}

	// Consume the ack reply.
	if _, err = s.ch.Recv(nil); err != nil {
		errCh <- err
		close(sampleCh)
		close(errCh)
		return sampleCh, errCh
	}

	go func() {
		defer close(sampleCh)
		defer close(errCh)
		for {
			obj, err := s.ch.Recv(nil)
			if err != nil {
				errCh <- err
				return
			}
			sample, err := decodeSample(obj)
			if err != nil {
				continue
			}
			sampleCh <- sample
		}
	}()

	return sampleCh, errCh
}

// Stop ends GPU sampling. Fire-and-forget; no reply expected.
func (s *Service) Stop() error {
	return s.ch.Send("stopSampling", new(remotesvr.ArgumentAux), false)
}

// decodeSample converts the raw DTX payload into a Sample.
// The device sends a dict of counter name → float64 value.
func decodeSample(raw any) (*Sample, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("graphics: unexpected sample type %T", raw)
	}

	sample := &Sample{Raw: m}

	// Extract well-known keys when present; names vary by device/iOS version.
	for k, v := range m {
		f, ok := toFloat64(v)
		if !ok {
			continue
		}
		switch k {
		case "Frame Rate", "frameTime", "GPU Frame Time":
			sample.FrameTime = f
		case "GPU", "gpuUtilization", "GPU Utilization", "Device Utilization %":
			sample.GPUUtilization = f
		case "Tiler Utilization", "tilerUtilization":
			sample.TilerUtilization = f
		case "Renderer Utilization", "rendererUtilization":
			sample.RendererUtilization = f
		}
	}

	return sample, nil
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}
