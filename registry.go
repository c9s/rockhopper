package rockhopper

import (
	"fmt"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

type RegistryKey struct {
	Package string
	Version int64
}

// registeredGoMigrations stores the global registered migrations
// applications may register their compiledd migration scrips into this map
var registeredGoMigrations = map[RegistryKey]*Migration{}

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
	v, err := FileNumericComponent(filename)
	if err != nil {
		log.Panic(err)
	}

	migration := &Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

	key := RegistryKey{Package: packageName, Version: v}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with %q", filename, existing.Source))
	}

	registeredGoMigrations[key] = migration
}
