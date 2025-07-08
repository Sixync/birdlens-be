// path: birdlens-be/cmd/api/routes.go
// (complete file content here - full imports, package names, all code)
package main

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import the cors package
)

func (app *application) routes() http.Handler {
	mux := chi.NewRouter()

	mux.NotFound(app.notFound)
	mux.MethodNotAllowed(app.methodNotAllowed)

	mux.Use(app.logAccess)
	mux.Use(app.recoverPanic)

	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://birdlens.netlify.app", "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	mux.Route("/posts", func(r chi.Router) {
		r.With(app.authMiddleware).With(app.paginate).Get("/", app.getPostsHandler)
		r.With(app.authMiddleware).Post("/", app.createPostHandler)
		r.With(app.authMiddleware).With(app.getPostMiddleware).With(app.paginate).Get("/{post_id}/comments", app.getPostCommentsHandler)
		r.With(app.authMiddleware).With(app.getPostMiddleware).Post("/{post_id}/reactions", app.addUserReactionHandler)
		r.With(app.authMiddleware).With(app.getPostMiddleware).Post("/{post_id}/comments", app.createCommentHandler)
	})

	mux.Route("/comments", func(r chi.Router) {
		r.With(app.authMiddleware).With(app.paginate).Get("/", app.getPostsHandler)
		r.With(app.authMiddleware).Post("/", app.createPostHandler)
		r.With(app.authMiddleware).With(app.getPostMiddleware).With(app.paginate).Get("/{post_id}/comments", app.getPostCommentsHandler)
		r.With(app.authMiddleware).With(app.getPostMiddleware).Post("/{post_id}/reactions", app.addUserReactionHandler)
		r.With(app.authMiddleware).With(app.getPostMiddleware).Post("/{post_id}/comments", app.createCommentHandler)
	})

	mux.Route("/users", func(r chi.Router) {
		r.With(app.paginate).With(app.getUserMiddleware).Get("/{user_id}/followers", app.getUserFollowersHandler)
		// Add the new life-list route
		r.With(app.authMiddleware).With(app.getUserMiddleware).Get("/{user_id}/life-list", app.getUserLifeListHandler)
		r.Post("/", app.createUserHandler)
		r.With(app.authMiddleware).Get("/me", app.getCurrentUserProfileHandler)
	})

	mux.Route("/auth", func(r chi.Router) {
		r.Post("/login", app.loginHandler)
		r.Post("/register", app.registerHandler)
		r.Post("/refresh_token", app.refreshTokenHandler)
		r.Get("/verify-email", app.verifyEmailHandler)
		r.Post("/forgot-password", app.forgotPasswordHandler)
		r.Post("/reset-password", app.resetPasswordHandler)
	})

	mux.Route("/tours", func(r chi.Router) {
		r.With(app.paginate).Get("/", app.getToursHandler)
		r.With(app.getTourMiddleware).Get("/{tour_id}", app.getTourHandler)
		r.Post("/", app.createTourHandler)
		r.With(app.getTourMiddleware).Put("/{tour_id}/images", app.addTourImagesHandler)
		r.With(app.getTourMiddleware).Put("/{tour_id}/thumbnail", app.addTourThumbnailHandler)
	})

	mux.Route("/subscriptions", func(r chi.Router) {
		r.Get("/", app.getSubscriptionsHandler)
		r.Post("/", app.createSubscriptionHandler)
	})

	mux.Route("/events", func(r chi.Router) {
		r.With(app.paginate).Get("/", app.getEventsHandler)
		r.Post("/", app.createEventHandler)
		r.With(app.getEventMiddleware).Get("/{event_id}", app.getEventHandler)
		r.With(app.getEventMiddleware).Delete("/{event_id}", app.deleteEventHandler)
	})

	mux.Route("/hotspots", func(r chi.Router) {
		r.With(app.authMiddleware).Get("/{locId}/visiting-times", app.getHotspotVisitingTimesHandler)
	})

	mux.Route("/species", func(r chi.Router) {
		r.With(app.authMiddleware).Get("/range", app.getSpeciesRangeHandler)
	})

	mux.Route("/ai", func(r chi.Router) {
		r.Use(app.authMiddleware)
		r.Post("/identify-bird", app.identifyBirdHandler)
		r.Post("/ask-question", app.askAiQuestionHandler)
	})

	// --- Payment Routes ---
	// All routes that require a logged-in user are protected by app.authMiddleware.
	// mux.With(app.authMiddleware).Post("/create-payment-intent", app.handleCreatePaymentIntent)
	mux.With(app.authMiddleware).Post("/payos/create-payment-link", app.createPayOSPaymentLinkHandler)

	// Webhook endpoints must be public and MUST NOT have the authMiddleware.
	// They use other forms of security (e.g., signature verification).
	// mux.Post("/stripe-webhooks", app.handleStripeWebhook)
	mux.Post("/payos-webhook", app.handlePayOSWebhook)
	mux.Post("/webhooks/github", app.handleGitHubWebhook)
	mux.Post("/services/send-weekly-newsletter", app.handleSendNewsletter)

	return mux
}

func (app *application) GetQueryInt(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	result, err := strconv.Atoi(value)
	if err != nil {
		return -1, err
	}

	return result, nil
}