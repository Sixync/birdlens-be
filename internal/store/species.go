package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"log"
	"time"

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
	// Increase timeout for complex geospatial queries
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var ranges []RangeData

	log.Printf("[STORE] Executing scientific name search for: %s", scientificName)

	// Option 1: Exact match first (fastest)
	const exactQuery = `
        SELECT
            ST_AsGeoJSON(ST_SimplifyPreserveTopology(geom, 0.01)) as geojson
        FROM
            public.species_ranges
        WHERE
            sci_name = $1
    `

	err := s.db.SelectContext(ctx, &ranges, exactQuery, scientificName)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[STORE-ERROR] Error in exact match query: %v", err)
		return nil, err
	}

	// If exact match found, return it
	if len(ranges) > 0 {
		log.Printf("[STORE] Exact match found %d rows.", len(ranges))
		return ranges, nil
	}

	log.Printf("[STORE] No exact match found, trying case-insensitive search...")

	// Option 2: Case-insensitive exact match
	const iexactQuery = `
        SELECT
            ST_AsGeoJSON(ST_SimplifyPreserveTopology(geom, 0.01)) as geojson
        FROM
            public.species_ranges
        WHERE
            LOWER(sci_name) = LOWER($1)
    `

	err = s.db.SelectContext(ctx, &ranges, iexactQuery, scientificName)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[STORE-ERROR] Error in case-insensitive exact query: %v", err)
		return nil, err
	}

	// If case-insensitive match found, return it
	if len(ranges) > 0 {
		log.Printf("[STORE] Case-insensitive exact match found %d rows.", len(ranges))
		return ranges, nil
	}

	log.Printf("[STORE] No exact matches found, trying partial search...")

	// Option 3: Partial match (slower, but more flexible)
	const partialQuery = `
        SELECT
            ST_AsGeoJSON(ST_SimplifyPreserveTopology(geom, 0.01)) as geojson
        FROM
            public.species_ranges
        WHERE
            sci_name ILIKE $1
        LIMIT 100
    `

	searchPattern := "%" + scientificName + "%"
	err = s.db.SelectContext(ctx, &ranges, partialQuery, searchPattern)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[STORE] Partial search found 0 rows.")
			return []RangeData{}, nil
		}
		log.Printf("[STORE-ERROR] Error fetching species range by partial pattern '%s': %v", searchPattern, err)
		return nil, err
	}

	log.Printf("[STORE] Partial search found %d rows.", len(ranges))
	return ranges, nil
}