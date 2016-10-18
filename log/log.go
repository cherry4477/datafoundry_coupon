package log

import (
	"github.com/astaxie/beego/logs"
)

const (
	CHANNELLEN = 3000
)

var (
	SetDebug bool
	logger   *logs.BeeLogger
)

func init() {

	logger = logs.NewLogger(CHANNELLEN)

	err := logger.SetLogger("console", "")
	if err != nil {
		logger.Error("set logger err:", err)
		return
	}

	//显示文件名和行号
	logger.EnableFuncCallDepth(true)

}

func InitLog() {
	//判断是不是以 DEBUG 模式启动
	if SetDebug == false {
		logger.Info("mode is info...")
		logger.SetLevel(logs.LevelInfo)
	} else {
		logger.Info("mode is debug...")
		logger.SetLevel(logs.LevelDebug)
	}
}

func GetLogger() *logs.BeeLogger {
	return logger
}
