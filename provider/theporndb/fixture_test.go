package theporndb

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/metatube-community/metatube-sdk-go/internal/envconfig"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
	"golang.org/x/text/language"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFixtureActorConfigAndProfile(t *testing.T) {
	const profile = `{"id":"fixture-id","slug":"fixture-actor","name":"Fixture Actor","aliases":["Fixture Alias"],"extras":{"birthday":"1990-01-02","height":"165cm","cupsize":"34D","waist":"24","hips":"35"}}`
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "profile", true: "malformed"}[malformed], func(t *testing.T) {
			p := NewThePornDBActor()
			if p.Priority() > 0 {
				t.Fatal("provider without credentials must not be offered")
			}
			if _, err := p.GetActorInfoByID("fixture"); err == nil {
				t.Fatal("missing token must return an error")
			}
			cfg := envconfig.NewConfig()
			cfg.Set("ACCESS_TOKEN", " fixture-token ")
			if err := p.SetConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if p.Priority() <= 0 {
				t.Fatal("configured provider must be enabled")
			}
			requests := 0
			p.Scraper = scraper.NewDefaultScraper(ActorProviderName, actorBaseURL, Priority, language.English, scraper.WithTransport(fixtureTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Header.Get("Authorization") != "Bearer fixture-token" {
					t.Error("configured token was not sent")
				}
				body := `{"data":` + profile + `}`
				if r.URL.RawQuery != "" {
					body = `{"data":[` + profile + `]}`
				}
				if malformed {
					body = `{"data":`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})))
			info, err := p.GetActorInfoByID("fixture")
			if malformed {
				if err == nil {
					t.Fatal("invalid detail JSON accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !info.IsValid() || info.Height != 165 || info.CupSize != "D" || time.Time(info.Birthday).Format("2006-01-02") != "1990-01-02" {
					t.Fatal("profile fields missing")
				}
			}
			results, err := p.SearchActor("Fixture")
			if malformed {
				if err == nil {
					t.Fatal("invalid search JSON accepted")
				}
			} else if err != nil || len(results) != 1 || !results[0].IsValid() {
				t.Fatal("search mapping failed")
			}
			if requests != 2 {
				t.Fatalf("requests: %d", requests)
			}
		})
	}
}
