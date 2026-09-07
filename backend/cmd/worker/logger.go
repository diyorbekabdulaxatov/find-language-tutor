package main

import (
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// asynqLogger adapts *slog.Logger to asynq.Logger.
type asynqLogger struct {
	l *slog.Logger
}

func newAsynqLogger(l *slog.Logger) asynq.Logger { return &asynqLogger{l: l} }

func (a *asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a *asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a *asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a *asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }
func (a *asynqLogger) Fatal(args ...any) { a.l.Error("FATAL: " + fmt.Sprint(args...)) }
