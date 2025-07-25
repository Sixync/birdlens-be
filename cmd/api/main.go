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
	// Logic: The entire stripe struct is removed from the config.
	eBird struct {
		apiKey string
	}
	gemini struct {
		apiKey string
	}
	payos struct {
		clientID    string
		apiKey      string
		checksumKey string
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
	log.Println("run() function entered.")
	slog.Info("run() function entered with slog.")

	var cfg config

	slog.Info("Loading environment variables...")

	errMail := godotenv.Load("/env/mail.env")
	if errMail != nil {
		slog.Warn("Could not load /env/mail.env", "error", errMail)
	}
	errDotEnv := godotenv.Load("/env/.env")
	if errDotEnv != nil {
		slog.Warn("Could not load /env/.env", "error", errDotEnv)
	}
	// Logic: The loading of stripe.env is removed.
	errCloudinary := godotenv.Load("/env/cloudinary.env")
	if errCloudinary != nil {
		slog.Warn("Could not load /env/cloudinary.env", "error", errCloudinary)
	}
	errEbird := godotenv.Load("/env/ebird.env")
	if errEbird != nil {
		slog.Warn("Could not load /env/ebird.env", "error", errEbird)
	}
	errGemini := godotenv.Load("/env/gemini.env")
	if errGemini != nil {
		slog.Warn("Could not load /env/gemini.env", "error", errGemini)
	}
	errPayOS := godotenv.Load("/env/payos.env")
	if errPayOS != nil {
		slog.Warn("Could not load /env/payos.env", "error", errPayOS)
	}

	cfg.httpPort = env.GetInt("HTTP_PORT", 6969)
	cfg.baseURL = env.GetString("BASE_URL", "http://localhost:8090")
	cfg.basicAuth.username = env.GetString("BASIC_AUTH_USERNAME", "admin")
	cfg.basicAuth.hashedPassword = env.GetString("BASIC_AUTH_HASHED_PASSWORD", "$2a$10$jRb2qniNcoCyQM23T59RfeEQUbgdAXfR6S0scynmKfJa5Gj3arGJa")
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
	cfg.frontEndUrl = env.GetString("FRONTEND_URL", "http://localhost")
	cfg.emailVerificationExpiresInHours = env.GetInt("EMAIL_VERIFICATION_EXPIRES_IN_HOURS", 7)
	cfg.forgotPasswordExpiresInHours = env.GetInt("FORGOT_PASSWORD_EXPIRES_IN_HOURS", 1)

	// Logic: The reading of Stripe keys is removed.
	cfg.eBird.apiKey = env.GetString("EBIRD_API_KEY", "")
	cfg.gemini.apiKey = env.GetString("GEMINI_API_KEY", "")
	cfg.payos.clientID = env.GetString("PAYOS_CLIENT_ID", "")
	cfg.payos.apiKey = env.GetString("PAYOS_API_KEY", "")
	cfg.payos.checksumKey = env.GetString("PAYOS_CHECKSUM_KEY", "")

	slog.Info("eBird API key loaded", "present", cfg.eBird.apiKey != "")
	slog.Info("Gemini API key loaded", "present", cfg.gemini.apiKey != "")
	
	if cfg.eBird.apiKey == "" {
		slog.Warn("EBIRD_API_KEY is not set. Premium features for hotspot analysis will fail.")
	}
	if cfg.gemini.apiKey == "" {
		slog.Error("CRITICAL: GEMINI_API_KEY is not set. AI features will fail.")
		return errors.New("GEMINI_API_KEY is not configured")
	}
	if cfg.payos.clientID == "" || cfg.payos.apiKey == "" || cfg.payos.checksumKey == "" {
		slog.Error("CRITICAL: PayOS credentials are not fully configured. PayOS payments will fail.")
		return errors.New("PAYOS_CLIENT_ID, PAYOS_API_KEY, or PAYOS_CHECKSUM_KEY are not configured")
	}
	slog.Info("PayOS credentials loaded", "clientID_present", cfg.payos.clientID != "")

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