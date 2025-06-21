// birdlens-be/internal/store/species.go
package store

import (
	"context"
	"database/sql"
	"log"

	"github.com/jmoiron/sqlx"
)

// RangeData struct remains the same. It correctly handles NULL values from the database.
type RangeData struct {
	GeoJSON sql.NullString `db:"geojson" json:"geo_json"`
}

type SpeciesStore struct {
	db *sqlx.DB
}

// Logic: The function is renamed to accurately reflect that it queries by scientific name.
// This is much clearer than querying by 'sisrecid' which we aren't using directly from the API.
func (s *SpeciesStore) GetRangeByScientificName(ctx context.Context, scientificName string) ([]RangeData, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var ranges []RangeData

	// Logic: The WHERE clause is updated to query the 'scientificname' column.
	// This now perfectly matches what our handler needs to do.
	const query = `
        SELECT
            ST_AsGeoJSON(ST_SimplifyPreserveTopology(geom, 0.005)) as geojson
        FROM
            public.species_ranges
        WHERE
            scientificname = $1
    `

	err := s.db.SelectContext(ctx, &ranges, query, scientificName)
	if err != nil {
		if err == sql.ErrNoRows {
			return []RangeData{}, nil
		}
		log.Printf("Error fetching species range from PostGIS for scientific name %s: %v", scientificName, err)
		return nil, err
	}

	var validRanges []RangeData
	for _, r := range ranges {
		if r.GeoJSON.Valid && r.GeoJSON.String != "" {
			validRanges = append(validRanges, r)
		}
	}

	return validRanges, nil
}