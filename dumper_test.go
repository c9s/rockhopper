package rockhopper

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderMigrationKeepsStatements guards that compiled migrations register
// their SQL as data (so the console can preview each statement at runtime),
// rather than baking the SQL into opaque up/down function bodies.
func TestRenderMigrationKeepsStatements(t *testing.T) {
	m := &Migration{
		Package: "billing",
		Name:    "create_invoices",
		Version: 20200101000000,
		Source:  "migrations/20200101000000_create_invoices.sql",
		UseTx:   true,
		UpStatements: []Statement{
			{Direction: DirectionUp, SQL: "CREATE TABLE invoices (id INT PRIMARY KEY)"},
		},
		DownStatements: []Statement{
			{Direction: DirectionDown, SQL: "DROP TABLE invoices"},
		},
	}

	out, err := renderMigration("migrations", m)
	require.NoError(t, err)

	src := string(out)

	// SQL is preserved as data so descMigration counts and the EXECUTING preview work.
	assert.Contains(t, src, "AddStatementMigration")
	assert.Contains(t, src, "rockhopper.DirectionUp")
	assert.Contains(t, src, "CREATE TABLE invoices (id INT PRIMARY KEY)")
	assert.Contains(t, src, "DROP TABLE invoices")

	// the SQL must no longer be hidden inside generated function bodies
	assert.NotContains(t, src, "func up")
	assert.NotContains(t, src, "tx.ExecContext")
}

func TestMigrationDumper(t *testing.T) {
	var loader SqlMigrationLoader
	var migrations, err = loader.Load("testdata/migrations")
	assert.NoError(t, err)
	assert.NotEmpty(t, migrations)

	var dir = "temp1"

	err = os.Mkdir(dir, 0777)
	if err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}

	var dumper = GoMigrationDumper{Dir: dir}
	err = dumper.Dump(migrations)
	if !assert.NoError(t, err) {
		return
	}

	// test compile
	cmd := exec.Command("go", "build", "./"+dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}

	// test compile
	testCmd := exec.Command("go", "test", "./"+dir)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	err = testCmd.Run()
	if err != nil {
		t.Fatal(err)
	}

	err = os.RemoveAll(dir)
	if err != nil {
		t.Fatal(err)
	}
}
