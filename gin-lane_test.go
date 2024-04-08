package gin_lane

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jimsnab/go-lane"
)

func testServer(t *testing.T, opt GinLaneOptions) (tl lane.TestingLane, srv *http.Server) {
	tl = lane.NewTestingLane(context.Background())
	tl.WantDescendantEvents(true)
	tl.AddTee(lane.NewLogLane(context.Background()))

	router := NewGinRouter(tl, opt)

	router.POST("/echo", func(c *gin.Context) {
		l := c.Request.Context().(lane.Lane)
		l.Infof("echo request received")

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}

		buf := bytes.NewBuffer(body)
		reader := bufio.NewReader(buf)

		c.DataFromReader(http.StatusOK, int64(len(body)), "application/json", reader, nil)
		if len(c.Errors) != 0 {
			panic(c.Errors[0])
		}
	})

	srv = &http.Server{
		Handler: router,
	}

	ln, err := net.Listen("tcp", ":8600")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(ln)
	}()

	t.Cleanup(func() {
		srv.Shutdown(tl)
		wg.Wait()
	})
	return
}

func testSendEcho(t *testing.T) {
	body, err := json.Marshal("testing 123")
	if err != nil {
		t.Fatal(err)
	}
	reader := strings.NewReader(string(body))
	resp, err := http.Post("http://localhost:8600/echo", "application/json", reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var data string
	err = json.Unmarshal(raw, &data)
	if err != nil {
		t.Fatal(err)
	}

	if data != "testing 123" {
		t.Error("wrong response data")
	}
}

func TestOneRequest(t *testing.T) {
	tl, _ := testServer(t, GinLaneOptionLogNone)
	testSendEcho(t)

	if !tl.FindEventText("DEBUG\tPOST \"/echo\" github.com/jimsnab/go-lane-gin.testServer.func1 handlers:3") {
		t.Fatal("debug not hooked")
	}
	if !tl.FindEventText("INFO\techo request received") {
		t.Fatal("missing handler logging")
	}
}

func TestRequestResult(t *testing.T) {
	tl, _ := testServer(t, GinLaneOptionLogRequestResult)
	testSendEcho(t)
	testSendEcho(t)
	if !tl.FindEventText("TRACE\trequest: client=127.0.0.1 POST \"/echo\" status 200") {
		t.Fatal("request result not logged")
	}
	if strings.Contains(tl.EventsToString(), "request-data") {
		t.Fatal("request data should not be logged")
	}
	if strings.Contains(tl.EventsToString(), "response-data") {
		t.Fatal("response data should not be logged")
	}
}

func TestLogHeaders(t *testing.T) {
	tl, _ := testServer(t, GinLaneOptionDumpRequest|GinLaneOptionDumpResponse)
	testSendEcho(t)
	if !tl.FindEventText("TRACE\trequest-data: POST /echo HTTP/1.1") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\trequest-data: Host: localhost:8600") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\trequest-data: Content-Length: 13") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: HTTP/1.1 200 OK") {
		t.Fatal("response header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: Content-Length: 13") {
		t.Fatal("response header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: Content-Type: application/json") {
		t.Fatal("response header not logged")
	}
}

func TestLogBody(t *testing.T) {
	tl, _ := testServer(t, GinLaneOptionDumpRequestBody|GinLaneOptionDumpResponseBody)
	testSendEcho(t)
	if !tl.FindEventText("TRACE\trequest-data: POST /echo HTTP/1.1") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\trequest-data: Host: localhost:8600") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\trequest-data: Content-Length: 13") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\trequest-data: \"testing 123\"") {
		t.Fatal("request header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: HTTP/1.1 200 OK") {
		t.Fatal("response header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: Content-Length: 13") {
		t.Fatal("response header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: Content-Type: application/json") {
		t.Fatal("response header not logged")
	}
	if !tl.FindEventText("TRACE\tresponse-data: \"testing 123\"") {
		t.Fatal("response header not logged")
	}
}
