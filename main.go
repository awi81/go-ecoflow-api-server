package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
	httpSwagger "github.com/swaggo/http-swagger"
	"go-ecoflow-api-server/constants"
	_ "go-ecoflow-api-server/docs" // Import generated docs package
	"go-ecoflow-api-server/handlers"
	"go-ecoflow-api-server/logger"
	"go-ecoflow-api-server/middleware"
	"go-ecoflow-api-server/service"
	"log/slog"
	"net/http"
)

// @title Ecoflow API Server
// @version 1.0
// @description API for managing Ecoflow devices.
// @BasePath /
// @securityDefinitions.apikey Authorization
// @type apiKey
// @name Authorization
// @in header
// @description Ecoflow Access Token. Use the format: Bearer <access_token>
//
// @securityDefinitions.apikey X-Secret-Token
// @type apiKey
// @name X-Secret-Token
// @in header
// @description Ecoflow Secret Token

// @security Authorization
// @security X-Secret-Token
func main() {
	// Parse command line flags for HTTPS configuration
	tlsCertFile := flag.String("tls-cert", os.Getenv("TLS_CERT_FILE"), "TLS certificate file path")
	tlsKeyFile := flag.String("tls-key", os.Getenv("TLS_KEY_FILE"), "TLS key file path")
	allowedOrigins := flag.String("allowed-origins", os.Getenv("ALLOWED_ORIGINS"), "Comma-separated list of allowed CORS origins (empty for all)")
	port := flag.String("port", os.Getenv("PORT"), "Server port (default: 8080)")
	flag.Parse()

	log := logger.GetLogger(slog.LevelDebug)

	// Tibber API-Key ist jetzt optional
	// Kann als Umgebungsvariable oder pro Request als Header gesetzt werden
	tibberAPIKey := os.Getenv("TIBBER_API_KEY")
	if tibberAPIKey != "" {
		slog.Info("Tibber API key configured")
	}

	router := chi.NewRouter()
	baseHandler := handlers.NewBaseHandler(log, service.GetEcoflowClient)
	deviceHandler := handlers.NewDeviceHandler(baseHandler)
	powerStationHandler := handlers.NewPowerStationHandler(baseHandler)
	pulseHandler := handlers.NewPulseHandler(baseHandler)

	// Configure CORS
	corsConfig := middleware.DefaultCORSConfig()
	if *allowedOrigins != "" {
		corsConfig.AllowedOrigins = strings.Split(*allowedOrigins, ",")
	}

	// create api routes - with auth
	router.Group(func(apiRouter chi.Router) {
		setMiddleware(apiRouter, log, baseHandler, corsConfig)
		deviceHandler.RegisterRoutes(apiRouter)
		powerStationHandler.RegisterRoutes(apiRouter)
	})

	// Tibber routes - without EcoFlow auth (just needs Tibber API key)
	router.Group(func(apiRouter chi.Router) {
		// Only CORS and basic middleware, no auth
		apiRouter.Use(chimiddleware.RequestID)
		apiRouter.Use(chimiddleware.RealIP)
		apiRouter.Use(httplog.RequestLogger(log))
		apiRouter.Use(chimiddleware.Recoverer)
		apiRouter.Use(middleware.CORS(corsConfig))

		// Register Tibber routes (API-Key wird pro Request geprüft)
		tibberHandler := handlers.NewTibberHandler(baseHandler, func(r *http.Request) (*service.TibberClient, error) {
			// Allow overriding the API key via header for client-specific requests
			apiKey := r.Header.Get("X-Tibber-API-Key")
			if apiKey == "" {
				apiKey = tibberAPIKey
			}
			if apiKey == "" {
				return nil, fmt.Errorf("TIBBER_API_KEY not configured - bitte im Frontend eingeben oder TIBBER_API_KEY Umgebungsvariable setzen")
			}
			return service.NewTibberClient(apiKey), nil
		})
		tibberHandler.RegisterRoutes(apiRouter)

		// Register Tibber Pulse routes
		pulseHandler.RegisterRoutes(apiRouter)

		// Register Charging routes
		chargingHandler := handlers.NewChargingHandler(baseHandler, func(r *http.Request) (*service.TibberClient, error) {
			apiKey := r.Header.Get("X-Tibber-API-Key")
			if apiKey == "" {
				apiKey = tibberAPIKey
			}
			if apiKey == "" {
				return nil, fmt.Errorf("TIBBER_API_KEY not configured")
			}
			return service.NewTibberClient(apiKey), nil
		})
		chargingHandler.RegisterRoutes(apiRouter)
	})

	router.Get("/swagger/*", httpSwagger.WrapHandler)

	// Determine server port
	serverPort := ":8080"
	if *port != "" {
		serverPort = ":" + *port
	}

	// Start server with or without TLS
	if *tlsCertFile != "" && *tlsKeyFile != "" {
		slog.Info("Starting Ecoflow API Server with HTTPS on "+serverPort+"...", "cert", *tlsCertFile, "key", *tlsKeyFile)
		err := http.ListenAndServeTLS(serverPort, *tlsCertFile, *tlsKeyFile, router)
		if err != nil {
			log.Error("Failed to start HTTPS server", "error", err)
		}
	} else {
		slog.Warn("Starting Ecoflow API Server on "+serverPort+" without HTTPS - use -tls-cert and -tls-key for HTTPS")
		err := http.ListenAndServe(serverPort, router)
		if err != nil {
			log.Error("Failed to start server", "error", err)
		}
	}
}

func setMiddleware(router chi.Router, log *httplog.Logger, baseHandler *handlers.BaseHandler, corsConfig middleware.CORSConfig) {
	router.Use(chimiddleware.RequestID)                         //add request id to each request
	router.Use(chimiddleware.RealIP)                            //get real ip address for headers
	router.Use(httplog.RequestLogger(log))                      //log all requests without sensitive headers
	router.Use(chimiddleware.Recoverer)                         //recover in case of panic
	router.Use(chimiddleware.Timeout(constants.RequestTimeout)) //max request duration

	// CORS middleware - must be before auth to handle preflight requests
	router.Use(middleware.CORS(corsConfig))

	authheaders := []string{constants.HeaderAuthorization, constants.HeaderXSecretToken}
	router.Use(middleware.NewAuthHeadersMiddleware(baseHandler, authheaders).CheckAuthHeaders)                                   // check mandatory auth headers
	router.Use(middleware.NewRateLimitMiddleware(baseHandler, constants.RateLimit, constants.RateLimitWindowLength).RateLimit()) // rate limit (60 requests per minute)
}
