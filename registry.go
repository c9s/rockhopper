package rockhopper

import (
	"fmt"
	"runtime"
	"strings"
)

type registryKey struct {
	Package string
	Version int64
}

var registeredGoMigrations = map[registryKey]*Migration{}

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
		registeredGoMigrations = make(map[registryKey]*Migration)
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

	key := registryKey{Package: packageName, Version: v}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with %q", filename, existing.Source))
	}

	registeredGoMigrations[key] = migration
}
