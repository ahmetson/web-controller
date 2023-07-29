package web

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/communication/message"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/proxy"
	"github.com/ahmetson/service-lib/remote"
	"github.com/valyala/fasthttp"
)

type Controller struct {
	Config             *configuration.Controller
	serviceUrl         string
	logger             *log.Logger
	requiredExtensions []string
	extensionConfigs   key_value.KeyValue
	extensions         remote.Clients
}

func NewWebController(parent *log.Logger) (*Controller, error) {
	logger := parent.Child("web-controller")

	webController := Controller{
		logger:             logger,
		requiredExtensions: make([]string, 0),
		extensionConfigs:   key_value.Empty(),
		extensions:         make(remote.Clients, 0),
	}

	return &webController, nil
}

func (web *Controller) AddConfig(config *configuration.Controller, serviceUrl string) {
	web.Config = config
	web.serviceUrl = serviceUrl
}

// AddExtensionConfig adds the configuration of the extension that the controller depends on
func (web *Controller) AddExtensionConfig(extension *configuration.Extension) {
	web.extensionConfigs.Set(extension.Url, extension)
}

// RequireExtension marks the extensions that this controller depends on.
// Before running, the required extension should be added from the configuration.
// Otherwise, controller won't run.
func (web *Controller) RequireExtension(name string) {
	web.requiredExtensions = append(web.requiredExtensions, name)
}

// RequiredExtensions returns the list of extension names required by this controller
func (web *Controller) RequiredExtensions() []string {
	return web.requiredExtensions
}

// AddRoute adds a command along with its handler to this controller
func (web *Controller) AddRoute(_ *command.Route) error {
	web.logger.Error("not implemented")
	return nil
}

func (web *Controller) ControllerType() configuration.Type {
	return configuration.ReplierType
}

func (web *Controller) initExtensionClients() error {
	for _, extensionInterface := range web.extensionConfigs {
		extensionConfig := extensionInterface.(*configuration.Extension)
		extension, err := remote.NewReq(extensionConfig.Url, extensionConfig.Port, web.logger)
		if err != nil {
			return fmt.Errorf("failed to create a request client: %w", err)
		}
		fmt.Println("extension is", extension, "config", extensionConfig)
		web.extensions.Set(extensionConfig.Url, extension)
	}

	return nil
}

func (web *Controller) Run() error {
	if len(web.Config.Instances) == 0 {
		return fmt.Errorf("no instance of the config")
	}

	// todo
	// init extension clients
	err := web.initExtensionClients()
	if err != nil {
		return fmt.Errorf("initExtensionClients: %w", err)
	}

	instanceConfig := web.Config.Instances[0]
	if instanceConfig.Port == 0 {
		web.logger.Fatal("instance port is invalid",
			"controller", instanceConfig.Name,
			"instance", instanceConfig.Instance,
			"port", instanceConfig.Port,
		)
	}

	addr := fmt.Sprintf(":%d", instanceConfig.Port)

	if err := fasthttp.ListenAndServe(addr, web.requestHandler); err != nil {
		return fmt.Errorf("error in ListenAndServe: %w at port %d", err, instanceConfig.Port)
	}

	return fmt.Errorf("http server was down")
}

func (web *Controller) requestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("json/application; charset=utf8")

	// Set arbitrary headers
	ctx.Response.Header.Set("X-Author", "Medet Ahmetson")

	var err error
	request := message.Request{}

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
	proxyClient := remote.GetClient(web.extensions, proxy.ControllerName)

	request, err = message.ParseRequest([]string{string(body)})
	if err != nil {
		ctx.SetStatusCode(403)
		_, _ = fmt.Fprintf(ctx, "%s", err.Error())
		return
	}

	if request.IsFirst() {
		request.SetUuid()
	}
	request.AddRequestStack(web.serviceUrl, web.Config.Name, web.Config.Instances[0].Instance)
	requestMessage, err := request.String()
	if err != nil {
		ctx.SetStatusCode(500)
		_, _ = fmt.Fprintf(ctx, "%s", err.Error())
		return
	}

	resp, err := proxyClient.RequestRawMessage(requestMessage)

	if err != nil {
		ctx.SetStatusCode(403)
		_, _ = fmt.Fprintf(ctx, "%s", err.Error())
		return
	}

	serverReply, err := message.ParseReply(resp)
	if err != nil {
		reply := request.Fail("failed to decode server data: " + err.Error())
		replyMessage, _ := reply.String()
		ctx.SetStatusCode(403)
		_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
	}

	err = serverReply.SetStack(web.serviceUrl, web.Config.Name, web.Config.Instances[0].Instance)
	if err != nil {
		web.logger.Warn("failed to add the stack", "reply", serverReply)
	}

	if serverReply.IsOK() {
		ctx.SetStatusCode(200)
	} else {
		ctx.SetStatusCode(403)
	}
	replyMessage, _ := serverReply.String()
	_, _ = fmt.Fprintf(ctx, "%s", replyMessage)
}
