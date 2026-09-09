package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/mooc-platform/backend/internal/config"
	"github.com/mooc-platform/backend/internal/course"
	"github.com/mooc-platform/backend/internal/learning"
	"github.com/mooc-platform/backend/internal/media"
	internalMw "github.com/mooc-platform/backend/internal/middleware"
	"github.com/mooc-platform/backend/internal/user"
)

func runMigrations(db *sql.DB) error {
	migrationDirs := []string{"./migrations", "/app/migrations"}
	var targetDir string
	for _, dir := range migrationDirs {
		if _, err := os.Stat(dir); err == nil {
			targetDir = dir
			break
		}
	}

	if targetDir == "" {
		log.Println("[MIGRATIONS] Advertencia: No se encontró la carpeta de migraciones. Omitiendo auto-migración.")
		return nil
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, filepath.Join(targetDir, entry.Name()))
		}
	}

	sort.Strings(upFiles)

	for _, file := range upFiles {
		log.Printf("[MIGRATIONS] Ejecutando migración SQL: %s", filepath.Base(file))
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	log.Println("[MIGRATIONS] Todas las migraciones SQL fueron aplicadas exitosamente.")
	return nil
}

func main() {
	cfg := config.Load()

	// Conectar a PostgreSQL
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Error al conectar con PostgreSQL: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(15 * time.Minute)

	// Ejecutar migraciones automáticas
	if err := runMigrations(db); err != nil {
		log.Fatalf("Error ejecutando migraciones: %v", err)
	}

	// Conectar a Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("[WARNING] No se pudo hacer ping a Redis (%s): %v", cfg.RedisAddr, err)
	}

	// Cliente Asynq para encolar tareas asíncronas
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer asynqClient.Close()

	// Inicializar Servicio de Almacenamiento MinIO/S3
	storageService, err := media.NewStorageService(cfg)
	if err != nil {
		log.Printf("[WARNING] No se pudo conectar con MinIO S3 inmediatamente: %v", err)
	}

	// Inicializar capas de Dominio
	userRepo := user.NewPostgresRepository(db)
	userUC := user.NewUseCase(userRepo)
	userHandler := user.NewHTTPHandler(userUC)

	// Seed de Administrador Principal Inicial
	if err := userUC.SeedDefaultAdmin(context.Background()); err != nil {
		log.Printf("[WARNING] Error al sembrar admin inicial: %v", err)
	}

	courseRepo := course.NewPostgresRepository(db)
	courseUC := course.NewUseCase(courseRepo, userRepo)
	courseHandler := course.NewHTTPHandler(courseUC)

	learningRepo := learning.NewPostgresRepository(db)
	learningUC := learning.NewUseCase(learningRepo, courseRepo, userRepo, fmt.Sprintf("http://localhost:%s", cfg.Port))
	learningHandler := learning.NewHTTPHandler(learningUC)

	mediaHandler := media.NewHTTPHandler(storageService, courseRepo, learningRepo, asynqClient)

	authMw := internalMw.NewAuthMiddleware(userRepo)
	rateLimiter := internalMw.NewRateLimiter(rdb, 100, 1*time.Minute)

	// Router Chi
	r := chi.NewRouter()

	// Middlewares globales
	r.Use(internalMw.SecurityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(internalMw.Idempotency(rdb))
	r.Use(authMw.Authenticate)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key", "X-Admin-ID", "X-Teacher-ID"},
		ExposedHeaders:   []string{"Link", "ETag"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Rate limiter aplicado a autenticación
	r.With(rateLimiter.Middleware).Group(func(r chi.Router) {
		userHandler.RegisterRoutes(r)
	})

	// Endpoint Healthcheck
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok", "timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Registrar rutas de dominios
	courseHandler.RegisterRoutes(r)
	if storageService != nil {
		mediaHandler.RegisterRoutes(r)
	}
	learningHandler.RegisterRoutes(r)

	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Servidor API de la Plataforma MOOC iniciado en %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, r); err != nil {
		log.Fatalf("Error al iniciar el servidor HTTP: %v", err)
	}
}
