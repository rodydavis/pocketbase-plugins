package plugins

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func VectorStore(app *pocketbase.PocketBase, collections ...string) {
	// core.DBConnectHooks = append(core.DBConnectHooks, func(conn *sqlite3.SQLiteConn) error {
	// if err := conn.RegisterFunc("convert_vectors", ConvertVectors, false); err != nil {
	// 	return err
	// }
	// for _, stmt := range []string{
	// 	`CREATE VIRTUAL TABLE IF NOT EXISTS vector_store_embeddings using vec0(
	// 			id INTEGER PRIMARY KEY AUTOINCREMENT,
	// 			embedding float[768]
	// 		);`,
	// 	`CREATE TABLE IF NOT EXISTS _vector_store (
	// 			id INTEGER PRIMARY KEY AUTOINCREMENT,
	// 			embedding_id INTEGER NOT NULL REFERENCES vector_store_embeddings(id),
	// 			vector_id TEXT NOT NULL REFERENCES vector_store(id),
	// 			UNIQUE(embedding_id, vector_id) ON CONFLICT REPLACE
	// 		);`,
	// 	``,
	// } {
	// 	if _, err := conn.Exec(stmt, nil); err != nil {
	// 		return err
	// 	}
	// }

	// 	return nil
	// })

	app.OnAfterBootstrap().Add(func(e *core.BootstrapEvent) error {
		type Info struct {
			Version string `db:"version" json:"version"`
		}

		items := []Info{}
		err := app.DB().NewQuery("select vec_version() as version").All(&items)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("vec_version=%s\n", items)

		err = app.DB().NewQuery("select sqlite_version() as version").All(&items)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("sqlite_version=%s\n", items)
		return nil
	})

	// TODO: Add hooks and routes
}
