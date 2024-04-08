package gin_lane

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jimsnab/go-lane"
)

type (
	ginRequestHandler struct {
		l   lane.Lane
		opt GinLaneOptions
	}

	laneWriter struct {
		l       lane.Lane
		isError bool
		buf     bytes.Buffer
	}

	responseCollector struct {
		gin.ResponseWriter
		written  bytes.Buffer
		req      *http.Request
		wantBody bool
	}

	GinLaneOptions int
)

const (
	GinLaneOptionLogNone          GinLaneOptions = 0
	GinLaneOptionLogRequestResult                = 1 << iota
	GinLaneOptionDumpRequest
	GinLaneOptionDumpRequestBody
	GinLaneOptionDumpResponse
	GinLaneOptionDumpResponseBody
)

var crlf = []byte("\r\n")

var ginGlobalsInitialized sync.Once

func initGin(l lane.Lane) {
	// gin's got multiple ways of logging and some of them are singletons
	ginGlobalsInitialized.Do(func() {
		gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
			l.Debugf("%s %#v %s handlers:%d", httpMethod, absolutePath, handlerName, nuHandlers)
		}

		gin.DefaultWriter = &laneWriter{l: l}
		gin.DefaultErrorWriter = &laneWriter{l: l, isError: true}
	})
}

// Provides a handler that ensures each gin request is associated with a lane
func NewGinRouter(l lane.Lane, opt GinLaneOptions) (engine *gin.Engine) {
	initGin(l)

	engine = gin.New()
	UseLaneMiddleware(engine, l, opt)
	engine.Use(gin.Recovery())
	return
}

// Attaches the lane middleware to the specified gin engine (aka router)
func UseLaneMiddleware(engine *gin.Engine, l lane.Lane, opt GinLaneOptions) {
	initGin(l)

	glh := &ginRequestHandler{l: l, opt: opt}
	engine.Use(glh.ginLaneMiddleware)
}

func (glh *ginRequestHandler) ginLaneMiddleware(c *gin.Context) {
	l2 := glh.l.Derive()
	c.Request = c.Request.WithContext(l2)

	if (glh.opt & (GinLaneOptionDumpRequest | GinLaneOptionDumpRequestBody)) != 0 {
		raw, err := httputil.DumpRequest(c.Request, (glh.opt&GinLaneOptionDumpRequestBody) != 0)
		if err != nil {
			l2.Tracef("request dump error: %v", err)
		} else {
			dump(l2, "request-data", raw)
		}
	}

	var collector *responseCollector
	if (glh.opt & (GinLaneOptionDumpResponse | GinLaneOptionDumpResponseBody)) != 0 {
		collector = &responseCollector{
			ResponseWriter: c.Writer,
			req:            c.Request,
			wantBody:       (glh.opt & GinLaneOptionDumpResponseBody) != 0,
		}
		c.Writer = collector
	}

	c.Next()

	if (glh.opt & GinLaneOptionLogRequestResult) != 0 {
		l2.Tracef("request: client=%s %s %#v status %d", c.ClientIP(), c.Request.Method, c.Request.RequestURI, c.Writer.Status())
	}

	if collector != nil {
		var raw []byte

		reader := bufio.NewReader(&collector.written)
		resp, err := http.ReadResponse(reader, c.Request)
		if err == nil {
			resp.Close = c.Request.Close
			raw, err = httputil.DumpResponse(resp, (glh.opt&GinLaneOptionDumpResponseBody) != 0)
		}
		if err != nil {
			l2.Tracef("response dump error: %v", err)
		} else {
			dump(l2, "response-data", raw)
		}
	}
}

func dump(l lane.Lane, context string, raw []byte) {
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			l.Tracef("%s: %s", context, strings.TrimRight(line, "\r"))
		}
	}
}

func (lw *laneWriter) Write(data []byte) (written int, err error) {
	written, err = lw.buf.Write(data)
	if err != nil {
		return
	}

	for {
		pos := 0
		by := lw.buf.Bytes()
		text := ""
		for pos < len(by) {
			ch, sz := utf8.DecodeRune(by[pos:])
			if sz == 0 {
				break
			}
			pos = pos + sz
			if ch == '\n' {
				text = string(by[:(pos - sz)])
				if pos < len(by) {
					right := make([]byte, len(by)-pos)
					copy(right, by[pos:])
					lw.buf.Reset()
					_, terr := lw.buf.Write(right)
					if terr != nil {
						err = terr
						return
					}
				}
				break
			}
		}

		if text == "" {
			break
		}

		if lw.isError {
			lw.l.Error(text)
		} else {
			lw.l.Debug(text)
		}
	}

	return
}

func (w *responseCollector) Write(b []byte) (int, error) {
	if w.req != nil {
		w.written.WriteString(fmt.Sprintf("HTTP/%d.%d %d %s%s", w.req.ProtoMajor, w.req.ProtoMinor, w.Status(), http.StatusText(w.Status()), crlf))
		hdr := w.Header().Clone()
		err := hdr.Write(&w.written)
		if err != nil {
			return 0, err
		}
		_, err = w.written.Write(crlf)
		if err != nil {
			return 0, err
		}
		w.req = nil
	}
	if w.wantBody {
		w.written.Write(b)
	}

	return w.ResponseWriter.Write(b)
}

func (w *responseCollector) WriteString(s string) (n int, err error) {
	return w.Write([]byte(s))
}
