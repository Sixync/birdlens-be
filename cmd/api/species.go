// birdlens-be/cmd/api/species.go
package main

import (
	"log"
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
)

func (app *application) getSpeciesRangeHandler(w http.ResponseWriter, r *http.Request) {
	speciesCode := r.PathValue("species_id")
	if speciesCode == "" {
		app.badRequest(w, r, http.ErrNoCookie)
		return
	}

	// This lookup map is a temporary bridge between eBird's speciesCode and BirdLife's scientificname.
	// For a full solution, you would build a more robust lookup method.
	sciNameLookup := map[string]string{
		"aspswi1": "Cypsiurus balasiensis", // Asian Palm Swift
		// You can add more species here for testing
	}

	scientificName, ok := sciNameLookup[speciesCode]
	if !ok {
		log.Printf("No scientific name mapping found for species code: %s", speciesCode)
		app.notFound(w, r)
		return
	}

	log.Printf("Fetching range for species: %s (Scientific Name: %s)", speciesCode, scientificName)

	// Logic: This now correctly calls the clean, specific function in the store.
	// The handler's responsibility is now much clearer.
	speciesRanges, err := app.store.Species.GetRangeByScientificName(r.Context(), scientificName)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if len(speciesRanges) == 0 {
		app.notFound(w, r)
		return
	}

	response.JSON(w, http.StatusOK, speciesRanges, false, "Species range data retrieved successfully")
}