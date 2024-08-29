package vector_search

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/google/generative-ai-go/genai"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

type VectorCollection struct {
	Name        string
	ExtraFields []*schema.SchemaField
}

func Init(app *pocketbase.PocketBase, collections ...VectorCollection) error {
	sqlite_vec.Auto()

	client, err := createGoogleAiClient()
	if err != nil {
		return err
	}

	app.OnAfterBootstrap().Add(func(e *core.BootstrapEvent) error {
		for _, target := range collections {
			collection, _ := app.Dao().FindCollectionByNameOrId(target.Name)
			if collection == nil {
				err := createCollection(app, target.Name, target.ExtraFields...)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnModelAfterCreate().Add(func(e *core.ModelEvent) error {
		tbl := e.Model.TableName()
		for _, target := range collections {
			if tbl == target.Name {
				err := modelModify(app, target.Name, client, e)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnModelAfterUpdate().Add(func(e *core.ModelEvent) error {
		tbl := e.Model.TableName()
		for _, target := range collections {
			if tbl == target.Name {
				err := modelModify(app, target.Name, client, e)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnModelAfterDelete().Add(func(e *core.ModelEvent) error {
		tbl := e.Model.TableName()
		for _, target := range collections {
			if tbl == target.Name {
				err := modelDelete(app, target.Name, e)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnCollectionAfterDeleteRequest().Add(func(e *core.CollectionDeleteEvent) error {
		for _, target := range collections {
			if e.Collection.Name == target.Name {
				err := deleteCollection(app, target.Name)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		group := e.Router.Group("/api/collections/:collectionIdOrName/records", apis.ActivityLogger(app))
		group.GET("/vector-search", func(c echo.Context) error {
			target := c.PathParam("collectionIdOrName")
			if _, err := app.Dao().FindCollectionByNameOrId(target); err != nil {
				app.Logger().Error(fmt.Sprint(err))
				return err
			}

			title := c.QueryParam("title")
			content := c.QueryParam("search")
			k := c.QueryParam("k")
			kNum := 5
			if k != "" {
				val, err := strconv.Atoi(k)
				if err == nil {
					kNum = val
				}
			}

			if content == "" {
				return c.NoContent(204)
			}

			vector, err := googleAiEmbedContent(client, genai.TaskTypeRetrievalQuery, title, genai.Text(content))
			if err != nil {
				return err
			}
			jsonVec, err := json.Marshal(vector)
			if err != nil {
				return err
			}

			stmt := "SELECT v.id, distance, v.content, v.created, v.updated "
			stmt += "FROM " + target + "_embeddings "
			stmt += "LEFT JOIN " + target + " v ON v.vector_id = " + target + "_embeddings.id "
			stmt += "WHERE embedding MATCH {:embedding} "
			stmt += "AND k = {:k};"

			results := []dbx.NullStringMap{}
			err = app.Dao().DB().
				NewQuery(stmt).
				Bind(dbx.Params{
					"embedding": string(jsonVec),
					"k":         kNum,
				}).
				All(&results)
			if err != nil {
				app.Logger().Error(fmt.Sprint(err))
				return err
			}
			app.Logger().Info(fmt.Sprint(results))

			c.Response().Header().Set(echo.HeaderContentType, "application/json")
			items := []map[string]any{}
			for _, result := range results {
				m := make(map[string]interface{})
				for key := range result {
					val := result[key]
					value, err := val.Value()
					if err != nil || !val.Valid {
						m[key] = nil
					} else {
						m[key] = value
					}
				}
				items = append(items, m)
			}

			// TODO: Paging result
			return c.JSON(200, items)

		})
		return nil
	})
	return nil
}

func deleteCollection(app *pocketbase.PocketBase, target string) error {
	if _, err := app.Dao().DB().
		NewQuery("DELETE FROM " + target + "_embeddings;").
		Execute(); err != nil {
		return err
	}
	if _, err := app.Dao().DB().
		NewQuery("DROP TABLE IF EXISTS " + target + "_embeddings;").
		Execute(); err != nil {
		return err
	}
	return nil
}

func modelDelete(app *pocketbase.PocketBase, target string, e *core.ModelEvent) error {
	_, err := e.Dao.FindRecordById(e.Model.TableName(), e.Model.GetId())
	if err != nil {
		return err
	}
	deleteEmbeddingsForRecord(app, target, e)
	return nil
}

func modelModify(app *pocketbase.PocketBase, target string, client *genai.Client, e *core.ModelEvent) error {
	record, err := e.Dao.FindRecordById(e.Model.TableName(), e.Model.GetId())
	if err != nil {
		return err
	}
	title := record.GetString("title")
	content := record.GetString("content")

	result, err := googleAiEmbedContent(client, genai.TaskTypeRetrievalDocument, title, genai.Text(content))
	if err != nil {
		return err
	}

	vector := ""
	jsonVec, err := json.Marshal(result)
	if err != nil {
		vector = "[]"
	} else {
		vector = string(jsonVec)
	}

	deleteEmbeddingsForRecord(app, target, e)

	{
		stmt := "INSERT INTO " + target + "_embeddings (embedding) "
		stmt += "VALUES ({:embedding});"
		res, err := app.DB().NewQuery(stmt).Bind(dbx.Params{
			"embedding": vector,
		}).Execute()
		if err != nil {
			return nil
		}
		vectorId, err := res.LastInsertId()
		if err != nil {
			return err
		}
		record.Set("vector_id", vectorId)
	}

	if err := app.Dao().WithoutHooks().SaveRecord(record); err != nil {
		return err
	}
	return nil
}

func deleteEmbeddingsForRecord(app *pocketbase.PocketBase, target string, e *core.ModelEvent) error {
	record, err := e.Dao.FindRecordById(e.Model.TableName(), e.Model.GetId())
	if err != nil {
		return err
	}

	type Meta struct {
		Id string `db:"id" json:"id"`
	}
	vectorId := record.GetInt("vector_id")
	items := []*Meta{}
	stmt := "SELECT id FROM " + target + "_embeddings "
	stmt += "WHERE id = {:id};"
	err = app.DB().NewQuery(stmt).Bind(dbx.Params{
		"id": vectorId,
	}).All(&items)
	if err != nil {
		return nil
	}

	stmt = "DELETE FROM " + target + "_embeddings "
	stmt += "WHERE id = {:id}"

	for _, item := range items {
		_, err = app.DB().NewQuery(stmt).Bind(dbx.Params{
			"id": item.Id,
		}).Execute()
		if err != nil {
			return nil
		}
	}

	return nil
}

func createCollection(app *pocketbase.PocketBase, target string, extraFields ...*schema.SchemaField) error {
	fields := []*schema.SchemaField{
		{
			Name: "title",
			Type: schema.FieldTypeText,
		},
		{
			Name:     "content",
			Required: true,
			Type:     schema.FieldTypeText,
		},
		{
			Name: "vector_id",
			Type: schema.FieldTypeNumber,
		},
	}
	for i, field := range extraFields {
		options := field.Options
		if options != nil {
			relationOption, ok := options.(schema.RelationOptions)
			if ok {
				colId := relationOption.CollectionId
				if strings.HasPrefix(colId, "$$$") {
					colId = strings.ReplaceAll(colId, "$$$", "")
					if col, err := app.Dao().FindCollectionByNameOrId(colId); err != nil {
						app.Logger().Error(fmt.Sprint(err))
						return err
					} else {
						relationOption.CollectionId = col.Id
						extraFields[i].Options = relationOption
					}
				}

			}

		}
	}
	fields = append(fields, extraFields...)
	collection := &models.Collection{
		Name:   target,
		Type:   models.CollectionTypeBase,
		Schema: schema.NewSchema(fields...),
		Indexes: types.JsonArray[string]{
			"CREATE UNIQUE INDEX idx_" + target + " ON " + target + " (title, content, vector_id);",
		},
	}

	if err := app.Dao().SaveCollection(collection); err != nil {
		return err
	}

	stmt := "CREATE VIRTUAL TABLE IF NOT EXISTS " + target + "_embeddings using vec0( "
	stmt += "	id INTEGER PRIMARY KEY AUTOINCREMENT, "
	stmt += "	embedding float[768] "
	stmt += ");"
	_, err := app.DB().NewQuery(stmt).Execute()
	if err != nil {
		return nil
	}

	return nil
}
