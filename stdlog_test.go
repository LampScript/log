package logtool

import (
	"testing"
)

func TestLog(t *testing.T) {
	Init("test", LevelDebug, true)
	Info("aaa")
}
