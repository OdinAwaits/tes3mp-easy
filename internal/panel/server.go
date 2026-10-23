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
	"path/filepath"
	"sync"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	maxHeaderBytes    = 1 << 20
	shutdownTimeout   = 10 * time.Second
)

// Server owns the HTTP redirect listener and the HTTPS listener. New prepares
// TLS material; Listen and Serve are separate so tests and operators can learn
// the actual ephemeral addresses before traffic starts.
type Server struct {
	cfg    Config
	logger *log.Logger

	httpSrv  *http.Server
	httpsSrv *http.Server

	mu        sync.Mutex
	httpLn    net.Listener
	httpsLn   net.Listener
	listening bool
}

// New validates cfg, loads or creates TLS material and the password hash, and
// builds both servers. A nil handler installs the built-in admin UI. A nil
// logger discards logs.
func New(cfg Config, handler http.Handler, logger *log.Logger) (*Server, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid panel config: %w", err)
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	certificate, created, err := loadOrCreateCertificate(cfg)
	if err != nil {
		return nil, err
	}
	if created {
		logger.Printf("tls: generated self-signed certificate in %s; browsers will warn",
			filepath.Dir(cfg.CertFile))
	} else {
		logger.Printf("tls: loaded certificate %s", cfg.CertFile)
	}

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
		handler, err = newHandler(cfg, auth, opsClient, logger, time.Now)
		if err != nil {
			return nil, fmt.Errorf("build panel handler: %w", err)
		}
	}

	secureHandler := withSecurityHeaders(handler)
	server := &Server{
		cfg:    cfg,
		logger: logger,
	}
	server.httpSrv = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newHTTPRedirectHandler(cfg),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          logger,
	}
	server.httpsSrv = &http.Server{
		Addr:              cfg.HTTPSAddr,
		Handler:           secureHandler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          logger,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		},
	}
	return server, nil
}

// Listen binds both addresses. It is idempotent. If the HTTPS bind fails, the
// already-bound HTTP listener is closed so a failed start leaves no port open.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listening {
		return nil
	}

	httpLn, err := net.Listen("tcp", s.cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen http %s: %w", s.cfg.HTTPAddr, err)
	}
	httpsLn, err := net.Listen("tcp", s.cfg.HTTPSAddr)
	if err != nil {
		closeErr := httpLn.Close()
		return errors.Join(
			fmt.Errorf("listen https %s: %w", s.cfg.HTTPSAddr, err),
			closeErr,
		)
	}

	s.httpLn = httpLn
	s.httpsLn = httpsLn
	s.listening = true
	s.logger.Printf("http listening on %s", httpLn.Addr())
	s.logger.Printf("https listening on %s", httpsLn.Addr())
	return nil
}

// HTTPAddr returns the actual HTTP listener address, or an empty string before
// Listen.
func (s *Server) HTTPAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpLn == nil {
		return ""
	}
	return s.httpLn.Addr().String()
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

// Serve binds if needed, serves both listeners until ctx is cancelled or one
// listener fails, then performs a graceful shutdown. A cancelled context is a
// normal stop and returns nil.
func (s *Server) Serve(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		// The caller may have bound the listeners explicitly before starting
		// Serve; a cancelled start must not leave those ports open.
		return s.closeListeners()
	}
	if err := s.Listen(); err != nil {
		return err
	}

	s.mu.Lock()
	httpLn, httpsLn := s.httpLn, s.httpsLn
	s.mu.Unlock()

	serveErr := make(chan error, 2)
	go func() { serveErr <- s.httpSrv.Serve(httpLn) }()
	go func() { serveErr <- s.httpsSrv.ServeTLS(httpsLn, "", "") }()

	var firstErr error
	handled := 0
	select {
	case err := <-serveErr:
		handled = 1
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			firstErr = err
		}
	case <-ctx.Done():
	}
	if firstErr != nil {
		s.logger.Printf("serve: %v", firstErr)
	}

	shutdownErr := s.shutdown()
	for handled < 2 {
		err := <-serveErr
		handled++
		if firstErr == nil && err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	for _, server := range []*http.Server{s.httpsSrv, s.httpSrv} {
		if server == nil {
			continue
		}
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
	listeners := []net.Listener{s.httpLn, s.httpsLn}
	s.mu.Unlock()

	var errs []error
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("close listener %s: %w",
				listener.Addr(), err))
		}
	}
	return errors.Join(errs...)
}
