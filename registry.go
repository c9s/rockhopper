package rockhopper

import (
	"fmt"
	"runtime"
	"strings"
)

var registeredGoMigrations map[int64]*Migration

// AddMigration registers a migration to the global map
func AddMigration(up, down TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	AddNamedMigration(packageName, filename, up, down)
}

// AddNamedMigration registers a migration to the global map with a given name
func AddNamedMigration(packageName, filename string, up, down TransactionHandler) {
	if registeredGoMigrations == nil {
		registeredGoMigrations = make(map[int64]*Migration)
	}

	v, _ := FileNumericComponent(filename)

	migration := &Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

	if existing, ok := registeredGoMigrations[v]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with %q", filename, existing.Source))
	}
	registeredGoMigrations[v] = migration
}
