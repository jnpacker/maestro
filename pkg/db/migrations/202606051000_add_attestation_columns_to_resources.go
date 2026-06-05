package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func addAttestationColumnsToResources() *gormigrate.Migration {
	type Resource struct {
		Attestation     datatypes.JSONMap
		AttestationMode string
	}

	return &gormigrate.Migration{
		ID: "202606051000",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Resource{})
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropColumn(&Resource{}, "attestation_mode"); err != nil {
				return err
			}
			return tx.Migrator().DropColumn(&Resource{}, "attestation")
		},
	}
}
