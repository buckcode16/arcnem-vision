package tools

import (
	"fmt"
	"strings"

	dbmodels "github.com/arcnem-ai/arcnem-vision/models/db/gen/models"
	"gorm.io/gorm"
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
