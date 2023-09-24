package web

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/handler-lib/base"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/pair"
	"github.com/ahmetson/log-lib"
)

type Handler struct {
	*base.Handler
	serviceUrl string
	logger     *log.Logger
	pairClient *client.Socket
	status     error // the web part status
	running    bool
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

// SetLogger sets the logger.
func (web *Handler) SetLogger(parent *log.Logger) error {
	web.logger = parent.Child("web")
	return web.Handler.SetLogger(parent)
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

	web.startWeb()

	if err := web.Handler.Start(); err != nil {
		return fmt.Errorf("web.base.Start: %w", err)
	}

	return nil
}
