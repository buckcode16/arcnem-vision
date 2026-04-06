package tools

import (
	"fmt"
	"strings"

	dbmodels "github.com/arcnem-ai/arcnem-vision/models/db/gen/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func normalizeModelVersion(version string) string {
	return strings.TrimSpace(version)
}

func findModelByIdentity(db *gorm.DB, provider string, name string, version string) (*dbmodels.Model, error) {
	normalizedProvider := strings.TrimSpace(provider)
	normalizedName := strings.TrimSpace(name)
	normalizedVersion := normalizeModelVersion(version)

	var model dbmodels.Model
	if err := db.Where(
		"provider = ? AND name = ? AND version = ?",
		normalizedProvider,
		normalizedName,
		normalizedVersion,
	).First(&model).Error; err != nil {
		return nil, fmt.Errorf(
			"failed to find model %s/%s version=%q in db: %w",
			normalizedProvider,
			normalizedName,
			normalizedVersion,
			err,
		)
	}

	return &model, nil
}

func ensureModelByIdentity(
	db *gorm.DB,
	provider string,
	name string,
	version string,
	modelType *string,
	embeddingDim *int32,
) (*dbmodels.Model, error) {
	normalizedProvider := strings.TrimSpace(provider)
	normalizedName := strings.TrimSpace(name)
	normalizedVersion := normalizeModelVersion(version)

	record := dbmodels.Model{
		Provider:     normalizedProvider,
		Name:         normalizedName,
		Version:      normalizedVersion,
		Type:         modelType,
		EmbeddingDim: embeddingDim,
		Config:       "{}",
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "name"},
			{Name: "version"},
		},
		DoNothing: true,
	}).Create(&record).Error; err != nil {
		return nil, fmt.Errorf(
			"failed to ensure model %s/%s version=%q: %w",
			normalizedProvider,
			normalizedName,
			normalizedVersion,
			err,
		)
	}

	return findModelByIdentity(db, normalizedProvider, normalizedName, normalizedVersion)
}
