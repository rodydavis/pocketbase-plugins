# WebRTC

## Getting Started

```go
package main

import (
    "log"
    "github.com/pocketbase/pocketbase"
    web_rtc "github.com/rodydavis/pocketbase-plugins/web-rtc"
)

func main() {
	app := pocketbase.New()

	err = web_rtc.Init(app)
	if err != nil {
		log.Fatal(err)
	}

    if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```
