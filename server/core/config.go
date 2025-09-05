package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/protobuf/encoding/prototext"

	myncer_pb "github.com/hansbala/myncer/proto/myncer"
)

const (
	// Referenced from the root of the server/ folder.
	cDevConfigPath  = "config.dev.textpb"
	cProdBucketName = "myncer-config"
	cProdObjectPath = "config.prod.textpb"
)

// MustGetConfig loads the application configuration.
// It prioritizes loading from environment variables. If key environment variables are not set,
// it falls back to the original file-based loading mechanism (local file or GCS).
func MustGetConfig() *myncer_pb.Config {
	// First, attempt to load configuration from environment variables.
	if config, loaded := tryLoadConfigFromEnv(); loaded {
		Printf("Configuration successfully loaded from environment variables.")
		return config
	}

	// Fallback to file-based loading if environment variables are not present.
	Warningf("Environment variables for configuration not found. Falling back to file-based methods.")

	// First try getting the config from local filesystem (for dev).
	devConfig, err := maybeGetDevConfig()
	if err == nil {
		Printf("Dev config loaded!")
		return devConfig
	}
	Warningf(
		fmt.Sprintf(
			"failed to get dev config because %s. Falling back to production config",
			err.Error(),
		),
	)
	// Fallback to prod config.
	prodConfig, err := getProdConfig()
	if err != nil {
		Errorf("failed to get production config")
		panic(err)
	}
	return prodConfig
}

// tryLoadConfigFromEnv attempts to build the configuration from environment variables.
// It returns the config and a boolean indicating if the load was successful.
func tryLoadConfigFromEnv() (*myncer_pb.Config, bool) {
	// Use the presence of a key variable as a signal to use this method.
	if os.Getenv("POSTGRES_HOST") == "" {
		return nil, false
	}

	// --- Database Configuration ---
	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getRequiredEnv("POSTGRES_USER"),
		getRequiredEnv("POSTGRES_PASSWORD"),
		getRequiredEnv("POSTGRES_HOST"),
		getEnv("POSTGRES_PORT", "5432"),
		getRequiredEnv("POSTGRES_DB"),
		getEnv("POSTGRES_SSLMODE", "disable"),
	)

	// --- Server Configuration ---
	serverModeStr := getEnv("SERVER_MODE", "PROD")
	var serverMode myncer_pb.ServerMode
	switch strings.ToUpper(serverModeStr) {
	case "PROD":
		serverMode = myncer_pb.ServerMode_PROD
	case "DEV":
		serverMode = myncer_pb.ServerMode_DEV
	default:
		serverMode = myncer_pb.ServerMode_UNSPECIFIED
	}

	// --- Secrets ---
	jwtSecret := getRequiredEnv("JWT_SECRET")

	// --- Spotify Configuration ---
	spotifyConfig := &myncer_pb.SpotifyConfig{
		ClientId: getRequiredEnv("SPOTIFY_CLIENT_ID"),
		ClientSecret: getRequiredEnv("SPOTIFY_CLIENT_SECRET"),
		RedirectUri: getRequiredEnv("SPOTIFY_REDIRECT_URI"),
	}

	// --- YouTube Configuration ---
	youtubeConfig := &myncer_pb.YoutubeConfig{
		ClientId: getRequiredEnv("YOUTUBE_CLIENT_ID"),
		ClientSecret: getRequiredEnv("YOUTUBE_CLIENT_SECRET"),
		RedirectUri: getRequiredEnv("YOUTUBE_REDIRECT_URI"),
	}

	// --- Tidal Configuration ---
	tidalConfig := &myncer_pb.TidalConfig{
		ClientId: getRequiredEnv("TIDAL_CLIENT_ID"),
		ClientSecret: getRequiredEnv("TIDAL_CLIENT_SECRET"),
		RedirectUri: getRequiredEnv("TIDAL_REDIRECT_URI"),
	}

	// --- LLM Configuration ---
	llmEnabled := getEnvAsBool("LLM_ENABLED", false)
	var llmConfig *myncer_pb.LlmConfig

	if llmEnabled {
		llmProviderStr := getEnv("LLM_PROVIDER", "GEMINI")
		var llmProvider myncer_pb.LlmProvider
		switch strings.ToUpper(llmProviderStr) {
		case "GEMINI":
			llmProvider = myncer_pb.LlmProvider_GEMINI
		case "OPENAI":
			llmProvider = myncer_pb.LlmProvider_OPENAI
		default:
			llmProvider = myncer_pb.LlmProvider_LLM_PROVIDER_UNSPECIFIED
		}

		llmConfig = &myncer_pb.LlmConfig{
			Enabled: llmEnabled,
			PreferredProvider: llmProvider,
		}

		// Configure LLM provider specific settings
		if llmProvider == myncer_pb.LlmProvider_GEMINI {
			llmConfig.GeminiConfig = &myncer_pb.GeminiConfig{
				ApiKey: getRequiredEnv("GEMINI_API_KEY"),
			}
		} else if llmProvider == myncer_pb.LlmProvider_OPENAI {
			llmConfig.OpenaiConfig = &myncer_pb.OpenAIConfig{
				ApiKey: getRequiredEnv("OPENAI_API_KEY"),
			}
		}
	} else {
		llmConfig = &myncer_pb.LlmConfig{Enabled: false}
	}

	// Build configuration object
	config := &myncer_pb.Config{
		DatabaseConfig: &myncer_pb.DatabaseConfig{
			DatabaseUrl: databaseUrl,
		},
		ServerMode: serverMode,
		JwtSecret: jwtSecret,
		SpotifyConfig: spotifyConfig,
		YoutubeConfig: youtubeConfig,
		TidalConfig: tidalConfig,
		LlmConfig: llmConfig,
	}

	return config, true
}

// getEnv retrieves an environment variable or returns a fallback value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

// getRequiredEnv retrieves an environment variable and panics if it is not set.
// This prevents the application from starting with a missing critical configuration.
func getRequiredEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		errMessage := fmt.Sprintf("Required environment variable '%s' is not set", key)
		Errorf(errMessage)
		panic(errMessage)
	}
	return value
}

// getEnvAsBool reads an environment variable as a boolean, returning a fallback if not set or invalid.
func getEnvAsBool(key string, fallback bool) bool {
	s := getEnv(key, "")
	if s == "" {
		return fallback
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		Warningf("Could not parse environment variable '%s' as boolean: %v. Using default value: %v", key, err, fallback)
		return fallback
	}
	return b
}

func maybeGetDevConfig() (*myncer_pb.Config, error) {
	bytes, err := os.ReadFile(cDevConfigPath)
	if err != nil {
		return nil, WrappedError(err, "failed to read dev config file")
	}
	config, err := parseConfigFileBytes(bytes)
	if err != nil {
		return nil, WrappedError(err, "failed to parse dev config file bytes")
	}
	return config, nil
}

func getProdConfig() (*myncer_pb.Config, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, WrappedError(err, "failed to create GCS client")
	}
	defer client.Close()

	rc, err := client.Bucket(cProdBucketName).Object(cProdObjectPath).NewReader(ctx)
	if err != nil {
		return nil, WrappedError(err, "failed to open config file from GCS")
	}
	defer rc.Close()

	bytes, err := io.ReadAll(rc)
	if err != nil {
		return nil, WrappedError(err, "failed to read config file bytes from GCS")
	}

	config, err := parseConfigFileBytes(bytes)
	if err != nil {
		return nil, WrappedError(err, "failed to parse prod config bytes")
	}

	return config, nil
}

func parseConfigFileBytes(bytes []byte /*const*/) (*myncer_pb.Config, error) {
	cs := &myncer_pb.Configs{}
	if err := prototext.Unmarshal(bytes, cs); err != nil {
		return nil, WrappedError(err, "failed to unmarshal config proto bytes")
	}
	configs := cs.GetConfig()
	switch len(configs) {
	case 0:
		return nil, NewError("configs file had no valid configs")
	case 1:
		return configs[0], nil
	default:
		return nil, NewError("config file should declare only one config but had multiple")
	}
}
