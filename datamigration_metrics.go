package rockhopper

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Data-migration Prometheus metrics. The collectors are created but not
// registered here: a library must not register into a global registry on
// import. Call RegisterDataMigrationMetrics with your registry (commonly
// prometheus.DefaultRegisterer) to expose them. Updating an unregistered
// collector is a harmless no-op, so the runner always records values and the
// cost of leaving metrics unregistered is only that they are not scraped.
//
// All durations are reported in milliseconds (suffix _milliseconds), per this
// project's convention of not using seconds for data-migration timings.
var (
	// dataMigrationAppliedVersion is the version id of the most recently
	// completed data migration in a package. Since data migrations run in
	// ascending version order, this only moves forward.
	dataMigrationAppliedVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rockhopper_data_migration_applied_version",
		Help: "Version id of the most recently completed data migration, per package.",
	}, []string{"package"})

	// dataMigrationUpdatedTimestamp is the Unix time in milliseconds of the last
	// committed batch of a data migration — i.e. when its state row was last
	// advanced. For a running migration this tracks liveness.
	dataMigrationUpdatedTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rockhopper_data_migration_updated_timestamp_milliseconds",
		Help: "Unix timestamp in milliseconds of the last committed batch of a data migration.",
	}, []string{"package", "version"})

	// dataMigrationPlanDuration observes how long Plan took, in milliseconds.
	dataMigrationPlanDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rockhopper_data_migration_plan_duration_milliseconds",
		Help:    "Wall-clock duration of a data migration's Plan call, in milliseconds.",
		Buckets: dataMigrationDurationBuckets,
	}, []string{"package", "version"})

	// dataMigrationBatchDuration observes how long each Batch took, in
	// milliseconds.
	dataMigrationBatchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rockhopper_data_migration_batch_duration_milliseconds",
		Help:    "Wall-clock duration of a single data migration Batch call, in milliseconds.",
		Buckets: dataMigrationDurationBuckets,
	}, []string{"package", "version"})

	// dataMigrationProgressPercent is the migrator-reported completion percentage
	// (0-100) of a data migration. Present only for migrators that implement
	// ProgressReporter and report a known Total.
	dataMigrationProgressPercent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rockhopper_data_migration_progress_percent",
		Help: "Migrator-reported completion percentage (0-100) of a data migration.",
	}, []string{"package", "version"})

	// dataMigrationProgressCompleted is the migrator-reported units of work done.
	dataMigrationProgressCompleted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rockhopper_data_migration_progress_completed",
		Help: "Migrator-reported units of work completed by a data migration.",
	}, []string{"package", "version"})

	// dataMigrationProgressTotal is the migrator-reported total units of work.
	dataMigrationProgressTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rockhopper_data_migration_progress_total",
		Help: "Migrator-reported total units of work for a data migration.",
	}, []string{"package", "version"})

	// dataMigrationETA is the framework-estimated time remaining, in
	// milliseconds, computed from the rate observed in the current run.
	dataMigrationETA = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rockhopper_data_migration_eta_milliseconds",
		Help: "Estimated time remaining for a data migration, in milliseconds.",
	}, []string{"package", "version"})
)

// dataMigrationDurationBuckets covers ~1ms to ~9 minutes with exponential
// spacing, so both fast in-memory batches and slow throttled ones land in a
// meaningful bucket.
var dataMigrationDurationBuckets = prometheus.ExponentialBuckets(1, 2, 20)

// dataMigrationCollectors is the full set of collectors, used by
// RegisterDataMigrationMetrics.
func dataMigrationCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		dataMigrationAppliedVersion,
		dataMigrationUpdatedTimestamp,
		dataMigrationPlanDuration,
		dataMigrationBatchDuration,
		dataMigrationProgressPercent,
		dataMigrationProgressCompleted,
		dataMigrationProgressTotal,
		dataMigrationETA,
	}
}

// RegisterDataMigrationMetrics registers the data-migration collectors with the
// given registry so they are exposed on scrape. It is safe to call once at
// startup, e.g. RegisterDataMigrationMetrics(prometheus.DefaultRegisterer). A
// prometheus.AlreadyRegisteredError for an individual collector is treated as
// success, so repeated calls (or sharing a registry with another rockhopper
// instance) do not fail.
func RegisterDataMigrationMetrics(r prometheus.Registerer) error {
	for _, c := range dataMigrationCollectors() {
		if err := r.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
				continue
			}

			return err
		}
	}

	return nil
}

// versionLabel renders a version id as a metric label value.
func versionLabel(version int64) string {
	return strconv.FormatInt(version, 10)
}

// durationMillis converts a duration to fractional milliseconds for reporting.
func durationMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// observePlanDuration records how long Plan took.
func observePlanDuration(dm *DataMigration, d time.Duration) {
	dataMigrationPlanDuration.WithLabelValues(dm.Package, versionLabel(dm.Version)).Observe(durationMillis(d))
}

// observeBatchDuration records how long a single Batch took.
func observeBatchDuration(dm *DataMigration, d time.Duration) {
	dataMigrationBatchDuration.WithLabelValues(dm.Package, versionLabel(dm.Version)).Observe(durationMillis(d))
}

// recordBatchCommitted stamps the last-updated timestamp after a batch commits.
func recordBatchCommitted(dm *DataMigration, at time.Time) {
	dataMigrationUpdatedTimestamp.WithLabelValues(dm.Package, versionLabel(dm.Version)).Set(float64(at.UnixMilli()))
}

// recordCompleted advances the applied-version gauge when a migration finishes.
func recordCompleted(dm *DataMigration) {
	dataMigrationAppliedVersion.WithLabelValues(dm.Package).Set(float64(dm.Version))
}

// recordProgressMetrics mirrors a migrator's progress report into gauges.
// Percent/Total are only set when the total is known; ETA only when computable.
func recordProgressMetrics(dm *DataMigration, p Progress, eta time.Duration) {
	v := versionLabel(dm.Version)
	dataMigrationProgressCompleted.WithLabelValues(dm.Package, v).Set(float64(p.Completed))

	if p.Total > 0 {
		dataMigrationProgressTotal.WithLabelValues(dm.Package, v).Set(float64(p.Total))
		dataMigrationProgressPercent.WithLabelValues(dm.Package, v).Set(p.Percent())
	}

	if eta > 0 {
		dataMigrationETA.WithLabelValues(dm.Package, v).Set(durationMillis(eta))
	}
}
