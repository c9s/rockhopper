// Package dockermanage provides small helpers around ory/dockertest for
// spinning up throwaway containers in integration tests.
//
// It is intentionally free of any direct dependency on the moby/docker SDK; all
// container operations go through dockertest. Downstream projects that embed
// rockhopper can import this package (and its dialect-specific subpackages such
// as dockermysql) to drive their own migration integration tests.
package dockermanage

import (
	"errors"
	"fmt"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	// ManagedLabelKey marks containers created by this package so they can be
	// listed and purged in bulk. The value describes the container type, e.g.
	// "mysql".
	ManagedLabelKey = "com.c9s.rockhopper.dockermanage"

	// DefaultExpire is the safety TTL, in seconds, applied to every container.
	// Docker force-kills the container after this many seconds even if a test
	// crashes without purging it, preventing leaked containers.
	DefaultExpire = 600
)

// Manager wraps a dockertest.Pool to start and tear down containers.
type Manager struct {
	pool *dockertest.Pool
}

// NewManager connects to the local Docker daemon (honoring DOCKER_HOST). It
// returns an error when Docker is not reachable, which callers can treat as a
// signal to skip docker-backed tests.
func NewManager() (*Manager, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("connect to docker: %w", err)
	}

	if err := pool.Client.Ping(); err != nil {
		return nil, fmt.Errorf("ping docker daemon: %w", err)
	}

	return &Manager{pool: pool}, nil
}

// Pool exposes the underlying dockertest pool for advanced use.
func (m *Manager) Pool() *dockertest.Pool {
	return m.pool
}

// StartOptions configures a container start.
type StartOptions struct {
	// Repository is the image repository, e.g. "mysql". Required.
	Repository string

	// Tag is the image tag, e.g. "8.0". Defaults to "latest" when empty.
	Tag string

	// Env holds environment variables in KEY=VALUE form.
	Env []string

	// Labels are merged with the managed label applied by this package.
	Labels map[string]string

	// Name optionally names the container. Leave empty to let Docker assign one.
	Name string

	// ExpireSecond is the safety TTL, in seconds. 0 falls back to DefaultExpire.
	ExpireSecond uint
}

// Container is a running container started by the Manager.
type Container struct {
	pool     *dockertest.Pool
	resource *dockertest.Resource
}

// Start pulls the image if needed, runs the container, and applies the safety
// expiry. It does NOT wait for the service inside the container to become
// ready; call WaitReady for that.
func (m *Manager) Start(opts StartOptions) (*Container, error) {
	if opts.Repository == "" {
		return nil, errors.New("repository is required")
	}

	labels := map[string]string{ManagedLabelKey: opts.Repository}
	for k, v := range opts.Labels {
		labels[k] = v
	}

	resource, err := m.pool.RunWithOptions(&dockertest.RunOptions{
		Name:       opts.Name,
		Repository: opts.Repository,
		Tag:        opts.Tag,
		Env:        opts.Env,
		Labels:     labels,
	}, func(hc *docker.HostConfig) {
		// Remove the container once it stops, and never restart it, so a
		// crashed test does not leave a restarting container behind.
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("run container %s:%s: %w", opts.Repository, opts.Tag, err)
	}

	expire := opts.ExpireSecond
	if expire == 0 {
		expire = DefaultExpire
	}

	if err := resource.Expire(expire); err != nil {
		_ = m.pool.Purge(resource)
		return nil, fmt.Errorf("set container expiry: %w", err)
	}

	return &Container{pool: m.pool, resource: resource}, nil
}

// HostPort returns the host-side "host:port" mapping for a container port such
// as "3306/tcp".
func (c *Container) HostPort(portID string) string {
	return c.resource.GetHostPort(portID)
}

// GetPort returns just the host-side port number for a container port.
func (c *Container) GetPort(portID string) string {
	return c.resource.GetPort(portID)
}

// ID returns the container ID.
func (c *Container) ID() string {
	return c.resource.Container.ID
}

// WaitReady retries the given probe using the pool's exponential backoff until
// it succeeds or maxWait elapses. A zero maxWait keeps the pool's current
// setting.
func (c *Container) WaitReady(maxWait time.Duration, probe func() error) error {
	if probe == nil {
		return errors.New("probe must not be nil")
	}

	if maxWait > 0 {
		c.pool.MaxWait = maxWait
	}

	return c.pool.Retry(probe)
}

// Purge stops and removes the container. It is safe to call on a nil Container.
func (c *Container) Purge() error {
	if c == nil || c.resource == nil {
		return nil
	}

	return c.pool.Purge(c.resource)
}

// PurgeManaged removes every container started by this package (matched by the
// managed label), regardless of which Manager started it. It is useful in
// TestMain teardown to clean up containers leaked by a crashed run.
func (m *Manager) PurgeManaged() error {
	containers, err := m.pool.Client.ListContainers(docker.ListContainersOptions{
		All:     true,
		Filters: map[string][]string{"label": {ManagedLabelKey}},
	})
	if err != nil {
		return fmt.Errorf("list managed containers: %w", err)
	}

	var errs []error
	for _, c := range containers {
		if err := m.pool.Client.RemoveContainer(docker.RemoveContainerOptions{
			ID:            c.ID,
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
