package gowebsocket

type Logger interface {
	Errorf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Tracef(format string, args ...interface{})
}

type NoOpLogger struct {
}

func (n NoOpLogger) Errorf(_ string, _ ...interface{}) {
}

func (n NoOpLogger) Warnf(_ string, _ ...interface{}) {
}

func (n NoOpLogger) Infof(_ string, _ ...interface{}) {
}

func (n NoOpLogger) Debugf(_ string, _ ...interface{}) {
}

func (n NoOpLogger) Tracef(_ string, _ ...interface{}) {
}
