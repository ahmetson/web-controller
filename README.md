# Web lib
An extended [handler](https://github.com/ahmetson/handler-lib) that runs on HTTP protocol.
Use it to support Http in your services.

The handler is defined in `handler.go`.
The HTTP protocol layer is defined in the `web.go`.

The HTTP protocol is based on [valyala/fasthttp](https://github.com/valyala/fasthttp).

The messages are received only in POST method.
Supports only `message.Request` in the POST body.

## Usage

```go
package main

import (
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/web-lib"
)

func onIndex(req message.Request) *message.Reply {
	return req.Ok(key_value.Empty())
}

func main() {
	handlerConfig, _ := config.NewHandler(config.ReplierType, "web")
	handlerConfig.Port = 80

	handler, _ := web.New()
	handler.SetConfig(handlerConfig)
	_ = handler.Route("index", onIndex)
	_ = handler.Start()
}
```

## Todo
* Add a routing to map to the GET paths.
* Add support for the raw message