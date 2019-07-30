package logtool

import (
	"fmt"
	"log"
	"testing"
)

//检查/data/logs/logtool_test/下是否有相应的日志
func Test_Filelog(t *testing.T) {
	Init("logtool_test", LevelDebug, true)
	Info("test info")
	Error("test error")
	Debug("test debug")
	Action(Fields{"a": "a", "b": 1})
	Action("{\"a\":1}")
	Action("hello")
	Warn("test warn")
	log.Printf("it's a log")
	log.Printf("[D]it's a debug")
	log.Printf("[I]it's a info")
	log.Printf("[W]it's a warn")
	log.Printf("[E]it's a error")
	Exit()
	select {}
}

func Test_Wait(t *testing.T) {
	Init("logtool_test", LevelDebug, true)
	Info("test wait")
	Error("test wait")
	Debug("test wait")
	Action(Fields{"a": "a", "b": 1})
	Action("{\"a\":1}")
	Warn("test wait")
}

func Test_Pre(t *testing.T) {
	//fmt.Println(*GetPrefix(2))
	Init("logtool_test", LevelInfo, true)
	fmt.Println("---")
	Info("test wait")

}
