package version

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintOutputContainsVersionVars(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()
	Print()
	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, Version)
	assert.Contains(t, output, Date)
	assert.Contains(t, output, Commit)
	assert.Contains(t, output, "Build version:")
	assert.Contains(t, output, "Build date:")
	assert.Contains(t, output, "Build commit:")
}

func TestVersionVariables(t *testing.T) {
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "unknown", Date)
	assert.Equal(t, "none", Commit)
}
