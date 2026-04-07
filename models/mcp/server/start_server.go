package server

import (
	"log"
	"net/http"
	"os"

	"github.com/arcnem-ai/arcnem-vision/models/mcp/tools"
	"github.com/arcnem-ai/arcnem-vision/models/shared/env"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	env.LoadEnv()
}

func StartServer() {
	server := mcp.NewServer(&mcp.Implementation{Name: os.Getenv("MCP_SERVER_NAME"), Version: os.Getenv("MCP_SERVER_VERSION")}, nil)

	tools.RegisterCreateDocumentDescription(server)
	tools.RegisterCreateDocumentEmbedding(server)
	tools.RegisterCreateDocumentOCR(server)
	tools.RegisterCreateDocumentSegmentation(server)
	tools.RegisterCreateDescriptionEmbedding(server)
	tools.RegisterFindSimilarDocuments(server)
	tools.RegisterFindSimilarDescriptions(server)
	tools.RegisterSearchDocumentsInScope(server)
	tools.RegisterBrowseDocumentsInScope(server)
	tools.RegisterReadDocumentContext(server)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3021"
	}
	addr := ":" + port
	log.Printf("MCP streamable HTTP server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
