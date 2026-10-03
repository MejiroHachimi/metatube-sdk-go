package dbengine

import (
	_ "embed"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
	"gorm.io/gorm/logger"

	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/engine/providerid"
	"github.com/metatube-community/metatube-sdk-go/model"
)

var (
	//go:embed testdata/actor_metadata.json
	actorMetadata string

	//go:embed testdata/movie_metadata.json
	movieMetadata string

	//go:embed testdata/movie_reviews.json
	movieReviews string
)

type DBEngineTestSuite struct {
	suite.Suite

	typ string
	dsn string
	eng DBEngine
}

func TestDBEngineTestSuite(t *testing.T) {
	suites := []struct {
		typ string
		dsn string
	}{
		{
			// SQLite (memory mode)
			typ: database.Sqlite,
			dsn: ":memory:",
		},
	}

	for _, s := range suites {
		t.Run(s.typ, func(t *testing.T) {
			suite.Run(t, &DBEngineTestSuite{typ: s.typ, dsn: s.dsn})
		})
	}
}

func (s *DBEngineTestSuite) SetupSuite() {
	db, err := database.Open(&database.Config{
		DSN:                  s.dsn,
		DisableAutomaticPing: true,
		LogLevel:             logger.Warn,
	})
	s.Require().NoError(err)

	s.eng = New(db)
	err = s.eng.AutoMigrate()
	s.Require().NoError(err)
}

func (s *DBEngineTestSuite) TestVersion() {
	version, err := s.eng.Version()
	s.Require().NoError(err)
	s.Require().NotEmpty(version)
}

func (s *DBEngineTestSuite) TestActor() {
	var jsonData []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Homepage string `json:"homepage"`
	}
	if err := json.
		NewDecoder(strings.NewReader(actorMetadata)).
		Decode(&jsonData); err != nil {
		s.Require().NoError(err)
	}
	for _, data := range jsonData {
		err := s.eng.SaveActorInfo(&model.ActorInfo{
			ID:       data.ID,
			Name:     data.Name,
			Provider: data.Provider,
			Homepage: data.Homepage,
		})
		s.Require().NoError(err)
	}

	s.T().Run("get actor info", func(t *testing.T) {
		got, err := s.eng.GetActorInfo(providerid.MustParse("AV-LEAGUE:1607"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "上原亜衣", got.Name)
	})

	s.T().Run("get actor info (case-insensitive)", func(t *testing.T) {
		got, err := s.eng.GetActorInfo(providerid.MustParse("av-league:1981"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "大場ゆい", got.Name)
	})

	s.T().Run("search actor by name (no case)", func(t *testing.T) {
		actors, err := s.eng.SearchActor("hitOmi", ActorSearchOptions{})
		require.NoError(t, err)
		require.Len(t, actors, 1)
		assert.Equal(t, "7106", actors[0].ID)
	})

	s.T().Run("search actor by name (match)", func(t *testing.T) {
		actors, err := s.eng.SearchActor("加藤あやの", ActorSearchOptions{})
		require.NoError(t, err)
		require.Len(t, actors, 1)
		assert.Equal(t, "2477", actors[0].ID)
	})

	s.T().Run("search actor by name (fuzz)", func(t *testing.T) {
		actors, err := s.eng.SearchActor("加藤", ActorSearchOptions{})
		require.NoError(t, err)
		require.Len(t, actors, 2)
		sort.Slice(actors, func(i, j int) bool { return actors[i].ID < actors[j].ID })
		assert.Equal(t, "2477", actors[0].ID)
		assert.Equal(t, "25352", actors[1].ID)
	})

	s.T().Run("search actor by name (fuzzer)", func(t *testing.T) {
		actors, err := s.eng.SearchActor("三", ActorSearchOptions{Threshold: 0.1})
		require.NoError(t, err)
		require.Len(t, actors, 3)
	})

	s.T().Run("search actor by name (limit)", func(t *testing.T) {
		actors, err := s.eng.SearchActor("夏", ActorSearchOptions{Threshold: 0.1, Limit: 2})
		require.NoError(t, err)
		require.Len(t, actors, 2)
	})

	s.T().Run("search actor by name (not found)", func(t *testing.T) {
		actors, err := s.eng.SearchActor("无名氏", ActorSearchOptions{})
		require.NoError(t, err)
		require.Empty(t, actors)
	})
}

func (s *DBEngineTestSuite) TestMovie() {
	var jsonData []struct {
		ID       string `json:"id"`
		Number   string `json:"number"`
		Title    string `json:"title"`
		Summary  string `json:"summary"`
		Provider string `json:"provider"`
		Homepage string `json:"homepage"`
		ThumbURL string `json:"thumb_url"`
		CoverURL string `json:"cover_url"`
	}
	if err := json.
		NewDecoder(strings.NewReader(movieMetadata)).
		Decode(&jsonData); err != nil {
		s.Require().NoError(err)
	}
	for _, data := range jsonData {
		err := s.eng.SaveMovieInfo(&model.MovieInfo{
			ID:       data.ID,
			Number:   data.Number,
			Title:    data.Title,
			Summary:  data.Summary,
			Provider: data.Provider,
			Homepage: data.Homepage,
			ThumbURL: data.ThumbURL,
			CoverURL: data.CoverURL,
		})
		s.Require().NoError(err)
	}

	s.T().Run("get movie info", func(t *testing.T) {
		got, err := s.eng.GetMovieInfo(providerid.MustParse("FANZA:pred00507"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "PRED-507", got.Number)
	})

	s.T().Run("get movie info (case-insensitive)", func(t *testing.T) {
		got, err := s.eng.GetMovieInfo(providerid.MustParse("fanZA:h_1133HonB006"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "HONB-006", got.Number)
	})

	s.T().Run("search movie by id (no case)", func(t *testing.T) {
		movies, err := s.eng.SearchMovie("juq00588", MovieSearchOptions{})
		require.NoError(t, err)
		require.Len(t, movies, 1)
		assert.Equal(t, "juq00588", movies[0].ID)
	})

	s.T().Run("search movie by number (match)", func(t *testing.T) {
		movies, err := s.eng.SearchMovie("SDMF-033", MovieSearchOptions{})
		require.NoError(t, err)
		require.Len(t, movies, 1)
		assert.Equal(t, "1sdmf00033", movies[0].ID)
	})

	s.T().Run("search movie by number (*match*)", func(t *testing.T) {
		movies, err := s.eng.SearchMovie("HEYZO-0", MovieSearchOptions{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(movies), 12)
		t.Log(jsonify(movies))
	})

	s.T().Run("search movie by number (*match*)", func(t *testing.T) {
		movies, err := s.eng.SearchMovie("ABP", MovieSearchOptions{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(movies), 6)
		t.Log(jsonify(movies))
	})

	s.T().Run("search movie by title (fuzz)", func(t *testing.T) {
		movies, err := s.eng.SearchMovie("密着誘惑してくるささやき淫語お姉", MovieSearchOptions{})
		require.NoError(t, err)
		require.Len(t, movies, 1)
		assert.Equal(t, "ssis00250", movies[0].ID)
		t.Log(jsonify(movies))
	})

	s.T().Run("search movie by title (fuzz&name)", func(t *testing.T) {
		movies, err := s.eng.SearchMovie("本田岬", MovieSearchOptions{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(movies), 1)
		t.Log(jsonify(movies))
	})
}

func (s *DBEngineTestSuite) TestMovie_Reviews() {
	var jsonData []struct {
		ID         string          `json:"id"`
		Provider   string          `json:"provider"`
		RawReviews json.RawMessage `json:"reviews"`
		Reviews    []struct {
			Title   string  `json:"title"`
			Author  string  `json:"author"`
			Comment string  `json:"comment"`
			Score   float64 `json:"score"`
			Date    string  `json:"date"`
		} `json:"-"`
	}
	if err := json.
		NewDecoder(strings.NewReader(movieReviews)).
		Decode(&jsonData); err != nil {
		s.Require().NoError(err)
	}
	for _, data := range jsonData {
		var reviewJSON string
		err := json.Unmarshal(data.RawReviews, &reviewJSON)
		s.Require().NoError(err)
		err = json.Unmarshal([]byte(reviewJSON), &data.Reviews)
		s.Require().NoError(err)
		err = s.eng.SaveMovieReviewInfo(&model.MovieReviewInfo{
			ID:       data.ID,
			Provider: data.Provider,
			Reviews: datatypes.NewJSONSlice(
				slices.Collect(func(yield func(detail *model.MovieReviewDetail) bool) {
					for _, review := range data.Reviews {
						if !yield(&model.MovieReviewDetail{
							Title:   review.Title,
							Author:  review.Author,
							Comment: review.Comment,
							Score:   review.Score,
							Date:    parser.ParseDate(review.Date),
						}) {
							return
						}
					}
				})),
		})
		s.Require().NoError(err)
	}

	s.T().Run("get review info", func(t *testing.T) {
		got, err := s.eng.GetMovieReviewInfo(providerid.MustParse("FANZA:ebwh00024"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.NotEmpty(t, got.Reviews)
	})

	s.T().Run("get review info (case-insensitive)", func(t *testing.T) {
		got, err := s.eng.GetMovieReviewInfo(providerid.MustParse("fanZA:DAsS00465"))
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.NotEmpty(t, got.Reviews)
	})
}

func jsonify(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "\t")
	return string(data)
}
