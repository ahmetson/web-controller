package web

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/base"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/log-lib"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	base               *base.Handler
	serviceUrl         string
	logger             *log.Logger
	requiredExtensions []string
	extensionConfigs   key_value.KeyValue
	extensions         []*client.Socket
	destinationSocket  *client.Socket
	destinationConfig  *clientConfig.Client
}

func New() (*Handler, error) {
	handler := base.New()

	webController := Handler{
		base:               handler,
		logger:             nil,
		requiredExtensions: make([]string, 0),
		extensionConfigs:   key_value.Empty(),
		extensions:         make([]*client.Socket, 0),
	}

	return &webController, nil
}

func (web *Handler) Config() *config.Handler {
	return web.base.Config()
}

func (web *Handler) SetDestination(destination *clientConfig.Client) {
	web.destinationConfig = destination
}

// SetConfig adds the parameters of the handler from the config.
func (web *Handler) SetConfig(handler *config.Handler) {
	handler.Type = config.ReplierType
	web.base.SetConfig(handler)
}

func (web *Handler) close() error {
	srv := &fasthttp.Server{}
	err := srv.Shutdown()
	if err != nil {
		return fmt.Errorf("server.Shutdown: %w", err)
	}
	return nil
}

// SetLogger sets the logger.
func (web *Handler) SetLogger(parent *log.Logger) error {
	web.logger = parent.Child("web")
	return web.base.SetLogger(parent)
}

// AddDepByService adds the config of the dependency. Intended to be called by Service not by developer
func (web *Handler) AddDepByService(dep *clientConfig.Client) error {
	return web.base.AddDepByService(dep)
}

// AddedDepByService returns true if the configuration exists
func (web *Handler) AddedDepByService(id string) bool {
	return web.base.AddedDepByService(id)
}

// DepIds return the list of extension names required by this handler.
func (web *Handler) DepIds() []string {
	return web.base.DepIds()
}

// Route adds a route along with its handler to this handler
func (web *Handler) Route(_ string, _ any, _ ...string) error {
	return fmt.Errorf("unsupported")
}

// Type returns the base handler type that web extends.
func (web *Handler) Type() config.HandlerType {
	return web.base.Type()
}

// Status is not supported.
func (web *Handler) Status() string {
	return web.base.Status()
}

func (web *Handler) Start() error {
	instanceConfig := web.base.Config()
	if instanceConfig == nil {
		return fmt.Errorf("no config")
	}

	if web.base.Manager == nil {
		return fmt.Errorf("handler manager not initiated. call SetConfig and SetLogger first")
	}

	if web.destinationConfig == nil {
		return fmt.Errorf("destination config not initiated. call SetDestination first")
	}

	// Web runs on http protocol only
	if instanceConfig.Port == 0 {
		return fmt.Errorf("only tcp channels supported. Port is not set")
	}

	if err := web.setRoutes(); err != nil {
		return fmt.Errorf("web.setRoutes: %w", err)
	}

	if err := web.base.Start(); err != nil {
		return fmt.Errorf("web.base.Start: %w", err)
	}

	addr := fmt.Sprintf(":%d", instanceConfig.Port)

	socket, err := client.New(web.destinationConfig)
	if err != nil {
		return fmt.Errorf("client.New('destination'): %w", err)
	}
	web.destinationSocket = socket

	if err := fasthttp.ListenAndServe(addr, web.requestHandler); err != nil {
		return fmt.Errorf("error in ListenAndServe: %w at port %d", err, instanceConfig.Port)
	}

	return fmt.Errorf("http server was down")
}

func (web *Handler) requestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("json/application; charset=utf8")

	// Set arbitrary headers
	ctx.Response.Header.Set("X-Author", "Medet Ahmetson")

	var err error
	request := &message.Request{}

	if !ctx.IsPost() {
		ctx.SetStatusCode(405)

		reply := request.Fail("only POST method allowed")
		replyMessage, _ := reply.String()
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
		return
	}
	body := ctx.PostBody()
	if len(body) == 0 {
		ctx.SetStatusCode(400)

		reply := request.Fail("empty body")
		replyMessage, _ := reply.String()
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
		return
	}

	request, err = message.NewReq([]string{string(body)})
	if err != nil {
		ctx.SetStatusCode(403)
		reply := request.Fail(err.Error())
		replyMessage, _ := reply.String()
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
		return
	}

	if request.IsFirst() {
		request.SetUuid()
	}
	//request.AddRequestStack(web.serviceUrl, web.config.Name, web.config.Instances[0].Instance)
	requestMessage, err := request.String()
	if err != nil {
		ctx.SetStatusCode(500)
		reply := request.Fail(err.Error())
		replyMessage, _ := reply.String()
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
		return
	}

	resp, err := web.destinationSocket.RawRequest(requestMessage)

	if err != nil {
		ctx.SetStatusCode(403)
		reply := request.Fail(err.Error())
		replyMessage, _ := reply.String()
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
		return
	}

	serverReply, err := message.ParseReply(resp)
	if err != nil {
		reply := request.Fail("failed to decode server data: " + err.Error())
		replyMessage, _ := reply.String()
		ctx.SetStatusCode(403)
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
	}

	//err = serverReply.SetStack(web.serviceUrl, web.config.Name, web.config.Instances[0].Instance)
	//if err != nil {
	//	web.logger.Warn("failed to add the stack," "reply", serverReply)
	//}

	if serverReply.IsOK() {
		ctx.SetStatusCode(200)
	} else {
		ctx.SetStatusCode(403)
	}
	replyMessage, _ := serverReply.String()
	_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
}
