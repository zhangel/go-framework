package framework

import (
	//	"context"
	//"fmt"
	//"github.com/zhangel/go-framework.git/config"
	"github.com/zhangel/go-framework.git/log"
	//"github.com/zhangel/go-framework.git/log/fields"
	//"github.com/zhangel/go-framework.git/log/level"
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	defer Init()()
	/*
		myConf := config.Config()
		logger := log.WithConfig(myConf)
		logger.Infof("xxxxxx")
	*/
	logger := log.WithRateLimit(time.Second * 10)
	logger.Infof("info message")
	/*
		logger.Debugf("debug message %+v", time.Second*100)
		logger.Tracef("tarce message %+v", time.Second*300)
		logger.Warnf("warn message %T", time.Second)
	*/
	/*
		DebugLevel := level.DebugLevel
		logger := log.WithLevel(DebugLevel)
		logger.Info("xxxxxxxxxxxxxxxxxxxx")
		logger.Debug("xxxxxxxxxxxxxxxxxxxx")
		logger.Trace("xxxxxxxxxxxxxxxxxxxx")
	*/

	/*
		logger := log.WithField("column", "value")
		logger.Infof("test %s", "test_value")
	*/

	/*
		Field := fields.Field{K: "func", V: "TestFramework"}
		Field2 := fields.Field{K: "func2", V: "TestFramework2"}
		Fields := fields.Fields{}
		Fields = append(Fields, Field)
		Fields = append(Fields, Field2)
		logger := log.WithFields(Fields)
		logger.Infof("test infof %s", "value_infof")
	*/

	/*
		ctx := context.Background()
		logger := log.WithContext(ctx)
		logger.Debugf("xxxx %s", "ssssssssssssss")
		logger.Debug("test debug info")
		logger.Infof("my info print %s", "info message")
		logger.Warn("print warn info")
		logger.Error("print error message")
		logger.Errorf("error print %s", "errorf message")
		logger.Warnf("warnf print :%s", "warnf message")
		logger.Trace("print trace message")
		logger.Tracef("print tracef info: %s", "tracef message")
		minLevel := log.LogLevel()
		logger.Infof("minLevel=%v", minLevel)
	*/

	/*
		logger.Fatal("print fatal message")
		logger.Fatalf("print fatal info: %s", "fatalf message")
	*/

}
