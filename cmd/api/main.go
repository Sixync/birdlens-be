// birdlens-be/cmd/api/main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log" // Standard log for initial messages
	"os"
	"runtime/debug"
	"sync"

	"log/slog" // Structured logging

	firebase "firebase.google.com/go"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/sixync/birdlens-be/auth"
	"github.com/sixync/birdlens-be/internal/database"
	"github.com/sixync/birdlens-be/internal/env"
	"github.com/sixync/birdlens-be/internal/jwt"
	"github.com/sixync/birdlens-be/internal/smtp"
	"github.com/sixync/birdlens-be/internal/store"
	mediamanager "github.com/sixync/birdlens-be/internal/store/media_manager"
	"github.com/sixync/birdlens-be/internal/version"
	"github.com/stripe/stripe-go/v82"
	"google.golang.org/api/option"
)

type config struct {
	httpPort    int
	baseURL     string
	frontEndUrl string
	basicAuth   struct {
		username       string
		hashedPassword string
	}
	db struct {
		dbConn string
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		from     string
	}
	jwt struct {
		secretKey                  string
		accessTokenExpDurationMin  int
		refreshTokenExpDurationDay int
	}
	emailVerificationExpiresInHours int
	forgotPasswordExpiresInHours    int
	stripe                          struct {
		secretKey      string
		publishableKey string
		webhookSecret  string
	}
	// Logic: Add a new struct to hold eBird-related configuration.
	eBird struct {
		apiKey string
	}
}

type EmailJob struct {
	Recipient string
	Data      any
	Patterns  []string
}
type application struct {
	config      config
	store       *store.Storage
	logger      *slog.Logger // Changed to slog.Logger
	mailer      *smtp.Mailer
	wg          sync.WaitGroup
	authService *auth.AuthService
	tokenMaker  *jwt.JWTMaker
	mediaClient mediamanager.MediaClient
}

var JobQueue = make(chan EmailJob, 100)

func main() {
	// Use standard log for very early messages before slog is configured
	log.Println("Application main() started.")

	// Initialize structured logger
	slogHandler := tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug, AddSource: true})
	logger := slog.New(slogHandler)
	slog.SetDefault(logger) // Set default for any library using slog's default logger

	log.Println("Slog logger configured.")                   // Standard log confirmation
	slog.Info("Slog logger initialized and set as default.") // Slog confirmation

	err := run(logger) // Pass the configured slog logger
	if err != nil {
		// Using standard log here to be absolutely sure it prints if slog itself had an issue
		log.Printf("CRITICAL ERROR from run(): %v\n", err)
		currentTrace := string(debug.Stack())
		log.Printf("Trace: %s\n", currentTrace)
		// Also log with slog if it's available
		slog.Error("CRITICAL ERROR from run()", "error", err.Error(), "trace", currentTrace)
		os.Exit(1)
	}
	log.Println("Application main() finished (Note: this line should ideally not be reached if server runs indefinitely).")
}

func run(logger *slog.Logger) error {
	log.Println("run() function entered.")         // Standard log
	slog.Info("run() function entered with slog.") // Slog confirmation

	var cfg config

	slog.Info("Loading environment variables...")

	// Load environment files. Errors will be logged by godotenv if files are not found, which is often okay.
	// Order might matter if keys are duplicated; last one loaded wins.
	errMail := godotenv.Load("/env/mail.env")
	if errMail != nil {
		slog.Warn("Could not load /env/mail.env", "error", errMail)
	}
	errDotEnv := godotenv.Load("/env/.env")
	if errDotEnv != nil {
		slog.Warn("Could not load /env/.env", "error", errDotEnv)
	}
	errStripe := godotenv.Load("/env/stripe.env")
	if errStripe != nil {
		slog.Warn("Could not load /env/stripe.env", "error", errStripe)
	}
	errCloudinary := godotenv.Load("/env/cloudinary.env")
	if errCloudinary != nil {
		slog.Warn("Could not load /env/cloudinary.env", "error", errCloudinary)
	}
	// Logic: Load the new ebird.env file.
	errEbird := godotenv.Load("/env/ebird.env")
	if errEbird != nil {
		slog.Warn("Could not load /env/ebird.env", "error", errEbird)
	}

	cfg.httpPort = env.GetInt("HTTP_PORT", 6969)
	cfg.baseURL = env.GetString("BASE_URL", "http://localhost:8090") // Nginx listens on 80, Go on 6969. BaseURL is external.
	cfg.basicAuth.username = env.GetString("BASIC_AUTH_USERNAME", "admin")
	cfg.basicAuth.hashedPassword = env.GetString("BASIC_AUTH_HASHED_PASSWORD", "$2a$10$jRb2qniNcoCyQM23T59RfeEQUbgdAXfR6S0scynmKfJa5Gj3arGJa") // Default "pa55word"
	cfg.db.dbConn = env.GetString("DB_ADDR", "postgres://admin:password@birdlens-db:5432/birdlens?sslmode=disable")

	slog.Info("DB_ADDR for database connection", "value", cfg.db.dbConn)

	cfg.smtp.host = env.GetString("SMTP_HOST", "mailpit")
	cfg.smtp.port = env.GetInt("SMTP_PORT", 1025)
	cfg.smtp.username = env.GetString("SMTP_USERNAME", "")
	cfg.smtp.password = env.GetString("SMTP_PASSWORD", "")
	cfg.smtp.from = env.GetString("SMTP_FROM", "test@example.com")
	slog.Info("SMTP Config", "host", cfg.smtp.host, "port", cfg.smtp.port, "from", cfg.smtp.from)

	cfg.jwt.secretKey = env.GetString("JWT_SECRET_KEY", "THISISASECRETKEYHALLELUJAHBABY123123123123123123123")
	cfg.jwt.accessTokenExpDurationMin = env.GetInt("ACCESS_TOKEN_EXP_MIN", 15)
	cfg.jwt.refreshTokenExpDurationDay = env.GetInt("REFRESH_TOKEN_EXP_DAY", 1)
	cfg.frontEndUrl = env.GetString("FRONTEND_URL", "http://localhost") // Assuming frontend is accessed via localhost (or actual domain)
	cfg.emailVerificationExpiresInHours = env.GetInt("EMAIL_VERIFICATION_EXPIRES_IN_HOURS", 7)
	cfg.forgotPasswordExpiresInHours = env.GetInt("FORGOT_PASSWORD_EXPIRES_IN_HOURS", 1)

	cfg.stripe.secretKey = env.GetString("STRIPE_SECRET_KEY", "")
	cfg.stripe.publishableKey = env.GetString("STRIPE_PUBLISHABLE_KEY", "")
	cfg.stripe.webhookSecret = env.GetString("STRIPE_WEBHOOK_SECRET", "")

	// Logic: Read the EBIRD_API_KEY from the environment into the config struct.
	cfg.eBird.apiKey = env.GetString("EBIRD_API_KEY", "")

	// Log the presence of each key
	slog.Info("Stripe keys loaded",
		"secret_key_present", cfg.stripe.secretKey != "",
		"publishable_key_present", cfg.stripe.publishableKey != "",
		"webhook_secret_present", cfg.stripe.webhookSecret != "")

	// Logic: Add a log to confirm the eBird key was loaded.
	slog.Info("eBird API key loaded", "present", cfg.eBird.apiKey != "")

	if cfg.stripe.secretKey == "" {
		slog.Warn("STRIPE_SECRET_KEY is not set. Payments will fail.")
		// Depending on strictness, you might want to return an error here:
		// return errors.New("STRIPE_SECRET_KEY is not set")
	}
	stripe.Key = cfg.stripe.secretKey
	slog.Info("Stripe SDK key initialized.")

	// Logic: Add a check for the eBird key.
	if cfg.eBird.apiKey == "" {
		slog.Warn("EBIRD_API_KEY is not set. Premium features for hotspot analysis will fail.")
	}

	slog.Info("Frontend URL for emails/redirects", "url", cfg.frontEndUrl)
	slog.Info("Base URL for API (self-awareness)", "url", cfg.baseURL)
	slog.Info("Environment variables loading complete.")

	showVersion := flag.Bool("version", false, "display version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("version: %s\n", version.Get())
		return nil
	}

	slog.Info("Attempting database connection...")
	db, err := database.New(cfg.db.dbConn)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err, "connection_string", cfg.db.dbConn)
		return fmt.Errorf("database.New failed: %w", err)
	}
	defer db.Close()
	slog.Info("Database connection successful.")

	store := store.NewStore(db)
	slog.Info("Data store initialized.")

	slog.Info("Initializing mailer...")
	mailer, err := smtp.NewMailer(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.from)
	if err != nil {
		slog.Error("Failed to initialize mailer", "error", err)
		return fmt.Errorf("smtp.NewMailer failed: %w", err)
	}
	slog.Info("Mailer initialized.")

	startWorkerPool(mailer, 5)

	firebaseCredsPath := env.GetString("PATH_TO_FIREBASE_CREDS", "")
	slog.Info("Firebase credentials path from env", "path", firebaseCredsPath)
	if firebaseCredsPath == "" {
		errMsg := "PATH_TO_FIREBASE_CREDS environment variable is not set. Firebase Admin SDK cannot be initialized."
		slog.Error(errMsg)
		return errors.New(errMsg)
	}

	slog.Info("Initializing Firebase Admin SDK...")
	opt := option.WithCredentialsFile(firebaseCredsPath)
	fbApp, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		slog.Error("Error initializing Firebase app", "error", err, "credentials_path", firebaseCredsPath)
		return fmt.Errorf("firebase.NewApp failed: %w. Check credentials file path and content", err)
	}
	slog.Info("Firebase app initialized successfully.")

	firebaseAuthClient, err := fbApp.Auth(context.Background())
	if err != nil {
		slog.Error("Error initializing Firebase Auth client", "error", err)
		return fmt.Errorf("fbApp.Auth failed: %w", err)
	}
	slog.Info("Firebase Auth client initialized successfully.")

	authService := auth.NewAuthService(store, firebaseAuthClient)
	slog.Info("Auth service initialized.")

	tokenMaker := jwt.NewJWTMaker(cfg.jwt.secretKey)
	slog.Info("JWT Token maker initialized.")

	slog.Info("Initializing Cloudinary client...")
	cloudinaryAPIKey := env.GetString("CLOUDINARY_API_KEY", "")
	cloudinaryAPISecret := env.GetString("CLOUDINARY_API_SECRET", "")
	cloudinaryCloudName := env.GetString("CLOUDINARY_CLOUD_NAME", "")

	if cloudinaryAPIKey == "" || cloudinaryAPISecret == "" || cloudinaryCloudName == "" {
		errMsg := "Cloudinary credentials (API_KEY, API_SECRET, CLOUD_NAME) are not fully set in environment variables. Media client cannot be initialized."
		slog.Error(errMsg)
	}

	cldClient := mediamanager.NewCloudinaryClient()
	if cldClient == nil {
		slog.Error("Failed to create media client (CloudinaryClient was nil after initialization attempt).")
		return errors.New("failed to create media client")
	}
	slog.Info("Cloudinary client initialized.")

	app := &application{
		config:      cfg,
		store:       store,
		logger:      logger,
		mailer:      mailer,
		tokenMaker:  tokenMaker,
		authService: authService,
		mediaClient: cldClient,
	}
	slog.Info("Application struct fully initialized.")

	slog.Info("Starting HTTP server...")
	return app.serveHTTP()
}

func worker(mailer *smtp.Mailer, id int) {
	for job := range JobQueue {
		slog.Info("Email worker processing job", "worker_id", id, "recipient", job.Recipient)
		err := mailer.Send(job.Recipient, job.Data, job.Patterns...)
		if err != nil {
			slog.Error("Email worker failed to send email", "worker_id", id, "recipient", job.Recipient, "error", err)
		} else {
			slog.Info("Email worker successfully sent email", "worker_id", id, "recipient", job.Recipient)
		}
	}
}

func startWorkerPool(mailer *smtp.Mailer, numWorkers int) {
	for w := 1; w <= numWorkers; w++ {
		go worker(mailer, w)
	}
	slog.Info("Started email worker pool", "num_workers", numWorkers)
}