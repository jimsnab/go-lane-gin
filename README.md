# Gin Lane

This library implements an adapter for [Gin](https://github.com/gin-gonic/gin) to
use lanes for logging and context.

Lanes are [contexts](https://pkg.go.dev/context), and provide a logging interface.
A correlation ID is logged, which is very helpful when following activity of a
single request. There are several options for log output, such as Go's standard
`log`, or to disk, or to [OpenSearch](https://github.com/jimsnab/go-lane-opensearch).
And there's a lane to test for log output, and a null lane to supress logging.

See [go-lane](https://github.com/jimsnab/go-lane) for more details about lanes.

# Use

## Instantiation
Instead of calling `gin.Default()`, call `NewGinRouter()` instead. This function
associates a parent lane with the new Gin router instance. As the router is run,
each request gets its own lane derived from the parent.

The request and the response can be logged by specifying one or more options to
`NewGinRouter()`, such as `GinLaneOptionLogRequestResult`.

If your code is making its own router via `gin.New()`, call `UseLaneMiddleware()`
to add the lane layer to the router. It usually needs to be the first middleware
in the chain.

## Logging in Handlers
With the Gin Lane middleware installed in the router, the request's context
becomes a lane. Handlers can access the lane instance from the request:

```go
    router.GET("/", func(c *gin.Context) {
        l := c.Request.Context().(lane.Lane)
        l.Infof("processing request...")
    }
```

Because a lane instance is a context, it can also be used for cancelation of
long-running requests, per the typical pattern of Go contexts.
