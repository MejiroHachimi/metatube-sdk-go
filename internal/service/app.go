package service

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/engine"
	"github.com/metatube-community/metatube-sdk-go/engine/providerid"
	"github.com/metatube-community/metatube-sdk-go/internal/envconfig"
	"github.com/metatube-community/metatube-sdk-go/route"
	"github.com/metatube-community/metatube-sdk-go/route/auth"
	"gorm.io/gorm/logger"
)

type token string

func (t token) Valid(s string) bool { return subtle.ConstantTimeCompare([]byte(t), []byte(s)) == 1 }

func Open(c Config) (http.Handler, func() error, error) {
	if c.ImageQueueSize < 0 || c.ImageQueueSize > 64 || c.ImagePixelBudget < 0 || c.MaxConcurrent < 1 || c.RequestTimeout <= 0 || c.CacheTTL <= 0 || c.CacheBytes < 0 {
		return nil, nil, fmt.Errorf("invalid service configuration")
	}
	if c.DSN == "" {
		if err := os.MkdirAll(c.DataDir, 0700); err != nil {
			return nil, nil, fmt.Errorf("create data directory: %w", err)
		}
		c.DSN = filepath.Join(c.DataDir, "metadata.db")
	}
	db, err := database.Open(&database.Config{DSN: c.DSN, LogLevel: logger.Silent, MaxOpenConns: 4})
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("database initialization failed")
	}
	fail := func() (http.Handler, func() error, error) {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("database migration failed")
	}
	if db.Name() == database.Sqlite {
		sqlDB.SetMaxOpenConns(1)
		if db.Exec("PRAGMA busy_timeout=5000").Error != nil {
			return fail()
		}
		if db.Exec("PRAGMA journal_mode=WAL").Error != nil {
			return fail()
		}
	}
	opts := []engine.Option{engine.WithEngineName("metatube"), engine.WithRequestTimeout(5 * time.Second)}
	for name, cfg := range envconfig.ActorProviderConfigs.Iterator() {
		opts = append(opts, engine.WithActorProviderConfig(name, cfg))
	}
	for name, cfg := range envconfig.MovieProviderConfigs.Iterator() {
		opts = append(opts, engine.WithMovieProviderConfig(name, cfg))
	}
	app := engine.New(db, opts...)
	if app.DBAutoMigrate(true) != nil {
		return fail()
	}
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	var validator auth.Validator
	if c.Token != "" {
		validator = token(c.Token)
	}
	router := route.New(app, validator)
	gateway := NewGateway(router, c, func(r *http.Request) error {
		raw := r.URL.Query().Get("url")
		if raw == "" {
			return nil
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 5 {
			return fmt.Errorf("invalid image path")
		}
		pid := providerid.ProviderID{Provider: parts[3], ID: parts[4]}
		var urls []string
		if app.IsActorProvider(pid.Provider) {
			info, err := app.GetActorInfoByProviderID(pid, true)
			if err != nil {
				return err
			}
			urls = info.Images
		} else {
			info, err := app.GetMovieInfoByProviderID(pid, true)
			if err != nil {
				return err
			}
			urls = append([]string{info.ThumbURL, info.BigThumbURL, info.CoverURL, info.BigCoverURL}, info.PreviewImages...)
		}
		if !slices.Contains(urls, raw) {
			return fmt.Errorf("unrecognized image URL")
		}
		return nil
	}, func(ctx context.Context) error { return sqlDB.PingContext(ctx) })
	return gateway, sqlDB.Close, nil
}
