package config

import (
	"os"
)

type Config struct {
	// Temporal
	TemporalHostPort string
	TemporalNamespace string
	
	// Kafka
	KafkaBrokers []string
	
	// Dapr
	DaprHTTPPort string
	DaprGRPCPort string
	
	// Redis
	RedisAddr string
	RedisPassword string
	RedisDB int
	
	// Database
	DatabaseURL string
	
	// TigerBeetle
	TigerBeetleClusterID uint128
	TigerBeetleAddresses []string
	
	// Lakehouse
	LakehouseEndpoint string
	LakehouseAccessKey string
	LakehouseSecretKey string
	
	// APISIX
	APISIXAdminURL string
	APISIXAdminKey string
	
	// Keycloak
	KeycloakURL string
	KeycloakRealm string
	KeycloakClientID string
	KeycloakClientSecret string
	
	// Permify
	PermifyURL string
}

type uint128 struct {
	High uint64
	Low  uint64
}

func LoadConfig() *Config {
	return &Config{
		TemporalHostPort:  getEnv("TEMPORAL_HOST_PORT", "localhost:7233"),
		TemporalNamespace: getEnv("TEMPORAL_NAMESPACE", "default"),
		
		KafkaBrokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		
		DaprHTTPPort: getEnv("DAPR_HTTP_PORT", "3500"),
		DaprGRPCPort: getEnv("DAPR_GRPC_PORT", "50001"),
		
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       0,
		
		DatabaseURL: getEnv("DATABASE_URL", ""),
		
		TigerBeetleAddresses: []string{getEnv("TIGERBEETLE_ADDRESSES", "3000")},
		
		LakehouseEndpoint:  getEnv("LAKEHOUSE_ENDPOINT", "http://localhost:9000"),
		LakehouseAccessKey: getEnv("LAKEHOUSE_ACCESS_KEY", ""),
		LakehouseSecretKey: getEnv("LAKEHOUSE_SECRET_KEY", ""),
		
		APISIXAdminURL: getEnv("APISIX_ADMIN_URL", "http://localhost:9180"),
		APISIXAdminKey: getEnv("APISIX_ADMIN_KEY", ""),
		
		KeycloakURL:          getEnv("KEYCLOAK_URL", "http://localhost:8080"),
		KeycloakRealm:        getEnv("KEYCLOAK_REALM", "social-protection"),
		KeycloakClientID:     getEnv("KEYCLOAK_CLIENT_ID", "admin-portal"),
		KeycloakClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		
		PermifyURL: getEnv("PERMIFY_URL", "http://localhost:3476"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
