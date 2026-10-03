package dbengine

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/model"
)

var _ DBEngine = (*engine)(nil)

type DBEngine interface {
	actorEngine
	movieEngine
	AutoMigrate() error
	Driver() string
	Version() (string, error)
}

type engine struct {
	db *gorm.DB
}

func New(db *gorm.DB) DBEngine {
	return &engine{db: db}
}

func (e *engine) DB() *gorm.DB {
	return e.db.Session(&gorm.Session{})
}

func (e *engine) Driver() string {
	return e.db.Name()
}

func (e *engine) AutoMigrate() error {
	return e.db.AutoMigrate(&model.MovieInfo{}, &model.ActorInfo{}, &model.MovieReviewInfo{})
}

func (e *engine) Version() (version string, err error) {
	switch e.Driver() {
	case database.Sqlite:
		err = e.DB().Raw("SELECT sqlite_version();").Scan(&version).Error
	default:
		err = fmt.Errorf("unsupported DB type: %s", e.Driver())
	}
	return
}
