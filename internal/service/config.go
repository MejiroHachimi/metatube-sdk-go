package service

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultExcludedGenres = "1080p,Blu-ray,Bluray,Blu ray,蓝光,藍光,ブルーレイ,Blu-ray（ブルーレイ）"

type Config struct {
	Address, DSN, DataDir, Token string
	ExcludedGenres               []string
	CacheBytes                   int64
	CacheTTL, RequestTimeout     time.Duration
	MaxConcurrent                int
}

func LoadConfig() (Config, error) {
	c := Config{DataDir: "./data", CacheBytes: 64 << 20, CacheTTL: 24 * time.Hour, RequestTimeout: 25 * time.Second, MaxConcurrent: 4}
	if v := os.Getenv("DATA_DIR"); v != "" {
		c.DataDir = v
	}
	c.DSN = os.Getenv("DSN")
	if c.DSN == "" {
		c.DSN = os.Getenv("DATABASE_URL")
	}
	c.Token = os.Getenv("TOKEN")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("PORT must be between 1 and 65535")
	}
	c.Address = net.JoinHostPort(os.Getenv("BIND"), port)
	excluded, ok := os.LookupEnv("EXCLUDED_GENRES")
	if !ok {
		excluded = DefaultExcludedGenres
	}
	c.ExcludedGenres = strings.Split(excluded, ",")
	for _, p := range []struct {
		name   string
		target *time.Duration
	}{{"IMAGE_CACHE_TTL", &c.CacheTTL}, {"REQUEST_TIMEOUT", &c.RequestTimeout}} {
		if v := os.Getenv(p.name); v != "" {
			d, e := time.ParseDuration(v)
			if e != nil || d <= 0 || d > 30*24*time.Hour {
				return c, fmt.Errorf("%s must be a positive duration no greater than 720h", p.name)
			}
			*p.target = d
		}
	}
	if v := os.Getenv("IMAGE_CACHE_MB"); v != "" {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil || n < 0 || n > 1024 {
			return c, fmt.Errorf("IMAGE_CACHE_MB must be between 0 and 1024")
		}
		c.CacheBytes = n << 20
	}
	if v := os.Getenv("MAX_CONCURRENT"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 64 {
			return c, fmt.Errorf("MAX_CONCURRENT must be between 1 and 64")
		}
		c.MaxConcurrent = n
	}
	return c, nil
}
