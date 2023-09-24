package web

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/handler_manager"
)

// setRoutes sets the default command handlers for the handler manager
func (web *Handler) setRoutes() error {
	// Requesting status which is calculated from statuses of the handler parts
	onStatus := func(req message.Request) message.Reply {
		params := key_value.Empty()
		params.Set("status", handler_manager.Ready)
		return req.Ok(params)
	}

	// onClose adds a close signal to the queue.
	onClose := func(req message.Request) message.Reply {
		err := web.close()
		if err != nil {
			return req.Fail(fmt.Sprintf("web.close: %v", err))
		}

		return req.Ok(key_value.Empty())
	}

	// Stop one of the parts.
	// For example, frontend or instance_manager
	onClosePart := func(req message.Request) message.Reply {
		return req.Ok(key_value.Empty())
	}

	onRunPart := func(req message.Request) message.Reply {
		return req.Ok(key_value.Empty())
	}

	onInstanceAmount := func(req message.Request) message.Reply {
		return req.Ok(key_value.Empty().Set("instance_amount", 1))
	}

	// Returns queue amount and currently processed images amount
	onMessageAmount := func(req message.Request) message.Reply {
		params := key_value.Empty().
			Set("queue_length", 0).
			Set("processing_length", 0)
		return req.Ok(params)
	}

	// Add a new instance, but it doesn't check that instance was added
	onAddInstance := func(req message.Request) message.Reply {
		return req.Fail("instance change is not allowed")
	}

	// Delete the instance
	onDeleteInstance := func(req message.Request) message.Reply {
		return req.Fail("instance change is not allowed")
	}

	onParts := func(req message.Request) message.Reply {
		var parts []string
		var messageTypes []string

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
	if err := web.Handler.Manager.Route(config.InstanceAmount, onInstanceAmount); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.InstanceAmount, err)
	}
	if err := web.Handler.Manager.Route(config.MessageAmount, onMessageAmount); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.MessageAmount, err)
	}
	if err := web.Handler.Manager.Route(config.AddInstance, onAddInstance); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.AddInstance, err)
	}
	if err := web.Handler.Manager.Route(config.DeleteInstance, onDeleteInstance); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.DeleteInstance, err)
	}
	if err := web.Handler.Manager.Route(config.Parts, onParts); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.Parts, err)
	}
	if err := web.Handler.Manager.Route(config.HandlerClose, onClose); err != nil {
		return fmt.Errorf("overwriting handler manager '%s' failed: %w", config.HandlerClose, err)
	}

	return nil
}
