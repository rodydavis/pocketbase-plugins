package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	env_config "github.com/rodydavis/pocketbase-plugins/env-config"
	full_text_search "github.com/rodydavis/pocketbase-plugins/full-text-search"
	vector_search "github.com/rodydavis/pocketbase-plugins/vector-search"
	web_rtc "github.com/rodydavis/pocketbase-plugins/web-rtc"
)

func main() {
	app := pocketbase.New()

	err := env_config.Init(app)
	if err != nil {
		log.Fatal(err)
	}

	err = web_rtc.Init(app)
	if err != nil {
		log.Fatal(err)
	}

	vectorCollections := []vector_search.VectorCollection{
		{
			Name: "vectors",
		},
	}
	err = vector_search.Init(app, vectorCollections...)
	if err != nil {
		log.Fatal(err)
	}

	fullTextSearchCollections := []string{}
	for _, col := range vectorCollections {
		fullTextSearchCollections = append(fullTextSearchCollections, col.Name)
	}
	err = full_text_search.Init(app, fullTextSearchCollections...)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
