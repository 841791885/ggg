package logger

import (
	"fmt"

	"ggg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 根据配置创建应用日志器；开发环境输出控制台文本，生产环境输出 JSON。
func New(logConfig config.LogConfig) (*zap.Logger, error) {
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(logConfig.Level)); err != nil {
		return nil, fmt.Errorf("解析日志级别：%w", err)
	}
	encoder := zap.NewDevelopmentEncoderConfig()
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder.EncodeLevel = zapcore.CapitalLevelEncoder
	encoding := "console"
	if logConfig.Environment == "production" {
		encoder = zap.NewProductionEncoderConfig()
		encoder.EncodeTime = zapcore.ISO8601TimeEncoder
		encoding = "json"
	}
	return zap.Config{Level: level, Development: logConfig.Environment != "production", Encoding: encoding, EncoderConfig: encoder, OutputPaths: []string{"stdout"}, ErrorOutputPaths: []string{"stderr"}}.Build()
}
