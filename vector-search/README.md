# Vector Store

## Getting Started

```go
package main

import (
    "log"
    "github.com/pocketbase/pocketbase"
    vector_store "github.com/rodydavis/pocketbase-plugins/vector-store"
)

func main() {
	app := pocketbase.New()

	err = vector_store.Init(app, "vectors")
	if err != nil {
		log.Fatal(err)
	}

    if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

### REST API

```curl
curl -X GET http://127.0.0.1:8090/api/collections/vectors/records/vector-search?search=Hello
```
