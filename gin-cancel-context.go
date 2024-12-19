package gin_lane

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jimsnab/go-lane"
)

// RunWithContext starts the Gin server and serves until the lane is canceled.
// It gracefully shuts down the server when the lane is canceled and returns http.ErrServerClosed.
func RunWithContext(l lane.Lane, engine *gin.Engine, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	// channel to listen for server errors
	errChan := make(chan error, 1)

	go func() {
		l.Infof("server is running on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	// wait for lane cancellation or server error
	select {
	case <-l.Done():
		// lane canceled: initiate graceful shutdown
		l.Infof("shutdown signal received, shutting down server...")
		shutdownCtx, cancel := l.DeriveWithTimeout(30 * time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			l.Errorf("server shutdown failed: %v", err)
			return err
		}
		l.Infof("shutdown complete")
		return http.ErrServerClosed
	case err := <-errChan:
		// server error occurred
		l.Errorf("server error: %v", err)
		return err
	}
}
