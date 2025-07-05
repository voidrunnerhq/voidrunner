package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/mock"
)

// MockSDKClient is a mock implementation of SDKClientInterface for testing.
type MockSDKClient struct {
	mock.Mock
}

func (m *MockSDKClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	args := m.Called(ctx, imageID)
	// Handle cases where ImageInspect might be nil
	var inspect types.ImageInspect
	if args.Get(0) != nil {
		inspect = args.Get(0).(types.ImageInspect)
	}
	var raw []byte
	if args.Get(1) != nil {
		raw = args.Get(1).([]byte)
	}
	return inspect, raw, args.Error(2)
}

func (m *MockSDKClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, refStr, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockSDKClient) ContainerCreate(
	ctx context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	platform *v1.Platform,
	containerName string,
) (container.CreateResponse, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, platform, containerName)
	// Handle cases where CreateResponse might be zero struct
	var resp container.CreateResponse
	if args.Get(0) != nil {
		resp = args.Get(0).(container.CreateResponse)
	}
	return resp, args.Error(1)
}

func (m *MockSDKClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockSDKClient) ContainerWait(
	ctx context.Context,
	containerID string,
	condition container.WaitCondition,
) (<-chan container.WaitResponse, <-chan error) {
	args := m.Called(ctx, containerID, condition)
	var respCh <-chan container.WaitResponse
	var errCh <-chan error

	if args.Get(0) != nil {
		respCh = args.Get(0).(<-chan container.WaitResponse)
	}
	if args.Get(1) != nil {
		errCh = args.Get(1).(<-chan error)
	}
	return respCh, errCh
}

func (m *MockSDKClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, containerID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockSDKClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockSDKClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockSDKClient) Ping(ctx context.Context) (types.Ping, error) {
	args := m.Called(ctx)
	// Handle cases where Ping might be zero struct
	var ping types.Ping
	if args.Get(0) != nil {
		ping = args.Get(0).(types.Ping)
	}
	return ping, args.Error(1)
}

// Ensure MockSDKClient implements SDKClientInterface (compile-time check)
var _ SDKClientInterface = (*MockSDKClient)(nil)
