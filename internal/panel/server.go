package panel

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	maxHeaderBytes    = 1 << 20
	shutdownTimeout   = 10 * time.Second
)

// Server owns the HTTPS listener. New prepares the certificate manager; Listen
// and Serve are separate so tests and operators can learn the actual ephemeral
// address before traffic starts.
type Server struct {
	cfg     Config
	logger  *log.Logger
	certs   *certificateManager
	metrics *metricsStore

	httpsSrv *http.Server

	mu        sync.Mutex
	httpsLn   net.Listener
	listening bool
}

// New validates cfg, prepares TLS material and the password hash, and builds
// the server. A nil handler installs the built-in admin UI. A nil logger
// discards logs.
func New(cfg Config, handler http.Handler, logger *log.Logger) (*Server, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid panel config: %w", err)
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	certs, err := newCertificateManager(cfg, logger)
	if err != nil {
		return nil, err
	}

	var metrics *metricsStore
	if handler == nil {
		auth, authErr := openAuthStore(cfg, logger)
		if authErr != nil {
			return nil, authErr
		}
		opsClient, opsErr := NewOpsClient(OpsClientConfig{
			Root:   cfg.Root,
			Logger: logger,
		})
		if opsErr != nil {
			return nil, fmt.Errorf("build ops client: %w", opsErr)
		}
		h, handlerErr := newPanelHandler(cfg, auth, opsClient, logger, time.Now)
		if handlerErr != nil {
			return nil, fmt.Errorf("build panel handler: %w", handlerErr)
		}
		handler = h.routes()
		metrics = h.metrics
	}

	secureHandler := withSecurityHeaders(handler)
	server := &Server{
		cfg:     cfg,
		logger:  logger,
		certs:   certs,
		metrics: metrics,
	}
	server.httpsSrv = &http.Server{
		Addr:              cfg.HTTPSAddr,
		Handler:           secureHandler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          logger,
		TLSConfig: &tls.Config{
			GetCertificate: certs.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		},
	}
	return server, nil
}

// Listen binds the HTTPS address. It is idempotent.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listening {
		return nil
	}

	httpsLn, err := net.Listen("tcp", s.cfg.HTTPSAddr)
	if err != nil {
		return fmt.Errorf("listen https %s: %w", s.cfg.HTTPSAddr, err)
	}
	s.httpsLn = httpsLn
	s.listening = true
	s.logger.Printf("https listening on %s", httpsLn.Addr())
	return nil
}

// HTTPSAddr returns the actual HTTPS listener address, or an empty string before
// Listen.
func (s *Server) HTTPSAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpsLn == nil {
		return ""
	}
	return s.httpsLn.Addr().String()
}

// Serve ensures a certificate is available, binds if needed, and serves HTTPS
// until ctx is cancelled or the listener fails. A cancelled context is a normal
// stop and returns nil.
func (s *Server) Serve(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		// The caller may have bound the listener explicitly before starting
		// Serve; a cancelled start must not leave the port open.
		return s.closeListeners()
	}
	if err := s.certs.Ensure(ctx); err != nil {
		closeErr := s.closeListeners()
		if errors.Is(err, context.Canceled) {
			// SIGTERM during the initial ACME request is a normal stop.
			return closeErr
		}
		return errors.Join(err, closeErr)
	}
	if err := s.Listen(); err != nil {
		return err
	}

	s.mu.Lock()
	httpsLn := s.httpsLn
	s.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.certs.Run(runCtx)
	if s.metrics != nil {
		s.metrics.start(runCtx.Done())
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- s.httpsSrv.ServeTLS(httpsLn, "", "") }()

	var firstErr error
	handled := false
	select {
	case err := <-serveErr:
		handled = true
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			firstErr = err
		}
	case <-ctx.Done():
	}
	if firstErr != nil {
		s.logger.Printf("serve: %v", firstErr)
	}

	shutdownErr := s.shutdown()
	if !handled {
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) &&
			firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return shutdownErr
}

func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var errs []error
	if server := s.httpsSrv; server != nil {
		if err := server.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown %s: %w", server.Addr, err))
			if closeErr := server.Close(); closeErr != nil &&
				!errors.Is(closeErr, http.ErrServerClosed) {
				errs = append(errs, fmt.Errorf("close %s: %w", server.Addr, closeErr))
			}
		}
	}

	// Shutdown only closes listeners it has already tracked. A signal that
	// arrives between Listen and the Serve goroutine entering trackListener
	// would otherwise leave the port open.
	if err := s.closeListeners(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	s.logger.Printf("stopped")
	return nil
}

func (s *Server) closeListeners() error {
	s.mu.Lock()
	listener := s.httpsLn
	s.mu.Unlock()

	if listener == nil {
		return nil
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("close listener %s: %w", listener.Addr(), err)
	}
	return nil
}
