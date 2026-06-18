// Package dockermysql starts throwaway MySQL containers for integration tests
// on top of the dockermanage helpers. It applies opinionated MySQL defaults and
// builds a go-sql-driver/mysql DSN that is ready to hand to rockhopper.
package dockermysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	// Register the MySQL driver so the readiness probe can ping the container.
	_ "github.com/go-sql-driver/mysql"

	"github.com/c9s/rockhopper/v2/pkg/dockermanage"
)

const (
	// DefaultImage is the default MySQL image repository.
	DefaultImage = "mysql"

	// DefaultTag is the default MySQL image tag.
	DefaultTag = "8.0"

	// DefaultDatabase is the default database created in the container.
	DefaultDatabase = "rockhopper_test"

	// DefaultUser is the default user. Only the root user is supported, because
	// MYSQL_ROOT_PASSWORD is what the official image guarantees.
	DefaultUser = "root"

	// DefaultPassword is the default root password.
	DefaultPassword = "rockhopper" // pragma: allowlist secret

	// DefaultMaxWait bounds how long Start waits for MySQL to accept
	// connections. MySQL's first boot initializes the data directory, which can
	// take a while on a cold image.
	DefaultMaxWait = 120 * time.Second

	containerPort = "3306/tcp"
)

type config struct {
	image    string
	tag      string
	database string
	password string
	maxWait  time.Duration
}

// Option configures the MySQL container.
type Option func(*config)

// WithImage overrides the image repository (default "mysql").
func WithImage(image string) Option {
	return func(c *config) {
		if image != "" {
			c.image = image
		}
	}
}

// WithTag overrides the image tag (default "8.0").
func WithTag(tag string) Option {
	return func(c *config) {
		if tag != "" {
			c.tag = tag
		}
	}
}

// WithDatabase overrides the database name created in the container.
func WithDatabase(database string) Option {
	return func(c *config) {
		if database != "" {
			c.database = database
		}
	}
}

// WithPassword overrides the root password.
func WithPassword(password string) Option {
	return func(c *config) {
		if password != "" {
			c.password = password
		}
	}
}

// WithMaxWait overrides how long Start waits for readiness.
func WithMaxWait(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.maxWait = d
		}
	}
}

// Instance is a running MySQL container with helpers to build a DSN.
type Instance struct {
	Container *dockermanage.Container
	Database  string
	User      string
	Password  string

	hostPort string // host:port mapping for the exposed MySQL port
}

// DSN returns a go-sql-driver/mysql DSN with parseTime=true, which rockhopper
// requires so DATETIME/TIMESTAMP columns scan into time.Time.
func (i *Instance) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true",
		i.User, i.Password, i.hostPort, i.Database,
	)
}

// Purge stops and removes the container. It is safe to call on a nil Instance.
func (i *Instance) Purge() error {
	if i == nil || i.Container == nil {
		return nil
	}

	return i.Container.Purge()
}

// Start launches a MySQL container and blocks until it accepts connections.
// On any failure after the container has started, the container is purged
// before returning.
func Start(manager *dockermanage.Manager, opts ...Option) (_ *Instance, retErr error) {
	if manager == nil {
		return nil, errors.New("manager must not be nil")
	}

	cfg := &config{
		image:    DefaultImage,
		tag:      DefaultTag,
		database: DefaultDatabase,
		password: DefaultPassword,
		maxWait:  DefaultMaxWait,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	container, err := manager.Start(dockermanage.StartOptions{
		Repository: cfg.image,
		Tag:        cfg.tag,
		Env: []string{
			"MYSQL_ROOT_PASSWORD=" + cfg.password,
			"MYSQL_DATABASE=" + cfg.database,
		},
		Labels: map[string]string{dockermanage.ManagedLabelKey: "mysql"},
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, container.Purge())
		}
	}()

	inst := &Instance{
		Container: container,
		Database:  cfg.database,
		User:      DefaultUser,
		Password:  cfg.password,
		hostPort:  container.HostPort(containerPort),
	}

	if err := container.WaitReady(cfg.maxWait, func() error {
		db, err := sql.Open("mysql", inst.DSN())
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()

		return db.Ping()
	}); err != nil {
		return nil, fmt.Errorf("wait for mysql readiness: %w", err)
	}

	return inst, nil
}
