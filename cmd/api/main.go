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
	"github.com/sixync/birdlens-be/auth"
	"github.com/sixync/birdlens-be/internal/database"
	"github.com/sixync/birdlens-be/internal/env"
	"github.com/sixync/birdlens-be/internal/jwt"
	"github.com/sixync/birdlens-be/internal/smtp"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/version"
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
	httpPort  int
	baseURL   string
	basicAuth struct {
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
}

type application struct {
	config      config
	store       *store.Storage
	logger      *slog.Logger
	mailer      *smtp.Mailer
	wg          sync.WaitGroup
	authService *auth.AuthService
	tokenMaker  *jwt.JWTMaker
}

func run(logger *slog.Logger) error {
	var cfg config

	cfg.httpPort = env.GetInt("HTTP_PORT", 8090)

	// boilerplate
	cfg.baseURL = env.GetString("BASE_URL", "http://localhost:8090")
	cfg.basicAuth.username = env.GetString("BASIC_AUTH_USERNAME", "admin")
	cfg.basicAuth.hashedPassword = env.GetString("BASIC_AUTH_HASHED_PASSWORD", "$2a$10$jRb2qniNcoCyQM23T59RfeEQUbgdAXfR6S0scynmKfJa5Gj3arGJa")
	cfg.db.dbConn = env.GetString("DB_ADDR", "postgres://admin:password@birdlens-db:5432/birdlens?sslmode=disable")
	cfg.smtp.host = env.GetString("SMTP_HOST", "example.smtp.host")
	cfg.smtp.port = env.GetInt("SMTP_PORT", 25)
	cfg.smtp.username = env.GetString("SMTP_USERNAME", "example_username")
	cfg.smtp.password = env.GetString("SMTP_PASSWORD", "pa55word")
	cfg.smtp.from = env.GetString("SMTP_FROM", "Example Name <no_reply@example.org>")
	cfg.jwt.secretKey = env.GetString("JWT_SECRET_KEY", "THISISASECRETKEYHALLELUJAHBABY123123123123123123123")
	cfg.jwt.accessTokenExpDurationMin = env.GetInt("ACCESS_TOKEN_EXP_MIN", 15)
	cfg.jwt.refreshTokenExpDurationDay = env.GetInt("REFRESH_TOKEN_EXP_DAY", 1)

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

	app := &application{
		config:      cfg,
		store:       store,
		logger:      logger,
		mailer:      mailer,
		tokenMaker:  tokenMaker,
		authService: auth,
	}

	return app.serveHTTP()
}
