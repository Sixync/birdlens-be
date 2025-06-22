package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
)

// JSONNullString is a custom type that wraps sql.NullString
// and implements both json.Marshaler and sql.Scanner interfaces
type JSONNullString struct {
	String string
	Valid  bool // Valid is true if String is not NULL
}

// MarshalJSON implements the json.Marshaler interface
func (v JSONNullString) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.String)
	}
	return json.Marshal(nil)
}

// Scan implements the sql.Scanner interface
func (v *JSONNullString) Scan(value interface{}) error {
	if value == nil {
		v.String, v.Valid = "", false
		return nil
	}
	
	switch s := value.(type) {
	case string:
		v.String, v.Valid = s, true
	case []byte:
		v.String, v.Valid = string(s), true
	default:
		// Try to convert to string using sql.NullString
		var ns sql.NullString
		err := ns.Scan(value)
		if err != nil {
			return err
		}
		v.String, v.Valid = ns.String, ns.Valid
	}
	return nil
}

// Value implements the driver.Valuer interface
func (v JSONNullString) Value() (driver.Value, error) {
	if !v.Valid {
		return nil, nil
	}
	return v.String, nil
}

// RangeData struct using the custom JSONNullString type
type RangeData struct {
	GeoJSON JSONNullString `db:"geojson" json:"geo_json"`
}

type SpeciesStore struct {
	db *sqlx.DB
}

// GetRangeByScientificName queries the species range by scientific name
func (s *SpeciesStore) GetRangeByScientificName(ctx context.Context, scientificName string) ([]RangeData, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var ranges []RangeData

	searchPattern := "%" + scientificName + "%"
	log.Printf("[STORE] Executing scientific name search with pattern: %s", searchPattern)

	const query = `
        SELECT
            ST_AsGeoJSON(ST_SimplifyPreserveTopology(geom, 0.005)) as geojson
        FROM
            public.species_ranges
        WHERE
            TRIM(sci_name) ILIKE $1
    `

	err := s.db.SelectContext(ctx, &ranges, query, searchPattern)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[STORE] Scientific name search found 0 rows.")
			return []RangeData{}, nil
		}
		log.Printf("[STORE-ERROR] Error fetching species range by scientific name pattern '%s': %v", searchPattern, err)
		return nil, err
	}

	log.Printf("[STORE] Scientific name search found %d rows.", len(ranges))
	return ranges, nil
}