package dbengine

import (
	"path/filepath"
	"testing"

	"github.com/lib/pq"
	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/engine/providerid"
	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"
)

func TestSQLiteArraysUpdatesFiltersAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	db, err := database.Open(&database.Config{DSN: path, LogLevel: logger.Silent})
	require.NoError(t, err)
	e := New(db)
	require.NoError(t, e.AutoMigrate())
	movie := &model.MovieInfo{ID: "one", Provider: "FANZA", Number: "TEST-1", Title: "Example", Homepage: "https://example.com", CoverURL: "https://example.com/a.png", Genres: pq.StringArray{"Drama", "quote\"comma,", "日本語"}}
	require.NoError(t, e.SaveMovieInfo(movie))
	movie.Title = "Updated example"
	require.NoError(t, e.SaveMovieInfo(movie))
	movie.ID, movie.Number = "two", "TEST-2"
	require.NoError(t, e.SaveMovieInfo(movie))
	actor := &model.ActorInfo{ID: "one", Provider: "TEST", Name: "Example", Homepage: "https://example.com", Aliases: pq.StringArray{"Alias", "日本語"}}
	require.NoError(t, e.SaveActorInfo(actor))
	actor.ID = "two"
	require.NoError(t, e.SaveActorInfo(actor))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	db, err = database.Open(&database.Config{DSN: path, LogLevel: logger.Silent})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	e = New(db)
	require.NoError(t, e.AutoMigrate())
	stored, err := e.GetMovieInfo(providerid.ProviderID{Provider: "fanza", ID: "ONE"})
	require.NoError(t, err)
	require.Equal(t, "Updated example", stored.Title)
	require.Equal(t, movie.Genres, stored.Genres, "existing SQLite array representation must remain readable")
	movies, err := e.SearchMovie("example", MovieSearchOptions{Provider: "fanza", Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, movies, 1)
	actors, err := e.SearchActor("example", ActorSearchOptions{Provider: "test", Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, actors, 1)
	require.Equal(t, actor.Aliases, actors[0].Aliases)
	movies, err = e.SearchMovie("example", MovieSearchOptions{Provider: "other"})
	require.NoError(t, err)
	require.Empty(t, movies)
	require.Error(t, e.SaveActorInfo(&model.ActorInfo{}))
	require.Error(t, e.SaveMovieInfo(&model.MovieInfo{}))
	require.Error(t, e.SaveMovieReviewInfo(&model.MovieReviewInfo{}))
	require.Error(t, e.SaveMovieReviewInfo(&model.MovieReviewInfo{ID: "one", Provider: "FANZA"}))
	require.NoError(t, sqlDB.Close())
	_, err = e.SearchActor("example", ActorSearchOptions{})
	require.Error(t, err)
	_, err = e.SearchMovie("example", MovieSearchOptions{})
	require.Error(t, err)
}
