package hooks

import (
	"encoding/json"
	"log"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/google/generative-ai-go/genai"
	"github.com/mattn/go-sqlite3"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/rodydavis/pocketbase_extensions/ai"
)

func ConvertVectors(vectors []float32) string {
	// bytes, err := sqlite_vec.SerializeFloat32(vectors)
	// if err != nil {
	// 	return []byte{}
	// }
	jsonVec, err := json.Marshal(vectors)
	if err != nil {
		return "[]"
	}
	return string(jsonVec)
}

func InitVectorHooks(client *genai.Client, app *pocketbase.PocketBase) {
	app.OnModelAfterCreate(vectorStoreCollection).Add(modelModify(client, app))
	app.OnModelAfterUpdate(vectorStoreCollection).Add(modelModify(client, app))

	for i := 0; i < len(createStmt); i++ {
		stmt := createStmt[i]
		_, err := app.Dao().DB().NewQuery(stmt).Execute()
		if err != nil {
			log.Fatal(err)
		}
	}

	core.DBConnectHooks = append(core.DBConnectHooks, func(conn *sqlite3.SQLiteConn) error {
		if err := conn.RegisterFunc("convert_vectors", ConvertVectors, false); err != nil {
			return err
		}
		return nil
	})

}

var vectorStoreCollection = "vector_store"

var createStmt = []string{
	`CREATE VIRTUAL TABLE IF NOT EXISTS vector_store_embeddings using vec0(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		embedding float[768]
	);`,
	`CREATE TABLE IF NOT EXISTS _vector_store (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		embedding_id INTEGER NOT NULL REFERENCES vector_store_embeddings(id),
		vector_id TEXT NOT NULL REFERENCES vector_store(id),
		UNIQUE(embedding_id, vector_id) ON CONFLICT REPLACE
	);`,
	``,
}

// var insertStmt = `
// INSERT INTO vector_store_embeddings (embedding)
// VALUES (:embedding)
// `

func modelModify(client *genai.Client, app *pocketbase.PocketBase) func(e *core.ModelEvent) error {
	return func(e *core.ModelEvent) error {
		record, err := e.Dao.FindRecordById(e.Model.TableName(), e.Model.GetId())
		if err != nil {
			return err
		}
		title := record.GetString("title")
		content := record.GetString("content")
		taskType := record.GetString("task_type")
		resolvedTaskType := genai.TaskTypeUnspecified
		if taskType != "" {
			resolvedTaskType = ai.TaskTypes[taskType]
		}
		result, err := ai.EmbedContent(client, resolvedTaskType, title, genai.Text(content))
		if err != nil {
			return err
		}
		v, err := sqlite_vec.SerializeFloat32(result)
		if err != nil {
			return err
		}
		record.Set("vector", result)
		record.Set("bytes", v)
		if err := app.Dao().WithoutHooks().SaveRecord(record); err != nil {
			return err
		}
		return nil
	}
}
