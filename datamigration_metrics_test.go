package rockhopper

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterDataMigrationMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterDataMigrationMetrics(reg))

	// registering the same collectors into the same registry again is treated as
	// success (AlreadyRegisteredError is swallowed).
	require.NoError(t, RegisterDataMigrationMetrics(reg))
}

// TestDataMigrationMetrics_Recorded drives a full backfill through a registry
// and asserts the applied-version, duration and progress metrics were recorded.
func TestDataMigrationMetrics_Recorded(t *testing.T) {
	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterDataMigrationMetrics(reg))

	ctx := context.Background()
	db := openDataMigrationTestDB(t)
	seedUsers(t, db, 25)

	const version = int64(1700000000000030)
	mig := &progressMigrator{backfillMigrator{table: "users", batchSize: 10}}
	dm := &DataMigration{Package: DefaultPackageName, Version: version, Migrator: mig}

	require.NoError(t, RunDataMigration(ctx, db, dm))

	vl := versionLabel(version)

	// applied version gauge is set to this migration's version on completion.
	assert.Equal(t, float64(version),
		testutil.ToFloat64(dataMigrationAppliedVersion.WithLabelValues(DefaultPackageName)))

	// progress reached 100% with the full row count.
	assert.Equal(t, 100.0,
		testutil.ToFloat64(dataMigrationProgressPercent.WithLabelValues(DefaultPackageName, vl)))
	assert.Equal(t, 25.0,
		testutil.ToFloat64(dataMigrationProgressTotal.WithLabelValues(DefaultPackageName, vl)))

	// the last-updated timestamp was stamped within a sane window of now.
	updated := testutil.ToFloat64(dataMigrationUpdatedTimestamp.WithLabelValues(DefaultPackageName, vl))
	nowMs := float64(time.Now().UnixMilli())
	assert.InDelta(t, nowMs, updated, float64(60_000), "updated timestamp is recent")

	// Plan ran once and every Batch was timed: 1 plan + 3 batch observations.
	// Filter by this migration's version because the collectors are process-wide
	// globals shared with every other data-migration test.
	assert.Equal(t, 1, countHistogram(t, reg, "rockhopper_data_migration_plan_duration_milliseconds", vl))
	assert.Equal(t, 3, countHistogram(t, reg, "rockhopper_data_migration_batch_duration_milliseconds", vl))
}

func TestDurationMillis(t *testing.T) {
	assert.Equal(t, 1500.0, durationMillis(1500*time.Millisecond))
	assert.Equal(t, 0.5, durationMillis(500*time.Microsecond))
}

// countHistogram returns the sample count of the named histogram for the series
// whose "version" label equals versionLabel.
func countHistogram(t *testing.T, reg *prometheus.Registry, name, versionLabel string) int {
	t.Helper()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	total := 0
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}

		for _, m := range mf.GetMetric() {
			h := m.GetHistogram()
			if h == nil {
				continue
			}

			for _, lp := range m.GetLabel() {
				if lp.GetName() == "version" && lp.GetValue() == versionLabel {
					total += int(h.GetSampleCount())
				}
			}
		}
	}

	return total
}

// TestDataMigrationMetricNames guards the metric name prefix and unit
// convention (milliseconds, never seconds) that operators depend on.
func TestDataMigrationMetricNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	require.NoError(t, RegisterDataMigrationMetrics(reg))

	mfs, err := reg.Gather()
	require.NoError(t, err)

	require.NotEmpty(t, mfs)
	for _, mf := range mfs {
		name := mf.GetName()
		assert.True(t, strings.HasPrefix(name, "rockhopper_"), "metric %q must start with rockhopper_", name)
		assert.False(t, strings.HasSuffix(name, "_seconds"), "metric %q must not use seconds", name)
	}
}
