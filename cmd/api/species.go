// birdlens-be/cmd/api/species.go
package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/sixync/birdlens-be/internal/response"
)

func (app *application) getSpeciesRangeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("--- /species/range HANDLER ---")

	// Extract scientific name from query parameters
	scientificName := r.URL.Query().Get("scientific_name")
	
	// Validate that scientific name is provided
	if scientificName == "" {
		log.Println("[HANDLER-ERROR] Missing required parameter: scientific_name")
		response.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "Missing required parameter: scientific_name",
		}, true, "Bad Request")
		return
	}

	// Trim whitespace and validate the parameter
	scientificName = strings.TrimSpace(scientificName)
	if scientificName == "" {
		log.Println("[HANDLER-ERROR] Empty scientific_name parameter")
		response.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "scientific_name parameter cannot be empty",
		}, true, "Bad Request")
		return
	}

	log.Printf("[HANDLER] Using scientific name from query parameter: '%s'", scientificName)

	// Call the store method with the provided scientific name
	speciesRanges, err := app.store.Species.GetRangeByScientificName(r.Context(), scientificName)
	if err != nil {
		log.Printf("[HANDLER-ERROR] Store method returned an error: %v", err)
		app.serverError(w, r, err)
		return
	}
	
	log.Printf("[HANDLER] Database search completed, found %d polygons.", len(speciesRanges))

	if len(speciesRanges) == 0 {
		log.Printf("[HANDLER] No data found for scientific name: '%s'", scientificName)
		response.JSON(w, http.StatusNotFound, map[string]string{
			"message": "No range data found for the specified species",
		}, true, "Not Found")
		return
	}

	log.Printf("[HANDLER] Success: Returning %d polygons for scientific name: '%s'", len(speciesRanges), scientificName)
	response.JSON(w, http.StatusOK, speciesRanges, false, "Species range data retrieved successfully")
}