//go:build test_helpers
// +build test_helpers

// This file defines an interface for the Docker SDK client's methods that are used by our docker.Client.
// It's intended for use in tests with testify/mock.
// The build tag 'test_helpers' ensures this file is only compiled when that tag is active,
// preventing it from being part of the main build if an interface for the SDK is not desired in the main codebase.
// Alternatively, place this in a test-only package or directly in client_test.go if preferred.

package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// SDKClientInterface defines the subset of methods from the Docker SDK client
// (github.com/docker/docker/client.Client) that our internal docker.Client uses.
// This allows for mocking these interactions in unit tests.
type SDKClientInterface interface {
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	Ping(ctx context.Context) (types.Ping, error)
	// Add other methods from client.Client as they are used by your wrapper
}

// Ensure client.Client satisfies this interface (compile-time check)
// This line would ideally be in a non-test_helpers file if the interface was broadly used,
// or within client_test.go. For now, it's a conceptual check.
// var _ SDKClientInterface = (*client.Client)(nil)
