package web

import (
	"errors"
	"fmt"
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/frontend"
	"github.com/ahmetson/handler-lib/instance_manager"
	"github.com/ahmetson/handler-lib/manager_client"
	"github.com/ahmetson/log-lib"
	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

var headerContentTypeJson = []byte("application/json")

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestWebSuite struct {
	suite.Suite

	webHandler *Handler
	webConfig  *config.Handler
	tcpClient  *client.Socket
	logger     *log.Logger
	routes     map[string]interface{}
	webClient  *fasthttp.Client
}

// todo test in-process and external types of the handlers
// todo test the business of the handler
// Make sure that Account is set to five
// before each test
func (test *TestWebSuite) SetupTest() {
	s := &test.Suite

	logger, err := log.New("web", false)
	s.Require().NoError(err, "failed to create logger")
	test.logger = logger

	web, err := New()
	s.Require().NoError(err)
	test.webHandler = web

	// Socket to talk to clients
	test.routes = make(map[string]interface{}, 2)
	test.routes["command_1"] = func(request message.Request) *message.Reply {
		return request.Ok(request.Parameters.Set("id", request.Command))
	}
	test.routes["command_2"] = func(request message.Request) *message.Reply {
		return request.Ok(request.Parameters.Set("id", request.Command))
	}

	err = test.webHandler.Route("command_1", test.routes["command_1"])
	s.Require().NoError(err)
	err = test.webHandler.Route("command_2", test.routes["command_2"])
	s.Require().NoError(err)

	test.webConfig = &config.Handler{
		Type:           config.ReplierType,
		Category:       "main",
		InstanceAmount: 1,
		Port:           8081,
		Id:             "web_1",
	}

	// Setting the parameters of the Tcp Handler
	test.webHandler.SetConfig(test.webConfig)
	s.Require().NoError(test.webHandler.SetLogger(test.logger))

	// You may read the timeouts from some config
	readTimeout, _ := time.ParseDuration("500ms")
	writeTimeout, _ := time.ParseDuration("500ms")
	maxIdleConnDuration, _ := time.ParseDuration("1h")
	test.webClient = &fasthttp.Client{
		ReadTimeout:                   readTimeout,
		WriteTimeout:                  writeTimeout,
		MaxIdleConnDuration:           maxIdleConnDuration,
		NoDefaultUserAgentHeader:      true, // Don't send: User-Agent: fasthttp
		DisableHeaderNamesNormalizing: true, // If you set the case on your headers correctly, you can enable this
		DisablePathNormalizing:        true,
		// increase DNS cache time to an hour instead of default minute
		Dial: (&fasthttp.TCPDialer{
			Concurrency:      4096,
			DNSCacheDuration: time.Hour,
		}).Dial,
	}
}

// Test_14_Start starts the handler.
func (test *TestWebSuite) Test_14_Start() {
	s := &test.Suite

	s.Require().False(test.webHandler.running)

	err := test.webHandler.Start()
	s.Require().NoError(err)

	// Wait a bit for initialization
	time.Sleep(time.Millisecond * 100)

	// Make sure that everything works
	s.Require().Equal(test.webHandler.InstanceManager.Status(), instance_manager.Running)
	s.Require().Equal(test.webHandler.Frontend.Status(), frontend.RUNNING)
	s.Require().True(test.webHandler.running)

	// sending a data
	test.sendPostRequest("command_1")

	// Now let's close it
	fmt.Printf("%v\n", *test.webConfig)
	managerClient, err := manager_client.New(test.webConfig)
	s.Require().NoError(err)
	s.Require().NoError(managerClient.Close())

	// Wait a bit for closing handler threads
	time.Sleep(time.Millisecond * 100)

	s.Require().Equal(test.webHandler.InstanceManager.Status(), instance_manager.Idle)
	s.Require().Equal(test.webHandler.Frontend.Status(), frontend.CREATED)
	s.Require().False(test.webHandler.running)
}

func (test *TestWebSuite) sendPostRequest(cmd string) {
	s := &test.Suite

	// per-request timeout
	reqTimeout := time.Second

	reqEntity := message.Request{
		Command:    cmd,
		Parameters: key_value.Empty(),
	}
	reqEntityBytes, _ := reqEntity.Bytes()

	req := fasthttp.AcquireRequest()
	url := fmt.Sprintf("http://localhost:%d/", test.webConfig.Port)
	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentTypeBytes(headerContentTypeJson)
	req.SetBodyRaw(reqEntityBytes)

	resp := fasthttp.AcquireResponse()
	err := test.webClient.DoTimeout(req, resp, reqTimeout)
	fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	s.Require().NoError(err)
	if err != nil {
		errName, known := httpConnError(err)
		if known {
			_, err := fmt.Fprintf(os.Stderr, "WARN conn error: %v\n", errName)
			s.Require().NoError(err)
		} else {
			_, err := fmt.Fprintf(os.Stderr, "ERR conn failure: %v %v\n", errName, err)
			s.Require().NoError(err)
		}

		return
	}

	statusCode := resp.StatusCode()
	respBody := resp.Body()

	if statusCode != http.StatusOK {
		_, err := fmt.Fprintf(os.Stderr, "ERR invalid HTTP response code: %d\n", statusCode)
		s.Require().NoError(err)
		return
	}

	reply, err := message.ParseReply([]string{string(respBody)})
	if err != nil {
		s.Require().True(errors.Is(err, io.EOF))
	}
	test.logger.Info("replied", "reply", reply)
}

func httpConnError(err error) (string, bool) {
	var (
		errName string
		known   = true
	)

	switch {
	case errors.Is(err, fasthttp.ErrTimeout):
		errName = "timeout"
	case errors.Is(err, fasthttp.ErrNoFreeConns):
		errName = "conn_limit"
	case errors.Is(err, fasthttp.ErrConnectionClosed):
		errName = "conn_close"
	case reflect.TypeOf(err).String() == "*net.OpError":
		errName = "timeout"
	default:
		known = false
	}

	return errName, known
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHandler(t *testing.T) {
	suite.Run(t, new(TestWebSuite))
}
