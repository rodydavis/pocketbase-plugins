package ai

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/generative-ai-go/genai"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func EmbedderRoutes(client *genai.Client, e *core.ServeEvent, pb *pocketbase.PocketBase) {
	group := e.Router.Group("/embed", apis.ActivityLogger(pb)) // apis.RequireAdminAuth()
	group.POST("/content", embedRequest(client, genai.TaskTypeUnspecified))
	group.POST("/document", embedRequest(client, genai.TaskTypeRetrievalDocument))
	group.POST("/query", embedRequest(client, genai.TaskTypeRetrievalQuery))
	group.POST("/search", searchVectors(client, pb))
}

var searchStmt = `
SELECT id, distance
FROM vector_store_embeddings
WHERE embedding MATCH {:embedding}
AND k = {:k};
`

func searchVectors(client *genai.Client, pb *pocketbase.PocketBase) func(echo.Context) error {
	type Result struct {
		Id int `db:"id" json:"id"`
		Distance float32 `db:"distance" json:"distance"`
	}

	return func(c echo.Context) error {
		contentType := c.Request().Header.Get(echo.HeaderContentType)
		if contentType != "application/json" {
			return c.String(400, "content-type must be of type application/json")
		}
		body, err := readJson(c)
		if err != nil {
			return c.String(400, "error reading json body: "+fmt.Sprint(err))
		}
		go syncVectors(pb)
		content := body["content"].(string)
		title := ""
		titleVal, ok := body["title"]
		if ok {
			titleStr, ok := titleVal.(string)
			if ok {
				title = titleStr
			}
		}
		vector, err := QueryDocument(client, title, genai.Text(content))
		if err != nil {
			return err
		}
		// v, err := sqlite_vec.SerializeFloat32(vector)
		// if err != nil {
		// 	return err
		// }

		jsonVec, err := json.Marshal(vector)
		if err != nil {
			return err
		}

		results := []Result{}
		err = pb.Dao().DB().
			NewQuery(searchStmt).
			Bind(dbx.Params{
				"embedding": string(jsonVec),
				"k":         20,
			}).
			All(&results)

		if err != nil {
			return err
		}
		return c.JSON(200, results)
	}
}

func syncVectors(pb *pocketbase.PocketBase) error {
	query := pb.App.Dao().RecordQuery("vector_store")
	records := []*models.Record{}
	if err := query.All(&records); err != nil {
		return err
	}

	var deleteStmt = `
	DELETE FROM vector_store_embeddings;
	`

	_, err := pb.Dao().DB().
		NewQuery(deleteStmt).
		Execute()
	if err != nil {
		return err
	}

	var syncStmt = `
	INSERT INTO vector_store_embeddings (embedding)
	SELECT json(vector) as json_vector
	FROM vector_store;
	`

	_, err = pb.Dao().DB().
		NewQuery(syncStmt).
		Execute()
	if err != nil {
		pb.Logger().Info("error sync: " + fmt.Sprint(err))
		return err
	}

	// var insertStmt = `
	// INSERT INTO vector_store_embeddings (embedding)
	// VALUES ({:embedding});
	// `

	// for i := 0; i < len(records); i++ {
	// 	record := records[i]
	// 	vector := make([]float32, 0)
	// 	record.UnmarshalJSONField("vector", &vector)
	// 	// v, err := sqlite_vec.SerializeFloat32(vector)
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// 	jsonVec, err := json.Marshal(vector)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	_, err = pb.Dao().DB().
	// 		NewQuery(insertStmt).
	// 		Bind(dbx.Params{"embedding": string(jsonVec)}).
	// 		Execute()
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func embedRequest(client *genai.Client, defaultTaskType genai.TaskType) func(echo.Context) error {
	return func(c echo.Context) error {
		contentType := c.Request().Header.Get(echo.HeaderContentType)
		if contentType != "application/json" {
			return c.String(400, "content-type must be of type application/json")
		}
		body, err := readJson(c)
		if err != nil {
			return c.String(400, "error reading json body: "+fmt.Sprint(err))
		}
		taskType := defaultTaskType
		t, ok := body["task_type"]
		if ok {
			val, ok := t.(string)
			if ok {

				taskType = TaskTypes[val]
			}
		}
		title := ""
		titleVal, ok := body["title"]
		if ok {
			titleStr, ok := titleVal.(string)
			if ok {
				title = titleStr
			}
		}
		content := body["content"].(string)
		result, err := EmbedContent(client, taskType, title, genai.Text(content))
		if err != nil {
			return c.String(500, "error reading embedding content: "+fmt.Sprint(err))
		}
		c.Response().Header().Set(echo.HeaderContentType, "application/json")
		m := make(map[string]interface{})
		m["vector"] = result
		return c.JSON(200, m)
	}
}

func readJson(c echo.Context) (map[string]any, error) {
	b, _ := io.ReadAll(c.Request().Body)
	m := make(map[string]interface{})
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

var TaskTypes = map[string]genai.TaskType{
	"Unspecified":        genai.TaskTypeUnspecified,
	"RetrievalQuery":     genai.TaskTypeRetrievalQuery,
	"RetrievalDocument":  genai.TaskTypeRetrievalDocument,
	"SemanticSimilarity": genai.TaskTypeSemanticSimilarity,
	"Classification":     genai.TaskTypeClassification,
	"Clustering":         genai.TaskTypeClustering,
	"QuestionAnswering":  genai.TaskTypeQuestionAnswering,
	"FactVerification":   genai.TaskTypeFactVerification,
}
