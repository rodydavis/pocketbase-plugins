# ENV Config

## Getting Started

```go
package main

import (
    "log"
    "github.com/pocketbase/pocketbase"
    env_config "github.com/rodydavis/pocketbase-plugins/env-config"
)

func main() {
	app := pocketbase.New()

	err = env_config.Init(app)
	if err != nil {
		log.Fatal(err)
	}

    if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```
