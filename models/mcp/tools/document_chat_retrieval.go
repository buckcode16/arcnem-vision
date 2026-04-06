package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	dbclient "github.com/arcnem-ai/arcnem-vision/models/db/client"
	dbmodels "github.com/arcnem-ai/arcnem-vision/models/db/gen/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultDocumentSearchLimit = 5
const maxDocumentSearchLimit = 8
const hybridSearchCandidateFloor = 8
const maxHybridSearchCandidateLimit = 18
const reciprocalRankFusionOffset = 50.0
const lexicalSearchWeight = 1.25
const descriptionSemanticSearchWeight = 1.0

type documentSearchRow struct {
	DocumentID      string    `gorm:"column:document_id"`
	ObjectKey       string    `gorm:"column:object_key"`
	ContentType     string    `gorm:"column:content_type"`
	SizeBytes       int64     `gorm:"column:size_bytes"`
	ProjectID       string    `gorm:"column:project_id"`
	ProjectName     string    `gorm:"column:project_name"`
	DeviceID        *string   `gorm:"column:device_id"`
	DeviceName      *string   `gorm:"column:device_name"`
	DescriptionText string    `gorm:"column:description_text"`
	OCRText         string    `gorm:"column:ocr_text"`
	Score           float64   `gorm:"column:score"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

type documentContextRow struct {
	DocumentID      string     `gorm:"column:document_id"`
	ObjectKey       string     `gorm:"column:object_key"`
	ProjectID       string     `gorm:"column:project_id"`
	ProjectName     string     `gorm:"column:project_name"`
	DeviceID        *string    `gorm:"column:device_id"`
	DeviceName      *string    `gorm:"column:device_name"`
	DescriptionText string     `gorm:"column:description_text"`
	OCRText         string     `gorm:"column:ocr_text"`
	OCRCreatedAt    *time.Time `gorm:"column:ocr_created_at"`
	OCRModelLabel   *string    `gorm:"column:ocr_model_label"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
}

type segmentationContextRow struct {
	SourceDocumentID string    `gorm:"column:source_document_id"`
	SegmentationID   string    `gorm:"column:segmentation_id"`
	Prompt           *string   `gorm:"column:prompt"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	ModelLabel       string    `gorm:"column:model_label"`
	DescriptionText  string    `gorm:"column:description_text"`
	OCRText          string    `gorm:"column:ocr_text"`
	ObjectKey        *string   `gorm:"column:object_key"`
}

type documentSearchCandidate struct {
	Row                documentSearchRow
	DefaultMatchReason string
}

type documentSearchSignalSet struct {
	Rows               []documentSearchRow
	DefaultMatchReason string
	Weight             float64
}

func RegisterSearchDocumentsInScope(server *mcp.Server) {
	db := dbclient.NewPGClient()

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_documents_in_scope",
		Description: "Search top-level documents within an authenticated scope using OCR text, descriptions, and metadata.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchDocumentsInScopeInput) (*mcp.CallToolResult, SearchDocumentsInScopeOutput, error) {
		matches, err := searchDocumentsInScope(ctx, db, input)
		if err != nil {
			return nil, SearchDocumentsInScopeOutput{}, err
		}

		out := SearchDocumentsInScopeOutput{Matches: matches}
		outJSON, _ := json.Marshal(out)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(outJSON)}},
		}, out, nil
	})
}

func RegisterBrowseDocumentsInScope(server *mcp.Server) {
	db := dbclient.NewPGClient()

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browse_documents_in_scope",
		Description: "Browse recent top-level documents within an authenticated scope using metadata, OCR text, and descriptions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input BrowseDocumentsInScopeInput) (*mcp.CallToolResult, SearchDocumentsInScopeOutput, error) {
		matches, err := browseDocumentsInScope(ctx, db, input)
		if err != nil {
			return nil, SearchDocumentsInScopeOutput{}, err
		}

		out := SearchDocumentsInScopeOutput{Matches: matches}
		outJSON, _ := json.Marshal(out)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(outJSON)}},
		}, out, nil
	})
}

func RegisterReadDocumentContext(server *mcp.Server) {
	db := dbclient.NewPGClient()

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_document_context",
		Description: "Read normalized context for top-level documents in an authenticated scope, including metadata, OCR excerpts, and related segmentation excerpts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ReadDocumentContextInput) (*mcp.CallToolResult, ReadDocumentContextOutput, error) {
		documents, err := readDocumentContext(ctx, db, input)
		if err != nil {
			return nil, ReadDocumentContextOutput{}, err
		}

		out := ReadDocumentContextOutput{Documents: documents}
		outJSON, _ := json.Marshal(out)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(outJSON)}},
		}, out, nil
	})
}

func searchDocumentsInScope(ctx context.Context, db *gorm.DB, input SearchDocumentsInScopeInput) ([]DocumentSearchMatch, error) {
	normalizedQuery := strings.TrimSpace(input.Query)
	if normalizedQuery == "" {
		return []DocumentSearchMatch{}, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultDocumentSearchLimit
	}
	if limit > maxDocumentSearchLimit {
		limit = maxDocumentSearchLimit
	}

	candidateLimit := expandedDocumentSearchLimit(limit)
	signalSets := make([]documentSearchSignalSet, 0, 2)

	lexicalRows, err := runLexicalDocumentSearch(
		ctx,
		db,
		input.Scope,
		normalizedQuery,
		candidateLimit,
	)
	if err != nil {
		return nil, err
	}
	if len(lexicalRows) > 0 {
		signalSets = append(signalSets, documentSearchSignalSet{
			Rows:               lexicalRows,
			DefaultMatchReason: "",
			Weight:             lexicalSearchWeight,
		})
	}

	queryEmbedding, err := buildSemanticQueryEmbedding(ctx, normalizedQuery)
	if err != nil {
		log.Printf(
			"search_documents_in_scope semantic embedding unavailable query=%q err=%v",
			normalizedQuery,
			err,
		)
	} else if queryEmbedding != "" {
		model, modelErr := ensureOpenAITextEmbeddingModel(db)
		if modelErr != nil {
			log.Printf(
				"search_documents_in_scope semantic model unavailable query=%q err=%v",
				normalizedQuery,
				modelErr,
			)
		} else {
			if embedErr := ensureLatestDescriptionEmbeddingsInScope(
				ctx,
				db,
				input.Scope,
				model,
			); embedErr != nil {
				log.Printf(
					"search_documents_in_scope semantic backfill failed query=%q err=%v",
					normalizedQuery,
					embedErr,
				)
			}

			descriptionSemanticRows, descErr := runDescriptionSemanticSearch(
				ctx,
				db,
				input.Scope,
				queryEmbedding,
				model,
				candidateLimit,
			)
			if descErr != nil {
				log.Printf(
					"search_documents_in_scope description semantic search failed query=%q err=%v",
					normalizedQuery,
					descErr,
				)
			} else if len(descriptionSemanticRows) > 0 {
				signalSets = append(signalSets, documentSearchSignalSet{
					Rows:               descriptionSemanticRows,
					DefaultMatchReason: "semantic description match",
					Weight:             descriptionSemanticSearchWeight,
				})
			}
		}
	}

	if len(signalSets) == 0 {
		return []DocumentSearchMatch{}, nil
	}

	fusedCandidates := fuseDocumentSearchSignalSets(signalSets, limit)
	return buildDocumentSearchMatchesFromCandidates(fusedCandidates, normalizedQuery), nil
}

func runLexicalDocumentSearch(ctx context.Context, db *gorm.DB, scope ChatDocumentScope, normalizedQuery string, limit int) ([]documentSearchRow, error) {
	filters, args := buildDocumentScopeFilters("d", scope)
	searchableText := `
		COALESCE(latest_description.text, '') || ' ' ||
		COALESCE(latest_ocr.text, '') || ' ' ||
		COALESCE(d.object_key, '') || ' ' ||
		COALESCE(p.name, '') || ' ' ||
		COALESCE(dev.name, '')
	`

	query := fmt.Sprintf(`
		SELECT
			d.id AS document_id,
			d.object_key,
			d.content_type,
			d.size_bytes,
			d.project_id,
			p.name AS project_name,
			d.device_id,
			dev.name AS device_name,
			COALESCE(latest_description.text, '') AS description_text,
			COALESCE(latest_ocr.text, '') AS ocr_text,
			GREATEST(
				ts_rank_cd(
					to_tsvector('english', %s),
					websearch_to_tsquery('english', ?)
				),
				ts_rank_cd(
					to_tsvector('simple', %s),
					websearch_to_tsquery('simple', ?)
				)
			) AS score,
			d.created_at
		FROM documents d
		INNER JOIN projects p ON p.id = d.project_id
		LEFT JOIN devices dev ON dev.id = d.device_id
		LEFT JOIN LATERAL (
			SELECT dd.id, dd.text
			FROM document_descriptions dd
			WHERE dd.document_id = d.id
			ORDER BY dd.created_at DESC
			LIMIT 1
		) latest_description ON TRUE
		LEFT JOIN LATERAL (
			SELECT dor.text
			FROM document_ocr_results dor
			WHERE dor.document_id = d.id
			ORDER BY dor.created_at DESC
			LIMIT 1
		) latest_ocr ON TRUE
		WHERE NOT EXISTS (
			SELECT 1
			FROM document_segmentations ds_hidden
			WHERE ds_hidden.segmented_document_id = d.id
		)
			AND %s
			AND (
				to_tsvector('english', %s) @@ websearch_to_tsquery('english', ?)
				OR to_tsvector('simple', %s) @@ websearch_to_tsquery('simple', ?)
			)
		ORDER BY score DESC, d.created_at DESC
		LIMIT ?
	`, searchableText, searchableText, filters, searchableText, searchableText)

	var rows []documentSearchRow
	queryArgs := append([]any{normalizedQuery, normalizedQuery}, args...)
	queryArgs = append(queryArgs, normalizedQuery, normalizedQuery, limit)
	if err := db.WithContext(ctx).Raw(query, queryArgs...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("search_documents_in_scope lexical query failed: %w", err)
	}

	return rows, nil
}

func buildSemanticQueryEmbedding(ctx context.Context, query string) (string, error) {
	semanticQuery := normalizeEmbeddingText(query)
	if semanticQuery == "" {
		return "", nil
	}

	return buildOpenAITextEmbedding(ctx, semanticQuery)
}

func ensureOpenAITextEmbeddingModel(db *gorm.DB) (*dbmodels.Model, error) {
	modelType := "embedding"
	embeddingDim := int32(openAITextEmbeddingDim)
	return ensureModelByIdentity(
		db,
		openAITextEmbeddingProvider,
		openAITextEmbeddingModelName,
		"",
		&modelType,
		&embeddingDim,
	)
}

type missingDescriptionEmbeddingRow struct {
	DocumentDescriptionID string `gorm:"column:document_description_id"`
	Text                  string `gorm:"column:text"`
}

func ensureLatestDescriptionEmbeddingsInScope(
	ctx context.Context,
	db *gorm.DB,
	scope ChatDocumentScope,
	model *dbmodels.Model,
) error {
	filters, args := buildDocumentScopeFilters("d", scope)
	query := fmt.Sprintf(`
		SELECT
			latest_description.id AS document_description_id,
			latest_description.text
		FROM documents d
		INNER JOIN LATERAL (
			SELECT dd.id, dd.text
			FROM document_descriptions dd
			WHERE dd.document_id = d.id
			ORDER BY dd.created_at DESC
			LIMIT 1
		) latest_description ON TRUE
		LEFT JOIN document_description_embeddings dde
			ON dde.document_description_id = latest_description.id
			AND dde.model_id = ?
			AND dde.embedding_dim = ?
		WHERE NOT EXISTS (
			SELECT 1
			FROM document_segmentations ds_hidden
			WHERE ds_hidden.segmented_document_id = d.id
		)
			AND %s
			AND dde.id IS NULL
		ORDER BY d.created_at DESC
	`, filters)

	var rows []missingDescriptionEmbeddingRow
	queryArgs := append([]any{model.ID, openAITextEmbeddingDim}, args...)
	if err := db.WithContext(ctx).Raw(query, queryArgs...).Scan(&rows).Error; err != nil {
		return fmt.Errorf(
			"search_documents_in_scope missing description embeddings query failed: %w",
			err,
		)
	}
	if len(rows) == 0 {
		return nil
	}

	texts := make([]string, 0, len(rows))
	pendingRows := make([]missingDescriptionEmbeddingRow, 0, len(rows))
	for _, row := range rows {
		normalized := normalizeEmbeddingText(row.Text)
		if normalized == "" {
			continue
		}
		pendingRows = append(pendingRows, row)
		texts = append(texts, normalized)
	}
	if len(pendingRows) == 0 {
		return nil
	}

	embeddings, err := buildOpenAITextEmbeddings(ctx, texts)
	if err != nil {
		return err
	}
	if len(embeddings) != len(pendingRows) {
		return fmt.Errorf(
			"OpenAI embedding count mismatch: expected %d, got %d",
			len(pendingRows),
			len(embeddings),
		)
	}

	records := make([]dbmodels.DocumentDescriptionEmbedding, 0, len(pendingRows))
	for index, row := range pendingRows {
		records = append(records, dbmodels.DocumentDescriptionEmbedding{
			DocumentDescriptionID: row.DocumentDescriptionID,
			ModelID:               model.ID,
			EmbeddingDim:          openAITextEmbeddingDim,
			Embedding:             embeddings[index],
		})
	}

	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "document_description_id"},
			{Name: "model_id"},
			{Name: "embedding_dim"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"embedding", "updated_at"}),
	}).Create(&records).Error; err != nil {
		return fmt.Errorf("failed to save OpenAI description embeddings: %w", err)
	}

	return nil
}

func runDescriptionSemanticSearch(
	ctx context.Context,
	db *gorm.DB,
	scope ChatDocumentScope,
	queryEmbedding string,
	model *dbmodels.Model,
	limit int,
) ([]documentSearchRow, error) {
	filters, args := buildDocumentScopeFilters("d", scope)
	query := fmt.Sprintf(`
		SELECT
			d.id AS document_id,
			d.object_key,
			d.content_type,
			d.size_bytes,
			d.project_id,
			p.name AS project_name,
			d.device_id,
			dev.name AS device_name,
			COALESCE(latest_description.text, '') AS description_text,
			COALESCE(latest_ocr.text, '') AS ocr_text,
			GREATEST(0.0, 1 - (dde.embedding <=> ?::vector)) AS score,
			d.created_at
		FROM documents d
		INNER JOIN projects p ON p.id = d.project_id
		LEFT JOIN devices dev ON dev.id = d.device_id
		LEFT JOIN LATERAL (
			SELECT dd.id, dd.text
			FROM document_descriptions dd
			WHERE dd.document_id = d.id
			ORDER BY dd.created_at DESC
			LIMIT 1
		) latest_description ON TRUE
		INNER JOIN document_description_embeddings dde
			ON dde.document_description_id = latest_description.id
			AND dde.model_id = ?
			AND dde.embedding_dim = ?
		LEFT JOIN LATERAL (
			SELECT dor.text
			FROM document_ocr_results dor
			WHERE dor.document_id = d.id
			ORDER BY dor.created_at DESC
			LIMIT 1
		) latest_ocr ON TRUE
		WHERE NOT EXISTS (
			SELECT 1
			FROM document_segmentations ds_hidden
			WHERE ds_hidden.segmented_document_id = d.id
		)
			AND %s
		ORDER BY dde.embedding <=> ?::vector ASC, d.created_at DESC
		LIMIT ?
	`, filters)

	var rows []documentSearchRow
	queryArgs := []any{queryEmbedding, model.ID, openAITextEmbeddingDim}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, queryEmbedding, limit)
	if err := db.WithContext(ctx).Raw(query, queryArgs...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("search_documents_in_scope description semantic query failed: %w", err)
	}

	return rows, nil
}

func expandedDocumentSearchLimit(limit int) int {
	candidateLimit := limit * 3
	if candidateLimit < hybridSearchCandidateFloor {
		candidateLimit = hybridSearchCandidateFloor
	}
	if candidateLimit > maxHybridSearchCandidateLimit {
		candidateLimit = maxHybridSearchCandidateLimit
	}
	return candidateLimit
}

func fuseDocumentSearchSignalSets(signalSets []documentSearchSignalSet, limit int) []documentSearchCandidate {
	type fusedDocumentScore struct {
		Row                   documentSearchRow
		Score                 float64
		DefaultMatchReason    string
		StrongestContribution float64
	}

	byDocumentID := map[string]*fusedDocumentScore{}
	for _, signalSet := range signalSets {
		for index, row := range signalSet.Rows {
			if strings.TrimSpace(row.DocumentID) == "" {
				continue
			}

			contribution := signalSet.Weight / (reciprocalRankFusionOffset + float64(index+1))
			accumulator, ok := byDocumentID[row.DocumentID]
			if !ok {
				row.Score = 0
				accumulator = &fusedDocumentScore{Row: row}
				byDocumentID[row.DocumentID] = accumulator
			} else {
				accumulator.Row = mergeDocumentSearchRows(accumulator.Row, row)
			}

			accumulator.Score += contribution
			if contribution > accumulator.StrongestContribution {
				accumulator.StrongestContribution = contribution
				accumulator.DefaultMatchReason = signalSet.DefaultMatchReason
			}
		}
	}

	candidates := make([]documentSearchCandidate, 0, len(byDocumentID))
	for _, candidate := range byDocumentID {
		candidate.Row.Score = candidate.Score
		candidates = append(candidates, documentSearchCandidate{
			Row:                candidate.Row,
			DefaultMatchReason: candidate.DefaultMatchReason,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Row.Score == candidates[j].Row.Score {
			return candidates[i].Row.CreatedAt.After(candidates[j].Row.CreatedAt)
		}
		return candidates[i].Row.Score > candidates[j].Row.Score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates
}

func mergeDocumentSearchRows(current documentSearchRow, incoming documentSearchRow) documentSearchRow {
	if strings.TrimSpace(current.ObjectKey) == "" && strings.TrimSpace(incoming.ObjectKey) != "" {
		current.ObjectKey = incoming.ObjectKey
	}
	if strings.TrimSpace(current.ContentType) == "" && strings.TrimSpace(incoming.ContentType) != "" {
		current.ContentType = incoming.ContentType
	}
	if current.SizeBytes == 0 && incoming.SizeBytes > 0 {
		current.SizeBytes = incoming.SizeBytes
	}
	if strings.TrimSpace(current.ProjectID) == "" && strings.TrimSpace(incoming.ProjectID) != "" {
		current.ProjectID = incoming.ProjectID
	}
	if strings.TrimSpace(current.ProjectName) == "" && strings.TrimSpace(incoming.ProjectName) != "" {
		current.ProjectName = incoming.ProjectName
	}
	if current.DeviceID == nil && incoming.DeviceID != nil {
		current.DeviceID = incoming.DeviceID
	}
	if current.DeviceName == nil && incoming.DeviceName != nil {
		current.DeviceName = incoming.DeviceName
	}
	if strings.TrimSpace(current.DescriptionText) == "" && strings.TrimSpace(incoming.DescriptionText) != "" {
		current.DescriptionText = incoming.DescriptionText
	}
	if strings.TrimSpace(current.OCRText) == "" && strings.TrimSpace(incoming.OCRText) != "" {
		current.OCRText = incoming.OCRText
	}
	if incoming.CreatedAt.After(current.CreatedAt) {
		current.CreatedAt = incoming.CreatedAt
	}
	return current
}

func browseDocumentsInScope(ctx context.Context, db *gorm.DB, input BrowseDocumentsInScopeInput) ([]DocumentSearchMatch, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = defaultDocumentSearchLimit
	}
	if limit > maxDocumentSearchLimit {
		limit = maxDocumentSearchLimit
	}

	filters, args := buildDocumentScopeFilters("d", input.Scope)
	query := fmt.Sprintf(`
		SELECT
			d.id AS document_id,
			d.object_key,
			d.content_type,
			d.size_bytes,
			d.project_id,
			p.name AS project_name,
			d.device_id,
			dev.name AS device_name,
			COALESCE(latest_description.text, '') AS description_text,
			COALESCE(latest_ocr.text, '') AS ocr_text,
			EXTRACT(EPOCH FROM d.created_at)::float8 AS score,
			d.created_at
		FROM documents d
		INNER JOIN projects p ON p.id = d.project_id
		LEFT JOIN devices dev ON dev.id = d.device_id
		LEFT JOIN LATERAL (
			SELECT dd.text
			FROM document_descriptions dd
			WHERE dd.document_id = d.id
			ORDER BY dd.created_at DESC
			LIMIT 1
		) latest_description ON TRUE
		LEFT JOIN LATERAL (
			SELECT dor.text
			FROM document_ocr_results dor
			WHERE dor.document_id = d.id
			ORDER BY dor.created_at DESC
			LIMIT 1
		) latest_ocr ON TRUE
		WHERE NOT EXISTS (
			SELECT 1
			FROM document_segmentations ds_hidden
			WHERE ds_hidden.segmented_document_id = d.id
		)
			AND %s
		ORDER BY d.created_at DESC
		LIMIT ?
	`, filters)

	var rows []documentSearchRow
	queryArgs := append(args, limit)
	if err := db.WithContext(ctx).Raw(query, queryArgs...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("browse_documents_in_scope query failed: %w", err)
	}

	return buildDocumentSearchMatches(rows, "", "recent document"), nil
}

func buildDocumentSearchMatches(rows []documentSearchRow, query string, defaultMatchReason string) []DocumentSearchMatch {
	matches := make([]DocumentSearchMatch, 0, len(rows))
	for _, row := range rows {
		label := buildDocumentLabel(row.ObjectKey, row.DocumentID)
		snippet, matchReason := buildSearchSnippet(query, row.DescriptionText, row.OCRText, row.ObjectKey, row.ProjectName, row.DeviceName)
		if snippet == "" {
			snippet = buildPreferredExcerpt(row.DescriptionText, row.OCRText, row.ObjectKey)
		}
		if defaultMatchReason != "" && (strings.TrimSpace(query) == "" || matchReason == "document context") {
			matchReason = defaultMatchReason
		}

		citation := DocumentChatCitation{
			DocumentID:  row.DocumentID,
			ProjectID:   row.ProjectID,
			ProjectName: row.ProjectName,
			DeviceID:    row.DeviceID,
			DeviceName:  row.DeviceName,
			Label:       label,
			Excerpt:     snippet,
			MatchReason: matchReason,
		}

		matches = append(matches, DocumentSearchMatch{
			DocumentID:  row.DocumentID,
			ObjectKey:   row.ObjectKey,
			ContentType: row.ContentType,
			SizeBytes:   row.SizeBytes,
			CreatedAt:   row.CreatedAt.Format(time.RFC3339),
			ProjectID:   row.ProjectID,
			ProjectName: row.ProjectName,
			DeviceID:    row.DeviceID,
			DeviceName:  row.DeviceName,
			Label:       label,
			Snippet:     snippet,
			MatchReason: matchReason,
			Score:       row.Score,
			Citation:    citation,
		})
	}

	return matches
}

func buildDocumentSearchMatchesFromCandidates(candidates []documentSearchCandidate, query string) []DocumentSearchMatch {
	rows := make([]documentSearchRow, 0, len(candidates))
	defaultReasons := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, candidate.Row)
		defaultReasons = append(defaultReasons, candidate.DefaultMatchReason)
	}

	matches := make([]DocumentSearchMatch, 0, len(rows))
	for index, row := range rows {
		defaultMatchReason := defaultReasons[index]
		label := buildDocumentLabel(row.ObjectKey, row.DocumentID)
		snippet, matchReason := buildSearchSnippet(
			query,
			row.DescriptionText,
			row.OCRText,
			row.ObjectKey,
			row.ProjectName,
			row.DeviceName,
		)
		if snippet == "" {
			snippet = buildPreferredExcerpt(row.DescriptionText, row.OCRText, row.ObjectKey)
		}
		if defaultMatchReason != "" && matchReason == "document context" {
			matchReason = defaultMatchReason
		}

		citation := DocumentChatCitation{
			DocumentID:  row.DocumentID,
			ProjectID:   row.ProjectID,
			ProjectName: row.ProjectName,
			DeviceID:    row.DeviceID,
			DeviceName:  row.DeviceName,
			Label:       label,
			Excerpt:     snippet,
			MatchReason: matchReason,
		}

		matches = append(matches, DocumentSearchMatch{
			DocumentID:  row.DocumentID,
			ObjectKey:   row.ObjectKey,
			ContentType: row.ContentType,
			SizeBytes:   row.SizeBytes,
			CreatedAt:   row.CreatedAt.Format(time.RFC3339),
			ProjectID:   row.ProjectID,
			ProjectName: row.ProjectName,
			DeviceID:    row.DeviceID,
			DeviceName:  row.DeviceName,
			Label:       label,
			Snippet:     snippet,
			MatchReason: matchReason,
			Score:       row.Score,
			Citation:    citation,
		})
	}

	return matches
}

func readDocumentContext(ctx context.Context, db *gorm.DB, input ReadDocumentContextInput) ([]DocumentContextItem, error) {
	documentIDs := dedupeNonEmpty(input.DocumentIDs)
	if len(documentIDs) == 0 {
		return []DocumentContextItem{}, nil
	}

	scope := input.Scope
	scope.DocumentIDs = intersectDocumentIDs(scope.DocumentIDs, documentIDs)
	if len(scope.DocumentIDs) == 0 {
		scope.DocumentIDs = documentIDs
	}

	filters, args := buildDocumentScopeFilters("d", scope)
	query := fmt.Sprintf(`
		SELECT
			d.id AS document_id,
			d.object_key,
			d.project_id,
			p.name AS project_name,
			d.device_id,
			dev.name AS device_name,
			COALESCE(latest_description.text, '') AS description_text,
			COALESCE(latest_ocr.text, '') AS ocr_text,
			latest_ocr.created_at AS ocr_created_at,
			latest_ocr.model_label AS ocr_model_label,
			d.created_at
		FROM documents d
		INNER JOIN projects p ON p.id = d.project_id
		LEFT JOIN devices dev ON dev.id = d.device_id
		LEFT JOIN LATERAL (
			SELECT dd.text
			FROM document_descriptions dd
			WHERE dd.document_id = d.id
			ORDER BY dd.created_at DESC
			LIMIT 1
		) latest_description ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				dor.text,
				dor.created_at,
				TRIM(BOTH ' /' FROM COALESCE(m.provider, '') || ' / ' || COALESCE(m.name, '')) AS model_label
			FROM document_ocr_results dor
			LEFT JOIN models m ON m.id = dor.model_id
			WHERE dor.document_id = d.id
			ORDER BY dor.created_at DESC
			LIMIT 1
		) latest_ocr ON TRUE
		WHERE NOT EXISTS (
			SELECT 1
			FROM document_segmentations ds_hidden
			WHERE ds_hidden.segmented_document_id = d.id
		)
			AND %s
		ORDER BY d.created_at DESC
	`, filters)

	var rows []documentContextRow
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read_document_context documents query failed: %w", err)
	}

	if len(rows) == 0 {
		return []DocumentContextItem{}, nil
	}

	segmentationRows, err := loadSegmentationContext(ctx, db, rows)
	if err != nil {
		return nil, err
	}
	segmentationBySource := map[string][]DocumentSegmentationExcerpt{}
	for _, row := range segmentationRows {
		segmentationBySource[row.SourceDocumentID] = append(segmentationBySource[row.SourceDocumentID], buildSegmentationExcerpt(row))
	}

	documents := make([]DocumentContextItem, 0, len(rows))
	for _, row := range rows {
		label := buildDocumentLabel(row.ObjectKey, row.DocumentID)
		description := clipText(row.DescriptionText, 420)
		ocrExcerpts := buildOCRExcerpts(row)
		citationExcerpt := buildPreferredExcerpt(row.DescriptionText, row.OCRText, row.ObjectKey)

		documents = append(documents, DocumentContextItem{
			DocumentID:           row.DocumentID,
			ProjectID:            row.ProjectID,
			ProjectName:          row.ProjectName,
			DeviceID:             row.DeviceID,
			DeviceName:           row.DeviceName,
			Label:                label,
			Description:          description,
			OCRExcerpts:          ocrExcerpts,
			SegmentationExcerpts: segmentationBySource[row.DocumentID],
			Citation: DocumentChatCitation{
				DocumentID:  row.DocumentID,
				ProjectID:   row.ProjectID,
				ProjectName: row.ProjectName,
				DeviceID:    row.DeviceID,
				DeviceName:  row.DeviceName,
				Label:       label,
				Excerpt:     citationExcerpt,
				MatchReason: "document context",
			},
		})
	}

	return documents, nil
}

func loadSegmentationContext(ctx context.Context, db *gorm.DB, rows []documentContextRow) ([]segmentationContextRow, error) {
	sourceDocumentIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		sourceDocumentIDs = append(sourceDocumentIDs, row.DocumentID)
	}

	var segmentationRows []segmentationContextRow
	err := db.WithContext(ctx).Raw(`
		SELECT
			ds.source_document_id,
			ds.id AS segmentation_id,
			COALESCE(ds.input ->> 'prompt', '') AS prompt,
			ds.created_at,
			TRIM(BOTH ' /' FROM COALESCE(m.provider, '') || ' / ' || COALESCE(m.name, '')) AS model_label,
			COALESCE(seg_description.text, '') AS description_text,
			COALESCE(seg_ocr.text, '') AS ocr_text,
			segmented_document.object_key
		FROM document_segmentations ds
		LEFT JOIN models m ON m.id = ds.model_id
		LEFT JOIN documents segmented_document ON segmented_document.id = ds.segmented_document_id
		LEFT JOIN LATERAL (
			SELECT dd.text
			FROM document_descriptions dd
			WHERE dd.document_id = ds.segmented_document_id
			ORDER BY dd.created_at DESC
			LIMIT 1
		) seg_description ON TRUE
		LEFT JOIN LATERAL (
			SELECT dor.text
			FROM document_ocr_results dor
			WHERE dor.document_id = ds.segmented_document_id
			ORDER BY dor.created_at DESC
			LIMIT 1
		) seg_ocr ON TRUE
		WHERE ds.source_document_id IN ?
		ORDER BY ds.created_at DESC
	`, sourceDocumentIDs).Scan(&segmentationRows).Error
	if err != nil {
		return nil, fmt.Errorf("read_document_context segmentation query failed: %w", err)
	}

	return segmentationRows, nil
}

func buildDocumentScopeFilters(alias string, scope ChatDocumentScope) (string, []any) {
	clauses := []string{fmt.Sprintf("%s.organization_id = ?", alias)}
	args := []any{scope.OrganizationID}

	if ids := dedupeNonEmpty(scope.ProjectIDs); len(ids) > 0 {
		clauses = append(clauses, fmt.Sprintf("%s.project_id IN ?", alias))
		args = append(args, ids)
	}
	if ids := dedupeNonEmpty(scope.DeviceIDs); len(ids) > 0 {
		clauses = append(clauses, fmt.Sprintf("%s.device_id IN ?", alias))
		args = append(args, ids)
	}
	if ids := dedupeNonEmpty(scope.DocumentIDs); len(ids) > 0 {
		clauses = append(clauses, fmt.Sprintf("%s.id IN ?", alias))
		args = append(args, ids)
	}

	return strings.Join(clauses, " AND "), args
}

func buildSearchSnippet(query string, description string, ocrText string, objectKey string, projectName string, deviceName *string) (string, string) {
	queryTerms := splitTerms(query)
	if snippet := excerptAroundTerms(description, queryTerms, 220); snippet != "" {
		return snippet, "description"
	}
	if snippet := excerptAroundTerms(ocrText, queryTerms, 220); snippet != "" {
		return snippet, "OCR text"
	}

	metadataFields := []string{projectName, objectKey}
	if deviceName != nil {
		metadataFields = append(metadataFields, *deviceName)
	}
	for _, field := range metadataFields {
		if snippet := excerptAroundTerms(field, queryTerms, 160); snippet != "" {
			return snippet, "metadata"
		}
	}

	return buildPreferredExcerpt(description, ocrText, objectKey), "document context"
}

func buildPreferredExcerpt(description string, ocrText string, fallback string) string {
	if clipped := clipText(description, 220); clipped != "" {
		return clipped
	}
	if clipped := clipText(ocrText, 220); clipped != "" {
		return clipped
	}
	return clipText(fallback, 220)
}

func buildOCRExcerpts(row documentContextRow) []DocumentOCRExcerpt {
	if strings.TrimSpace(row.OCRText) == "" {
		return []DocumentOCRExcerpt{}
	}

	modelLabel := "OCR result"
	if row.OCRModelLabel != nil && strings.TrimSpace(*row.OCRModelLabel) != "" {
		modelLabel = *row.OCRModelLabel
	}
	createdAt := row.CreatedAt.Format(time.RFC3339)
	if row.OCRCreatedAt != nil {
		createdAt = row.OCRCreatedAt.Format(time.RFC3339)
	}

	return []DocumentOCRExcerpt{
		{
			ModelLabel: modelLabel,
			Excerpt:    clipText(row.OCRText, 420),
			CreatedAt:  createdAt,
		},
	}
}

func buildSegmentationExcerpt(row segmentationContextRow) DocumentSegmentationExcerpt {
	excerpt := buildPreferredExcerpt(row.DescriptionText, row.OCRText, "")
	if excerpt == "" && row.ObjectKey != nil {
		excerpt = clipText(*row.ObjectKey, 220)
	}

	prompt := ""
	if row.Prompt != nil {
		prompt = strings.TrimSpace(*row.Prompt)
	}

	return DocumentSegmentationExcerpt{
		SegmentationID: row.SegmentationID,
		ModelLabel:     row.ModelLabel,
		Prompt:         prompt,
		Excerpt:        excerpt,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
	}
}

func buildDocumentLabel(objectKey string, documentID string) string {
	base := strings.TrimSpace(filepath.Base(objectKey))
	if base != "" && base != "." && base != "/" {
		return base
	}
	if len(documentID) >= 8 {
		return fmt.Sprintf("Document %s", documentID[:8])
	}
	return "Document"
}

func excerptAroundTerms(text string, terms []string, maxLen int) string {
	normalized := normalizeWhitespace(text)
	if normalized == "" {
		return ""
	}

	lower := strings.ToLower(normalized)
	firstIndex := -1
	for _, term := range terms {
		if term == "" {
			continue
		}
		index := strings.Index(lower, strings.ToLower(term))
		if index >= 0 && (firstIndex == -1 || index < firstIndex) {
			firstIndex = index
		}
	}
	if firstIndex == -1 {
		return ""
	}

	start := firstIndex - maxLen/3
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(normalized) {
		end = len(normalized)
	}
	snippet := strings.TrimSpace(normalized[start:end])
	if start > 0 {
		snippet = "..." + strings.TrimLeft(snippet, ".,;:!? ")
	}
	if end < len(normalized) {
		snippet = strings.TrimRight(snippet, ".,;:!? ") + "..."
	}
	return snippet
}

func clipText(text string, maxLen int) string {
	normalized := normalizeWhitespace(text)
	if normalized == "" {
		return ""
	}
	if len(normalized) <= maxLen {
		return normalized
	}
	return strings.TrimSpace(normalized[:maxLen-3]) + "..."
}

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func splitTerms(query string) []string {
	parts := strings.Fields(strings.ToLower(query))
	deduped := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		trimmed := strings.Trim(part, "\"'.,;:!?()[]{}")
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	sort.Strings(deduped)
	return deduped
}

func dedupeNonEmpty(values []string) []string {
	deduped := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func intersectDocumentIDs(scopeDocumentIDs []string, requestedDocumentIDs []string) []string {
	scopeIDs := dedupeNonEmpty(scopeDocumentIDs)
	requestedIDs := dedupeNonEmpty(requestedDocumentIDs)
	if len(scopeIDs) == 0 {
		return requestedIDs
	}

	allowed := map[string]struct{}{}
	for _, id := range scopeIDs {
		allowed[id] = struct{}{}
	}

	intersection := make([]string, 0, len(requestedIDs))
	for _, id := range requestedIDs {
		if _, ok := allowed[id]; ok {
			intersection = append(intersection, id)
		}
	}
	return intersection
}
