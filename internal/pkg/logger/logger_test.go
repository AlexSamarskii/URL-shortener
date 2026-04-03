package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	originalLog := Log
	defer func() {
		Log = originalLog
	}()

	Init()

	assert.NotNil(t, Log)

	_, ok := Log.Handler().(*slog.JSONHandler)
	assert.True(t, ok, "handler should be JSONHandler")
}
