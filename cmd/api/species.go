// birdlens-be/cmd/api/species.go
package main

import (
	"log"
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
	
)

func (app *application) getSpeciesRangeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("--- /species/range HANDLER (v7 - Final Hardcoded Test) ---")

	// Logic: For this test, we are hardcoding the scientific name for "Crested Duck".
	// The check for the query parameter has been removed, so the handler will proceed
	// even if no parameters are sent from the client (like in your Postman test).
	scientificNameForTest := "Lophonetta specularioides"
	
	log.Printf("[HANDLER-TEST] Using hardcoded scientific name: '%s'", scientificNameForTest)

	// Call the store method with our hardcoded name.
	speciesRanges, err := app.store.Species.GetRangeByScientificName(r.Context(), scientificNameForTest)
	if err != nil {
		log.Printf("[HANDLER-ERROR-TEST] Store method returned an error: %v", err)
		app.serverError(w, r, err)
		return
	}
	log.Printf("[HANDLER-TEST] Database search completed, found %d polygons.", len(speciesRanges))

	if len(speciesRanges) == 0 {
		log.Printf("[HANDLER-TEST] Final result: Hardcoded query found no data. Returning 404.")
		app.notFound(w, r)
		return
	}

	log.Printf("[HANDLER-TEST] Final result: Success. Returning %d polygons from hardcoded query.", len(speciesRanges))
	response.JSON(w, http.StatusOK, speciesRanges, false, "Species range data (hardcoded test) retrieved successfully")
}