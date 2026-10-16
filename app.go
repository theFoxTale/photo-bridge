package main

import (
	"context"
	"sync"
)

// App is the struct bound to the frontend by Wails. Every exported method is
// callable from JavaScript as window.go.main.App.MethodName(...).
type App struct {
	ctx context.Context

	mu              sync.Mutex
	discoveryCancel context.CancelFunc
	discoveryToken  uint64
}

// NewApp creates the application struct.
func NewApp() *App {
	return &App{}
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
