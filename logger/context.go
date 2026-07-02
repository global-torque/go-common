package logger

import (
	"github.com/global-torque/go-common/context/keys"
	"github.com/rs/zerolog"
)

type ContextHook struct{}

func (h ContextHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	ctx := e.GetCtx()
	if ctx == nil {
		return
	}

	serviceCtx, ok := keys.GetCtxValue(ctx, keys.LogInfo).(ServiceContext)
	if !ok {
		return
	}

	e.Interface("serviceContext", serviceCtx)
}
