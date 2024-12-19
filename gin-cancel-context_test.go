package gin_lane

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jimsnab/go-lane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWithContext(t *testing.T) {
	// Step 1: Initialize a testing lane and derive a cancellable lane
	tl := lane.NewLogLane(nil)
	l, cancelFn := tl.DeriveWithCancel()
	defer cancelFn()

	// Step 2: Configure GinLaneOptions with proper type casting
	options := GinLaneOptions(GinLaneOptionLogRequestResult | GinLaneOptionDumpResponse)

	// Step 3: Create a Gin router and attach the lane middleware
	router := NewGinRouter(l, options)
	UseLaneMiddleware(router, l, options)

	// Step 4: Add a test route
	router.GET("/ping", func(c *gin.Context) {
		l.Infof("Handling /ping request")
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Step 5: Start the server
	serverAddr := ":8600"
	shutdownComplete := make(chan struct{})

	go func() {
		err := RunWithContext(l, router, serverAddr)
		if err != nil && err != http.ErrServerClosed {
			l.Errorf("unexpected server error: %v", err)
			t.Errorf("unexpected server error: %v", err)
		}
		close(shutdownComplete)
	}()

	// Step 6: Verify the server starts successfully
	time.Sleep(100 * time.Millisecond)
	resp, err := http.Get("http://localhost" + serverAddr + "/ping")
	require.NoError(t, err, "expected successful HTTP request")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK response")

	// Step 7: Trigger graceful shutdown
	cancelFn()

	// Step 8: Wait for server shutdown
	select {
	case <-shutdownComplete:
		l.Infof("Server shut down successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("server failed to shut down in a reasonable time")
	}

	// Step 9: Verify the server is no longer reachable
	_, err = http.Get("http://localhost" + serverAddr + "/ping")
	assert.Error(t, err, "expected server to be unreachable after shutdown")
}
