package main

import (
	"context"
	"errors"
	"multi-server/server/grpc"
	"multi-server/server/http"
	"multi-server/server/slave"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"multi-server/server"
)

func main() {
	ss := []server.Server{
		&slave.Slave{},
		&http.Http{},
		&grpc.Grpc{},
	}

	app := New(ss)
	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func New(ss []server.Server) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		ctx:    ctx,
		cancel: cancel,
		sigs:   []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
	}
}

type App struct {
	servers []server.Server
	sigs    []os.Signal // []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT}
	cancel  func()
	ctx     context.Context
}

func (a *App) ID() string {
	//TODO implement me
	panic("implement me")
}
func (a *App) Stop() error {

	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *App) Run() error {

	eg, ctx := errgroup.WithContext(a.ctx)
	wg := sync.WaitGroup{}

	//for _, fn := range a.opts.beforeStart {
	//	if err = fn(sctx); err != nil {
	//		return err
	//	}
	//}
	for _, srv := range a.servers {
		srv := srv
		eg.Go(func() error {
			<-ctx.Done() // wait for stop signal
			stopCtx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			return srv.Stop(stopCtx)
		})
		wg.Add(1)
		eg.Go(func() error {
			wg.Done() // here is to ensure server start has begun running before register, so defer is not needed
			return srv.Start(NewContext(ctx, a))
		})
	}
	wg.Wait()
	//if a.opts.registrar != nil {
	//	rctx, rcancel := context.WithTimeout(ctx, a.opts.registrarTimeout)
	//	defer rcancel()
	//	if err = a.opts.registrar.Register(rctx, instance); err != nil {
	//		return err
	//	}
	//}
	//for _, fn := range a.opts.afterStart {
	//	if err = fn(sctx); err != nil {
	//		return err
	//	}
	//}

	c := make(chan os.Signal, 1)
	signal.Notify(c, a.sigs...)
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case <-c:
			return a.Stop()
		}
	})
	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	//for _, fn := range a.opts.afterStop {
	//	err = fn(sctx)
	//}
	return nil
}

type AppInfo interface {
	ID() string
	//Name() string
	//Version() string
	//Metadata() map[string]string
}

type appKey struct{}

// NewContext returns a new Context that carries value.
func NewContext(ctx context.Context, s AppInfo) context.Context {
	return context.WithValue(ctx, appKey{}, s)
}

// FromContext returns the Transport value stored in ctx, if any.
func FromContext(ctx context.Context) (s AppInfo, ok bool) {
	s, ok = ctx.Value(appKey{}).(AppInfo)
	return
}
