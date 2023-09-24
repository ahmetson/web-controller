package web

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/frontend"
	"github.com/ahmetson/handler-lib/handler_manager"
	instances "github.com/ahmetson/handler-lib/instance_manager"
)

// setRoutes sets the default command handlers for the handler manager
func (web *Handler) setRoutes() error {
	m := web.Handler

	onStatus := func(req message.Request) *message.Reply {
		partStatuses := m.Manager.PartStatuses()
		frontendStatus, err := partStatuses.GetString("frontend")
		if err != nil {
			return req.Fail(fmt.Sprintf("partStatuses.GetString('frontend'): %v", err))
		}
		instanceStatus, err := partStatuses.GetString("instance_manager")
		if err != nil {
			return req.Fail(fmt.Sprintf("partStatuses.GetString('instance_manager'): %v", err))
		}
		layerStatus := LayerRunning
		if !web.running {
			layerStatus = LayerClosed
		}
		if web.status != nil {
			layerStatus = LayerClosed
		}

		params := key_value.Empty()

		if frontendStatus == frontend.RUNNING &&
			instanceStatus == instances.Running &&
			layerStatus == LayerRunning {
			params.Set("status", handler_manager.Ready)
		} else {
			partStatuses.Set("layer", layerStatus)
			params.Set("status", handler_manager.Incomplete).
				Set("parts", partStatuses)
			if web.status != nil {
				params.Set("errors", key_value.Empty().Set("layer", web.status.Error()))
			}
		}

		return req.Ok(params)
	}

	onClosePart := func(req message.Request) *message.Reply {
		part, err := req.Parameters.GetString("part")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.Parameters.GetString('part'): %v", err))
		}

		if part == "frontend" {
			if m.Frontend.Status() != frontend.RUNNING {
				return req.Fail("frontend not running")
			} else {
				if err := m.Frontend.Close(); err != nil {
					return req.Fail(fmt.Sprintf("failed to close the frontend: %v", err))
				}
				return req.Ok(key_value.Empty())
			}
		} else if part == "instance_manager" {
			if m.InstanceManager.Status() != instances.Running {
				return req.Fail("instance manager not running")
			} else {
				m.InstanceManager.Close()
				return req.Ok(key_value.Empty())
			}
		} else if part == "layer" {
			if !web.running {
				return req.Fail("layer is not running")
			}
			if err := web.closeWeb(); err != nil {
				return req.Fail(fmt.Sprintf("web.closeWeb: %v", err))
			}
			return req.Ok(key_value.Empty())
		} else {
			return req.Fail(fmt.Sprintf("unknown part '%s' to stop", part))
		}
	}
	onRunPart := func(req message.Request) *message.Reply {
		part, err := req.Parameters.GetString("part")
		if err != nil {
			return req.Fail(fmt.Sprintf("req.Parameters.GetString('part'): %v", err))
		}

		if part == "frontend" {
			if m.Frontend.Status() == frontend.RUNNING {
				return req.Fail("frontend running")
			} else {
				err := m.Frontend.Start()
				if err != nil {
					return req.Fail(fmt.Sprintf("m.Frontend.Start: %v", err))
				}
				return req.Ok(key_value.Empty())
			}
		} else if part == "instance_manager" {
			if m.InstanceManager.Status() == instances.Running {
				return req.Fail("instance manager running")
			} else {
				err := m.StartInstanceManager()
				if err != nil {
					return req.Fail(fmt.Sprintf("base.StartInstanceManager: %v", err))
				}
				return req.Ok(key_value.Empty())
			}
		} else if part == "layer" {
			if web.running {
				return req.Fail("web part already running")
			}
			web.startWeb()

			return req.Ok(key_value.Empty())
		} else {
			return req.Fail(fmt.Sprintf("unknown part '%s' to stop", part))
		}
	}

	onParts := func(req message.Request) *message.Reply {
		parts := []string{
			"frontend",
			"instance_manager",
			"layer",
		}
		messageTypes := []string{
			"queue_length",
			"processing_length",
		}

		params := key_value.Empty().
			Set("parts", parts).
			Set("message_types", messageTypes)

		return req.Ok(params)
	}

	if err := web.Handler.Manager.Route(config.HandlerStatus, onStatus); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.HandlerStatus, err)
	}
	if err := web.Handler.Manager.Route(config.ClosePart, onClosePart); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.ClosePart, err)
	}
	if err := web.Handler.Manager.Route(config.RunPart, onRunPart); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.RunPart, err)
	}
	if err := web.Handler.Manager.Route(config.Parts, onParts); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.Parts, err)
	}

	return nil
}
