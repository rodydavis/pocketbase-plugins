package full_text_search

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

// https://www.sqlite.org/fts5.html#external_content_tables
func Init(app *pocketbase.PocketBase, collections ...string) error {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		for _, target := range collections {
			err := createCollectionFts(app, target)
			if err != nil {
				app.Logger().Error(fmt.Sprint(err))
				return err
			}

		}
		return nil
	})
	app.OnCollectionAfterCreateRequest().Add(func(e *core.CollectionCreateEvent) error {
		target := e.Collection.Name
		for _, col := range collections {
			if col == target {
				err := createCollectionFts(app, target)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnCollectionAfterUpdateRequest().Add(func(e *core.CollectionUpdateEvent) error {
		target := e.Collection.Name
		for _, col := range collections {
			if col == target {
				err := deleteCollection(app, target)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
				err = createCollectionFts(app, target)
				if err != nil {
					app.Logger().Error(fmt.Sprint(err))
					return err
				}
			}
		}
		return nil
	})
	app.OnCollectionAfterDeleteRequest().PreAdd(func(e *core.CollectionDeleteEvent) error {
		target := e.Collection.Name
		for _, col := range collections {
			if col == target {
				err := deleteCollection(app, target)
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
		group.GET("/full-text-search", func(c echo.Context) error {
			target := c.PathParam("collectionIdOrName")
			if _, err := app.Dao().FindCollectionByNameOrId(target); err != nil {
				app.Logger().Error(fmt.Sprint(err))
				return err
			}
			tbl := target // collection.TableName()
			q := c.QueryParam("search")
			if q == "" {
				return c.NoContent(204)
			}
			var query strings.Builder
			query.WriteString("SELECT * ") // rank as '@rank'
			query.WriteString("FROM " + tbl + "_fts ")
			// query.WriteString("INNER JOIN " + tbl + " tbl ON tbl.id = " + tbl + "_fts._id ")
			query.WriteString("WHERE " + tbl + "_fts MATCH {:q} ")
			query.WriteString("ORDER BY rank;")

			results := []dbx.NullStringMap{}
			err := app.Dao().DB().
				NewQuery(query.String()).
				Bind(dbx.Params{"q": q}).
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

func createCollectionFts(app *pocketbase.PocketBase, target string) error {
	collection, err := app.Dao().FindCollectionByNameOrId(target)
	if err != nil {
		app.Logger().Error(fmt.Sprint(err))
		return err
	}
	fields := collectionFields(collection, "id")
	exists, _ := checkIfTableExists(app, target+"_fts")

	if !exists {
		tbl := "`" + target + "`"
		var stmt strings.Builder
		stmt.WriteString("CREATE VIRTUAL TABLE " + target + "_fts USING FTS5 (")
		stmt.WriteString("  " + strings.Join(fields, ", ") + ",")
		stmt.WriteString("  content=" + target + ",")
		// stmt.WriteString("  content=''")
		// stmt.WriteString("  content_rowid='id'")
		stmt.WriteString(");")
		app.Logger().Info(stmt.String())
		if _, err := app.Dao().DB().NewQuery(stmt.String()).Execute(); err != nil {
			app.Logger().Error(fmt.Sprint(err))
			return err
		}

		stmt.Reset()
		stmt.WriteString("CREATE TRIGGER  " + target + "_fts_insert AFTER INSERT ON " + tbl + " BEGIN ")
		stmt.WriteString("  INSERT INTO " + target + "_fts(" + strings.Join(fields, ", ") + ")")
		stmt.WriteString("  VALUES (" + strings.Join(surround(fields, "new.", ""), ", ") + ");")
		stmt.WriteString("END;")
		app.Logger().Info(stmt.String())
		if _, err := app.Dao().DB().NewQuery(stmt.String()).Execute(); err != nil {
			app.Logger().Error(fmt.Sprint(err))
			return err
		}

		stmt.Reset()
		stmt.WriteString("CREATE TRIGGER  " + target + "_fts_update AFTER UPDATE ON " + tbl + " BEGIN ")
		stmt.WriteString("  INSERT INTO " + target + "_fts(" + target + "_fts, " + strings.Join(fields, ", ") + ")")
		stmt.WriteString("  VALUES ('delete', " + strings.Join(surround(fields, "old.", ""), ", ") + ");")
		stmt.WriteString("  INSERT INTO " + target + "_fts(" + strings.Join(fields, ", ") + ")")
		stmt.WriteString("  VALUES (" + strings.Join(surround(fields, "new.", ""), ", ") + ");")
		stmt.WriteString("END;")
		app.Logger().Info(stmt.String())
		if _, err := app.Dao().DB().NewQuery(stmt.String()).Execute(); err != nil {
			app.Logger().Error(fmt.Sprint(err))
			return err
		}

		stmt.Reset()
		stmt.WriteString("CREATE TRIGGER  " + target + "_fts_delete AFTER DELETE ON " + tbl + " BEGIN ")
		stmt.WriteString("  INSERT INTO " + target + "_fts(" + target + "_fts, " + strings.Join(fields, ", ") + ")")
		stmt.WriteString("  VALUES ('delete', " + strings.Join(surround(fields, "old.", ""), ", ") + ");")
		stmt.WriteString("END;")
		app.Logger().Info(stmt.String())
		if _, err := app.Dao().DB().NewQuery(stmt.String()).Execute(); err != nil {
			app.Logger().Error(fmt.Sprint(err))
			return err
		}
	}

	err = syncCollection(app, target)
	if err != nil {
		app.Logger().Error(fmt.Sprint(err))
		return err
	}

	return nil
}

func deleteCollection(app *pocketbase.PocketBase, target string) error {
	if _, err := app.Dao().DB().
		NewQuery("DELETE FROM " + target + "_fts;").
		Execute(); err != nil {
		return err
	}
	if _, err := app.Dao().DB().
		NewQuery("DROP TABLE IF EXISTS " + target + "_fts;").
		Execute(); err != nil {
		return err
	}
	return nil
}

func checkIfTableExists(app *pocketbase.PocketBase, target string) (bool, error) {
	type Meta struct {
		Name string `db:"name" json:"name"`
	}

	meta := &Meta{}

	var stmt strings.Builder
	stmt.WriteString("SELECT name FROM sqlite_master ")
	stmt.WriteString("WHERE type='table' ")
	stmt.WriteString("AND name = {:table_name};")

	app.Logger().Info(stmt.String())
	if err := app.Dao().DB().NewQuery(stmt.String()).Bind(dbx.Params{"table_name": target}).One(&meta); err != nil {
		app.Logger().Error(fmt.Sprint(err))
		return false, err
	}

	valid := meta != nil
	return valid, nil
}

func syncCollection(app *pocketbase.PocketBase, target string) error {
	var stmt strings.Builder
	stmt.WriteString("INSERT INTO " + target + "_fts(" + target + "_fts) VALUES('rebuild');")
	// stmt.WriteString("INSERT INTO " + target + "_fts SELECT " + strings.Join(fields, ", ") + " FROM " + target)
	app.Logger().Info(stmt.String())
	if _, err := app.Dao().DB().NewQuery(stmt.String()).Execute(); err != nil {
		app.Logger().Error(fmt.Sprint(err))
		return err
	}

	return nil
}

func collectionFields(collection *models.Collection, id string) []string {
	fields := []string{id}
	for _, field := range collection.Schema.Fields() {
		name := field.Name
		fields = append(fields, name)
	}
	return fields
}

func surround(items []string, prefix string, suffix string) []string {
	results := []string{}
	for i := 0; i < len(items); i++ {
		item := items[i]
		results = append(results, prefix+item+suffix)
	}
	return results
}
