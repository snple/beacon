package api

import (
	"context"
	"crypto/tls"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/snple/beacon/edge"
	"go.uber.org/zap"
)

type ApiService struct {
	es *edge.EdgeService

	node *NodeService
	wire *WireService
	pin  *PinService

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup

	dopts apiOptions
}

func NewApiService(es *edge.EdgeService, opts ...ApiOption) (*ApiService, error) {
	ctx, cancel := context.WithCancel(es.Context())

	s := &ApiService{
		es:     es,
		ctx:    ctx,
		cancel: cancel,
		dopts:  defaultApiOptions(),
	}

	for _, opt := range extraApiOptions {
		opt.apply(&s.dopts)
	}

	for _, opt := range opts {
		opt.apply(&s.dopts)
	}

	s.node = newNodeService(s)
	s.wire = newWireService(s)
	s.pin = newPinService(s)

	return s, nil
}

func (s *ApiService) Register(router gin.IRouter) {
	s.node.register(router)
	s.wire.register(router)
	s.pin.register(router)
}

func (s *ApiService) Start() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()
}

func (s *ApiService) Stop() {
	s.cancel()
	s.closeWG.Wait()

	s.Logger().Sync()
}

func (s *ApiService) Edge() *edge.EdgeService {
	return s.es
}

func (s *ApiService) Context() context.Context {
	return s.ctx
}

func (s *ApiService) Logger() *zap.Logger {
	return s.dopts.logger
}

type apiOptions struct {
	debug             bool
	logger            *zap.Logger
	addr              string
	certFile, keyFile string
	tlsConfig         *tls.Config
}

func defaultApiOptions() apiOptions {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("zap.NewDevelopment(): %v", err)
	}

	return apiOptions{
		debug:  false,
		logger: logger,
		addr:   ":8006",
	}
}

type ApiOption interface {
	apply(*apiOptions)
}

var extraApiOptions []ApiOption

type funcApiOption struct {
	f func(*apiOptions)
}

func (fdo *funcApiOption) apply(do *apiOptions) {
	fdo.f(do)
}

func newFuncApiOption(f func(*apiOptions)) *funcApiOption {
	return &funcApiOption{
		f: f,
	}
}

func WithDebug(debug bool) ApiOption {
	return newFuncApiOption(func(o *apiOptions) {
		o.debug = debug
	})
}

func WithLogger(logger *zap.Logger) ApiOption {
	return newFuncApiOption(func(o *apiOptions) {
		o.logger = logger
	})
}
