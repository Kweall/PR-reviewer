package logger

type Logger interface {
	Info(msg string, kv ...any)
	Success(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)

	WithWorker(workerID string) Logger
}
