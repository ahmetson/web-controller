package web

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/base"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/pair"
	"github.com/ahmetson/log-lib"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	*base.Handler
	serviceUrl string
	logger     *log.Logger
	pairClient *client.Socket
}

func New() (*Handler, error) {
	handler := base.New()

	webController := Handler{
		Handler: handler,
		logger:  nil,
	}

	return &webController, nil
}

// SetConfig adds the parameters of the handler from the config.
func (web *Handler) SetConfig(handler *config.Handler) {
	handler.Type = config.ReplierType
	web.Handler.SetConfig(handler)
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
	return web.Handler.SetLogger(parent)
}

// Route adds a route along with its handler to this handler
func (web *Handler) Route(_ string, _ any, _ ...string) error {
	return fmt.Errorf("unsupported")
}

// Type returns the base handler type that web extends.
func (web *Handler) Type() config.HandlerType {
	return web.Handler.Type()
}

func (web *Handler) Start() error {
	instanceConfig := web.Handler.Config()
	if instanceConfig == nil {
		return fmt.Errorf("no config")
	}

	if web.Handler.Manager == nil {
		return fmt.Errorf("handler manager not initiated. call SetConfig and SetLogger first")
	}

	// Web runs on http protocol only
	if instanceConfig.Port == 0 {
		return fmt.Errorf("only tcp channels supported. Port is not set")
	}

	if err := web.Handler.Frontend.PairExternal(); err != nil {
		return fmt.Errorf("web.Handler.Frontend.PairExternal: %w", err)
	}
	if web.pairClient != nil {
		if err := web.pairClient.Close(); err != nil {
			return fmt.Errorf("already initiated pair client close: %w", err)
		}
	}
	pairClient, err := pair.NewClient(instanceConfig)
	if err != nil {
		return fmt.Errorf("pair.NewClient(web.base.Config()): %w", err)
	}
	web.pairClient = pairClient

	if err := web.setRoutes(); err != nil {
		return fmt.Errorf("web.setRoutes: %w", err)
	}

	if err := web.Handler.Start(); err != nil {
		return fmt.Errorf("web.base.Start: %w", err)
	}

	addr := fmt.Sprintf(":%d", instanceConfig.Port)

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

	resp, err := web.pairClient.RawRequest(requestMessage)

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

	if serverReply.IsOK() {
		ctx.SetStatusCode(200)
	} else {
		ctx.SetStatusCode(403)
	}
	replyMessage, _ := serverReply.String()
	_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
}
