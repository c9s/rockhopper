package rockhopper

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
