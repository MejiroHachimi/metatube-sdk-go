package engine

import (
	"path/filepath"
	"testing"

	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/internal/envconfig"
	"github.com/metatube-community/metatube-sdk-go/model"
	avleague "github.com/metatube-community/metatube-sdk-go/provider/av-league"
	"gorm.io/gorm/logger"
)

func TestDefaultActorProviders(t *testing.T) {
	e := New(nil)
	for _, name := range []string{"AV-LEAGUE", "Gfriends"} {
		if !e.IsActorProvider(name) {
			t.Errorf("default provider missing: %s", name)
		}
	}
	if e.IsActorProvider("ThePornDBActor") {
		t.Fatal("unconfigured credentialed provider was offered")
	}
	cfg := envconfig.NewConfig()
	cfg.Set("ACCESS_TOKEN", "fixture-token")
	e = New(nil, WithActorProviderConfig("ThePornDBActor", cfg))
	if !e.IsActorProvider("ThePornDBActor") {
		t.Fatal("engine did not apply actor token configuration")
	}
	cfg.Set("PRIORITY", "0")
	e = New(nil, WithActorProviderConfig("ThePornDBActor", cfg))
	if e.IsActorProvider("ThePornDBActor") {
		t.Fatal("explicit provider disable ignored")
	}
}

func TestActorWithoutGfriends(t *testing.T) {
	db, err := database.Open(&database.Config{DSN: filepath.Join(t.TempDir(), "actor.db"), LogLevel: logger.Silent})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	cfg := envconfig.NewConfig()
	cfg.Set("priority", "0")
	e := New(db, WithActorProviderConfig("Gfriends", cfg))
	if err := e.DBAutoMigrate(true); err != nil {
		t.Fatal(err)
	}
	p := avleague.New()
	called := 0
	fetch := func() (*model.ActorInfo, error) {
		called++
		return &model.ActorInfo{ID: "fixture", Name: "Fixture Actor", Provider: p.Name(), Homepage: "https://example.invalid/actor", Height: 165, BloodType: "AB", CupSize: "D", Images: []string{"https://example.invalid/avatar.jpg"}}, nil
	}
	for i := 0; i < 2; i++ {
		info, err := e.getActorInfoWithCallback(p, "fixture", true, fetch)
		if err != nil {
			t.Fatal(err)
		}
		if info.Height != 165 || info.BloodType != "AB" || info.CupSize != "D" || len(info.Images) != 1 {
			t.Fatal("profile was lost without Gfriends")
		}
	}
	if called != 1 {
		t.Fatal("actor detail did not use SQLite cache")
	}
}
