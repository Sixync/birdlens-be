package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"

	firebase "firebase.google.com/go"
	"github.com/joho/godotenv"
	"github.com/sixync/birdlens-be/auth"
	"github.com/sixync/birdlens-be/internal/database"
	"github.com/sixync/birdlens-be/internal/env"
	"github.com/sixync/birdlens-be/internal/jwt"
	"github.com/sixync/birdlens-be/internal/smtp"
	"github.com/sixync/birdlens-be/internal/store"
	mediamanager "github.com/sixync/birdlens-be/internal/store/media_manager"
	"github.com/sixync/birdlens-be/internal/version"
	"github.com/stripe/stripe-go/v82" // Import Stripe SDK
	"google.golang.org/api/option"

	"github.com/lmittmann/tint"
)

func main() {
	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug}))

	err := run(logger)
	if err != nil {
		trace := string(debug.Stack())
		logger.Error(err.Error(), "trace", trace)
		os.Exit(1)
	}
}

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
	stripe                          struct { // Stripe configuration
		secretKey     string
		publishableKey string // Good to have, though not used in this backend part
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
	logger      *slog.Logger
	mailer      *smtp.Mailer
	wg          sync.WaitGroup
	authService *auth.AuthService
	tokenMaker  *jwt.JWTMaker
	mediaClient mediamanager.MediaClient
}

var JobQueue = make(chan EmailJob, 100)

func run(logger *slog.Logger) error {
	var cfg config

	cfg.httpPort = env.GetInt("HTTP_PORT", 6969)
	godotenv.Load("/env/mail.env")
	godotenv.Load("/env/.env")
	godotenv.Load("/env/stripe.env") // Load Stripe environment variables

	cfg.baseURL = env.GetString("BASE_URL", "http://localhost:8090")
	cfg.basicAuth.username = env.GetString("BASIC_AUTH_USERNAME", "admin")
	cfg.basicAuth.hashedPassword = env.GetString("BASIC_AUTH_HASHED_PASSWORD", "$2a$10$jRb2qniNcoCyQM23T59RfeEQUbgdAXfR6S0scynmKfJa5Gj3arGJa")
	cfg.db.dbConn = env.GetString("DB_ADDR", "postgres://admin:password@birdlens-db:5432/birdlens?sslmode=disable")
	cfg.smtp.host = env.GetString("SMTP_HOST", "mailpit")
	cfg.smtp.port = env.GetInt("SMTP_PORT", 1025)
	cfg.smtp.username = env.GetString("SMTP_USERNAME", "")
	cfg.smtp.password = env.GetString("SMTP_PASSWORD", "")
	cfg.smtp.from = env.GetString("SMTP_FROM", "test@example.com")
	cfg.jwt.secretKey = env.GetString("JWT_SECRET_KEY", "THISISASECRETKEYHALLELUJAHBABY123123123123123123123")
	cfg.jwt.accessTokenExpDurationMin = env.GetInt("ACCESS_TOKEN_EXP_MIN", 15)
	cfg.jwt.refreshTokenExpDurationDay = env.GetInt("REFRESH_TOKEN_EXP_DAY", 1)
	cfg.frontEndUrl = env.GetString("FRONTEND_URL", "http://localhost")
	cfg.emailVerificationExpiresInHours = env.GetInt("EMAIL_VERIFICATION_EXPIRES_IN_HOURS", 7)

	// Stripe Configuration
	cfg.stripe.secretKey = env.GetString("STRIPE_SECRET_KEY", "")
	cfg.stripe.publishableKey = env.GetString("STRIPE_PUBLISHABLE_KEY", "")

	stripe.Key = cfg.stripe.secretKey // Initialize Stripe SDK with secret key

	log.Println("frontend url is", cfg.frontEndUrl)
	log.Println("Stripe Secret Key Loaded:", cfg.stripe.secretKey != "")
	log.Println("Stripe Publishable Key Loaded:", cfg.stripe.publishableKey != "")


	showVersion := flag.Bool("version", false, "display version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf("version: %s\n", version.Get())
		return nil
	}

	db, err := database.New(cfg.db.dbConn)
	if err != nil {
		return err
	}
	defer db.Close()

	store := store.NewStore(db)

	mailer, err := smtp.NewMailer(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.from)
	if err != nil {
		return err
	}

	log.Println("host:", cfg.smtp.host, "port:", cfg.smtp.port, "username:", cfg.smtp.username, "from:", cfg.smtp.from, "password:", cfg.smtp.password)

	startWorkerPool(mailer, 5)

	filePath := env.GetString("PATH_TO_FIREBASE_CREDS", "")
	log.Println("path to firebase is", filePath)

	if filePath == "" {
		log.Fatal("please provide path to firebase credentials file")
		return errors.ErrUnsupported
	}

	opt := option.WithCredentialsFile(filePath)
	fbApp, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing firebase app: %v\n", err)
		return err
	}

	c, err := fbApp.Auth(context.Background())
	if err != nil {
		log.Fatalf("error initializing auth client: %v\n", err)
		return err
	}

	auth := auth.NewAuthService(store, c)

	tokenMaker := jwt.NewJWTMaker(cfg.jwt.secretKey)
	godotenv.Load("/env/cloudinary.env")

	cldClient := mediamanager.NewCloudinaryClient()

	if cldClient == nil {
		return fmt.Errorf("failed to create media client")
	}

	app := &application{
		config:      cfg,
		store:       store,
		logger:      logger,
		mailer:      mailer,
		tokenMaker:  tokenMaker,
		authService: auth,
		mediaClient: cldClient,
	}

	return app.serveHTTP()
}

func worker(mailer *smtp.Mailer, id int) {
	for job := range JobQueue {
		log.Printf("Worker %d: processing email job for %s", id, job.Recipient)
		err := mailer.Send(job.Recipient, job.Data, job.Patterns...)
		if err != nil {
			log.Printf("Worker %d: FAILED to send email to %s after retries: %v", id, job.Recipient, err)
		} else {
			log.Printf("Worker %d: successfully sent email to %s", id, job.Recipient)
		}
	}
}

func startWorkerPool(mailer *smtp.Mailer, numWorkers int) {
	for w := 1; w <= numWorkers; w++ {
		go worker(mailer, w)
	}
	log.Printf("Started %d email workers", numWorkers)
}