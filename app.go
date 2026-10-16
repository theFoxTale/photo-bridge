package main

import (
	"context"
	"sync"

	"photo-bridge/backend"
)

// App is the struct bound to the frontend by Wails. Every exported method is
// callable from JavaScript as window.go.main.App.MethodName(...).
type App struct {
	ctx context.Context

	mu              sync.Mutex
	discoveryCancel context.CancelFunc
	discoveryToken  uint64

	library backend.Library
}

// NewApp creates the application struct.
func NewApp() *App {
	return &App{library: backend.NewLibrary()}
}

// runtimeContext returns the Wails runtime context, or a background context
// when startup has not run (for example in tests).
func (a *App) runtimeContext() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}

// startup stores the Wails runtime context. It is not exported, so it is not
// bound to the frontend.
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx = ctx
}

// shutdown cancels any in-flight work when the window closes.
func (a *App) shutdown(_ context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.discoveryCancel != nil {
		a.discoveryCancel()
		a.discoveryCancel = nil
	}
}
