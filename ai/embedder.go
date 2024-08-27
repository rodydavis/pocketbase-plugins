package ai

import (
	"context"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"os"
)

func CreateClient() (*genai.Client, error) {
	var apiKey string = os.Getenv("GOOGLE_AI_API_KEY")
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func CreateModel(client *genai.Client) *genai.EmbeddingModel {
	em := client.EmbeddingModel("text-embedding-004")
	return em
}

func EmbedContent(client *genai.Client, taskType genai.TaskType, title string, parts ...genai.Part) ([]float32, error) {
	ctx := context.Background()
	model := CreateModel(client)
	model.TaskType = taskType
	res, err := model.EmbedContentWithTitle(ctx, title, parts...)
	if err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func EmbedDocument(client *genai.Client, title string, parts ...genai.Part) ([]float32, error) {
	return EmbedContent(client, genai.TaskTypeRetrievalDocument, title, parts...)
}

func QueryDocument(client *genai.Client, title string, parts ...genai.Part) ([]float32, error) {
	return EmbedContent(client, genai.TaskTypeRetrievalQuery, title, parts...)
}
