// birdlens-be/cmd/api/hotspots.go
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/sixync/birdlens-be/internal/response"
)

// Represents a single observation from the eBird API
type EbirdHistoricObservation struct {
	SpeciesCode string `json:"speciesCode"`
	ObsDt       string `json:"obsDt"`
	HowMany     int    `json:"howMany"`
}

// The final analysis data structure we will send to the client
type VisitingTimesAnalysis struct {
	MonthlyActivity []MonthlyStat `json:"monthly_activity"`
	HourlyActivity  []HourlyStat  `json:"hourly_activity"`
}

type MonthlyStat struct {
	Month             string  `json:"month"`
	RelativeFrequency float64 `json:"relative_frequency"`
}

type HourlyStat struct {
	Hour              int     `json:"hour"` // 0-23
	RelativeFrequency float64 `json:"relative_frequency"`
}

// Logic: Create structs for the worker pool pattern to make the code cleaner.
type fetchJob struct {
	daysAgo int
}

type fetchResult struct {
	observations []EbirdHistoricObservation
	err          error
}

func (app *application) getHotspotVisitingTimesHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Check for ExBird subscription (This part was correct)
	user := app.getUserFromFirebaseClaimsCtx(r)
	if user == nil {
		app.unauthorized(w, r)
		return
	}

	subscription, err := app.store.Subscriptions.GetUserSubscriptionByEmail(r.Context(), user.Email)
	if err != nil || subscription == nil || subscription.Name != "ExBird" {
		app.errorMessage(w, r, http.StatusForbidden, "This feature is exclusive to ExBird subscribers.", nil)
		return
	}

	// 2. Get parameters from request (This part was correct)
	locId := r.PathValue("locId")
	speciesCode := r.URL.Query().Get("speciesCode")

	slog.Info("Starting visiting times analysis", "locId", locId, "speciesCode", speciesCode, "user", user.Email)

	// 3. FIX: Fetch and Analyze Data using a Worker Pool to avoid overwhelming the system.
	const numJobs = 365
	const numWorkers = 20 // A reasonable number of concurrent workers

	jobs := make(chan fetchJob, numJobs)
	results := make(chan fetchResult, numJobs)
	//var wg sync.WaitGroup

	// Start the workers
	for w := 1; w <= numWorkers; w++ {
		go func(workerID int) {
			for job := range jobs {
				date := time.Now().AddDate(0, 0, -job.daysAgo)
				url := fmt.Sprintf("https://api.ebird.org/v2/data/obs/%s/historic/%d/%d/%d", locId, date.Year(), date.Month(), date.Day())

				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Set("X-eBirdApiToken", app.config.eBird.apiKey)

				client := &http.Client{Timeout: 15 * time.Second} // Increased timeout slightly
				resp, err := client.Do(req)

				if err != nil {
					results <- fetchResult{nil, fmt.Errorf("worker %d failed for date %s: %w", workerID, date.Format("2006-01-02"), err)}
					continue
				}

				if resp.StatusCode != http.StatusOK {
					// Don't treat "not found" as a fatal error for a single day.
					if resp.StatusCode == http.StatusNotFound {
						resp.Body.Close()
						results <- fetchResult{nil, nil} // Send a success with no data
						continue
					}
					resp.Body.Close()
					results <- fetchResult{nil, fmt.Errorf("worker %d: eBird API returned status %d for date %s", workerID, resp.StatusCode, date.Format("2006-01-02"))}
					continue
				}

				var observations []EbirdHistoricObservation
				if err := json.NewDecoder(resp.Body).Decode(&observations); err != nil {
					resp.Body.Close()
					results <- fetchResult{nil, fmt.Errorf("worker %d failed to decode JSON for date %s: %w", workerID, date.Format("2006-01-02"), err)}
					continue
				}

				resp.Body.Close()
				results <- fetchResult{observations, nil}
			}
		}(w)
	}

	// Send jobs to the workers
	for i := 0; i < numJobs; i++ {
		jobs <- fetchJob{daysAgo: i}
	}
	close(jobs) // Close the jobs channel to signal workers to stop once they are done.

	// Collect results
	allObservations := []EbirdHistoricObservation{}
	for i := 0; i < numJobs; i++ {
		result := <-results
		if result.err != nil {
			// Fail on the first error to avoid long waits for potentially failing requests.
			app.serverError(w, r, fmt.Errorf("failed during data fetch from eBird: %w", result.err))
			return
		}
		if result.observations != nil {
			allObservations = append(allObservations, result.observations...)
		}
	}

	// Filter observations by species code if provided
	if speciesCode != "" {
		filteredObs := []EbirdHistoricObservation{}
		for _, obs := range allObservations {
			if obs.SpeciesCode == speciesCode {
				filteredObs = append(filteredObs, obs)
			}
		}
		allObservations = filteredObs
	}

	if len(allObservations) == 0 {
		response.JSON(w, http.StatusNotFound, nil, true, "Not enough observation data to perform analysis for this location/species.")
		return
	}

	// 4. Aggregate Data (This part was correct)
	monthCounts := make(map[time.Month]int)
	hourCounts := make(map[int]int)

	for _, obs := range allObservations {
		obsTime, err := time.Parse("2006-01-02 15:04", obs.ObsDt)
		if err != nil {
			continue // Skip records with invalid date format
		}
		monthCounts[obsTime.Month()]++
		hourCounts[obsTime.Hour()]++
	}

	// 5. Normalize Data and respond (This part was correct)
	analysis := VisitingTimesAnalysis{
		MonthlyActivity: normalizeMonthCounts(monthCounts),
		HourlyActivity:  normalizeHourCounts(hourCounts),
	}

	response.JSON(w, http.StatusOK, analysis, false, "Analysis successful")
}

func normalizeMonthCounts(counts map[time.Month]int) []MonthlyStat {
	var maxCount float64 = 0
	for _, count := range counts {
		if float64(count) > maxCount {
			maxCount = float64(count)
		}
	}

	if maxCount == 0 {
		return []MonthlyStat{}
	}

	stats := make([]MonthlyStat, 0, len(counts))
	for month, count := range counts {
		stats = append(stats, MonthlyStat{
			Month:             month.String(),
			RelativeFrequency: float64(count) / maxCount,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		mi, _ := time.Parse("January", stats[i].Month)
		mj, _ := time.Parse("January", stats[j].Month)
		return mi.Month() < mj.Month()
	})
	return stats
}

func normalizeHourCounts(counts map[int]int) []HourlyStat {
	var maxCount float64 = 0
	for _, count := range counts {
		if float64(count) > maxCount {
			maxCount = float64(count)
		}
	}

	if maxCount == 0 {
		return []HourlyStat{}
	}

	stats := make([]HourlyStat, 0, len(counts))
	for hour, count := range counts {
		stats = append(stats, HourlyStat{
			Hour:              hour,
			RelativeFrequency: float64(count) / maxCount,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Hour < stats[j].Hour
	})
	return stats
}