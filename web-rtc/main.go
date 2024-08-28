package web_rtc

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

var StunServers = []string{
	"stun:stun1.l.google.com:19302",
	"stun:stun2.l.google.com:19302",
}

var (
	AuthCollection             = "users"
	IceServerCollection        = "ice_servers"
	CallsCollection            = "calls"
	OfferCandidatesCollection  = "offer_candidates"
	AnswerCandidatesCollection = "answer_candidates"
)

func Init(app *pocketbase.PocketBase) error {
	app.OnAfterBootstrap().Add(func(e *core.BootstrapEvent) error {
		iceServer, err := createIceServerCollection(app, IceServerCollection)
		if err != nil {
			return err
		}
		if iceServer != nil {
			for _, server := range StunServers {
				record := models.NewRecord(iceServer)
				record.Set("url", server)
				if err := app.Dao().SaveRecord(record); err != nil {
					return err
				}
			}
		}
		_, err = createCallsServerCollection(app, CallsCollection, AuthCollection)
		if err != nil {
			return err
		}
		_, err = createCandidatesServerCollection(app, OfferCandidatesCollection, CallsCollection)
		if err != nil {
			return err
		}
		_, err = createCandidatesServerCollection(app, AnswerCandidatesCollection, CallsCollection)
		if err != nil {
			return err
		}
		return nil
	})

	return nil
}

func createIceServerCollection(app *pocketbase.PocketBase, target string) (*models.Collection, error) {
	current, _ := app.Dao().FindCollectionByNameOrId(target)
	if current != nil {
		return nil, nil
	}
	fields := []*schema.SchemaField{
		{
			Name:     "url",
			Type:     schema.FieldTypeText,
			Required: true,
		},
	}
	collection := &models.Collection{
		Name:     target,
		Type:     models.CollectionTypeBase,
		Schema:   schema.NewSchema(fields...),
		ListRule: types.Pointer("@request.auth.id != ''"),
		ViewRule: types.Pointer("@request.auth.id != ''"),
		Indexes: types.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_" + target + " ON " + target + " (url);",
		},
	}

	if err := app.Dao().SaveCollection(collection); err != nil {
		return nil, err
	}

	return collection, nil
}

func createCallsServerCollection(app *pocketbase.PocketBase, target string, authCollection string) (*models.Collection, error) {
	current, _ := app.Dao().FindCollectionByNameOrId(target)
	if current != nil {
		return nil, nil
	}
	auth, err := app.Dao().FindCollectionByNameOrId(authCollection)
	if err != nil {
		return nil, err
	}
	fields := []*schema.SchemaField{
		{
			Name:     "user_id",
			Type:     schema.FieldTypeRelation,
			Required: true,
			Options: &schema.RelationOptions{
				MaxSelect:     types.Pointer(1),
				CollectionId:  auth.Id,
				CascadeDelete: true,
			},
		},
		{
			Name: "offer",
			Type: schema.FieldTypeJson,
		},
		{
			Name: "answer",
			Type: schema.FieldTypeJson,
		},
	}
	collection := &models.Collection{
		Name:       target,
		Type:       models.CollectionTypeBase,
		Schema:     schema.NewSchema(fields...),
		ListRule:   types.Pointer("@request.auth.id != ''"),
		ViewRule:   types.Pointer("@request.auth.id != ''"),
		CreateRule: types.Pointer("@request.auth.id != ''"),
		UpdateRule: types.Pointer("@request.auth.id != ''"),
		DeleteRule: types.Pointer("@request.auth.id != ''"),
		Indexes: types.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_" + target + " ON " + target + " (user_id);",
		},
	}

	if err := app.Dao().SaveCollection(collection); err != nil {
		return nil, err
	}

	return collection, nil
}

func createCandidatesServerCollection(app *pocketbase.PocketBase, target string, callsCollection string) (*models.Collection, error) {
	current, _ := app.Dao().FindCollectionByNameOrId(target)
	if current != nil {
		return nil, nil
	}
	calls, err := app.Dao().FindCollectionByNameOrId(callsCollection)
	if err != nil {
		return nil, err
	}
	fields := []*schema.SchemaField{
		{
			Name:     "call_id",
			Type:     schema.FieldTypeRelation,
			Required: true,
			Options: &schema.RelationOptions{
				MaxSelect:     types.Pointer(1),
				CollectionId:  calls.Id,
				CascadeDelete: true,
			},
		},
		{
			Name: "data",
			Type: schema.FieldTypeJson,
		},
	}
	collection := &models.Collection{
		Name:       target,
		Type:       models.CollectionTypeBase,
		Schema:     schema.NewSchema(fields...),
		ListRule:   types.Pointer("@request.auth.id != ''"),
		ViewRule:   types.Pointer("@request.auth.id != ''"),
		CreateRule: types.Pointer("@request.auth.id != ''"),
		UpdateRule: types.Pointer("@request.auth.id != ''"),
		DeleteRule: types.Pointer("@request.auth.id != ''"),
		Indexes: types.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_" + target + " ON " + target + " (call_id);",
		},
	}

	if err := app.Dao().SaveCollection(collection); err != nil {
		return nil, err
	}

	return collection, nil
}
