package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"

	"github.com/sagikazarmark/registry-auth/auth"
	"github.com/sagikazarmark/registry-auth/auth/authn"
	"github.com/sagikazarmark/registry-auth/config"
)

func init() {
	jwt.MarshalSingleStringAsArray = false
}

func main() {
	var (
		configFile string
		addr       string
		debug      bool
		err        error

		realm string
	)

	flag.StringVar(&configFile, "config", "config.yaml", "Configuration file")
	flag.StringVar(&addr, "addr", "localhost:8080", "Address to listen on")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.StringVar(&realm, "realm", "", "Authentication realm")
	flag.Parse()

	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if debug {
		handlerOptions.Level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))

	if realm == "" {
		logger.Error("must provide realm")

		os.Exit(1)
	}

	var config config.Config

	{
		file, err := os.Open(configFile)
		if err != nil {
			logger.Error(fmt.Sprintf("loading config file: %v", err))

			os.Exit(1)
		}
		defer file.Close()

		decoder := yaml.NewDecoder(file)

		err = decoder.Decode(&config)
		if err != nil {
			logger.Error(fmt.Sprintf("decoding config file: %v", err))

			os.Exit(1)
		}
	}

	if err := config.Validate(); err != nil {
		logger.Error(fmt.Sprintf("invalid configuration: %v", err))

		os.Exit(1)
	}

	passwordAuthenticator, err := config.PasswordAuthenticator.New()
	if err != nil {
		logger.Error(fmt.Sprintf("creating authenticator: %v", err))

		os.Exit(1)
	}

	accessTokenIssuer, err := config.AccessTokenIssuer.New()
	if err != nil {
		logger.Error(fmt.Sprintf("creating access token issuer: %v", err))

		os.Exit(1)
	}

	refreshTokenIssuer, err := config.RefreshTokenIssuer.New()
	if err != nil {
		logger.Error(fmt.Sprintf("creating refresh token issuer: %v", err))

		os.Exit(1)
	}

	refreshTokenVerifier, ok := refreshTokenIssuer.(authn.RefreshTokenVerifier)
	if !ok {
		logger.Error("refresh token issuer cannot verify refresh tokens")

		os.Exit(1)
	}

	subjectRepository, ok := passwordAuthenticator.(authn.SubjectRepository)
	if !ok {
		logger.Error("password authenticator should also serve as a subject repository")

		os.Exit(1)
	}

	// TODO: configuration
	refreshTokenAuthenticator := authn.NewRefreshTokenAuthenticator(refreshTokenVerifier, subjectRepository)

	tokenIssuer := auth.TokenIssuer{
		AccessTokenIssuer:  accessTokenIssuer,
		RefreshTokenIssuer: refreshTokenIssuer,
	}

	authenticator := auth.Authenticator{
		PasswordAuthenticator:     passwordAuthenticator,
		RefreshTokenAuthenticator: refreshTokenAuthenticator,
	}

	authorizer, err := config.Authorizer.New()
	if err != nil {
		logger.Error(fmt.Sprintf("creating authorizer issuer: %v", err))

		os.Exit(1)
	}

	var service auth.TokenService

	service = auth.TokenServiceImpl{
		Authenticator: authenticator,
		Authorizer:    authorizer,
		TokenIssuer:   tokenIssuer,
	}
	service = auth.LoggerTokenService{
		Service: service,
		Logger:  logger,
	}

	server := auth.TokenServer{
		Service: service,
		Logger:  logger,
	}

	router := mux.NewRouter()
	router.Path("/token").Methods("GET").HandlerFunc(server.TokenHandler)
	router.Path("/token").Methods("POST").HandlerFunc(server.OAuth2Handler)

	logger.Info("launching server")

	err = http.ListenAndServe(addr, router)
	if err != nil {
		logger.Error(fmt.Sprintf("error serving: %v", err))

		os.Exit(1)
	}
}
