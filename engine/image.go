package engine

import (
	"context"
	"image"

	"github.com/metatube-community/metatube-sdk-go/common/number"
	R "github.com/metatube-community/metatube-sdk-go/constant"
	"github.com/metatube-community/metatube-sdk-go/detector"
	"github.com/metatube-community/metatube-sdk-go/engine/providerid"
	"github.com/metatube-community/metatube-sdk-go/imageutil"
	"github.com/metatube-community/metatube-sdk-go/model"
	mt "github.com/metatube-community/metatube-sdk-go/provider"
)

// ImageEngine is a request-scoped view; provider registries remain shared.
type ImageEngine struct {
	*Engine
	ctx context.Context
}

func (e *Engine) Images(ctx context.Context) *ImageEngine { return &ImageEngine{Engine: e, ctx: ctx} }

// Default position constants for different kind of images.
const (
	defaultActorPrimaryImagePosition  = 0.5
	defaultMoviePrimaryImagePosition  = 1.0
	defaultMovieThumbImagePosition    = 0.5
	defaultMovieBackdropImagePosition = 0.0
)

func (e *ImageEngine) GetActorPrimaryImage(pid providerid.ProviderID) (image.Image, error) {
	info, err := e.GetActorInfoByProviderID(pid, true)
	if err != nil {
		return nil, err
	}
	if len(info.Images) == 0 {
		return nil, mt.ErrImageNotFound
	}
	return e.GetImageByURL(
		e.MustGetActorProviderByName(pid.Provider), info.Images[0],
		R.PrimaryImageRatio, defaultActorPrimaryImagePosition, false,
	)
}

func (e *ImageEngine) GetMoviePrimaryImage(pid providerid.ProviderID, ratio, pos float64) (image.Image, error) {
	url, info, err := e.getPreferredMovieImageURLAndInfo(pid, true)
	if err != nil {
		return nil, err
	}
	if ratio < 0 /* default primary ratio */ {
		ratio = R.PrimaryImageRatio
	}
	var auto bool
	if pos < 0 /* manual position disabled */ {
		pos = defaultMoviePrimaryImagePosition
		auto = number.RequiresFaceDetection(info.Number)
	}
	return e.GetImageByURL(
		e.MustGetMovieProviderByName(pid.Provider),
		url, ratio, pos, auto,
	)
}

func (e *ImageEngine) GetMovieThumbImage(pid providerid.ProviderID) (image.Image, error) {
	url, _, err := e.getPreferredMovieImageURLAndInfo(pid, false)
	if err != nil {
		return nil, err
	}
	return e.GetImageByURL(
		e.MustGetMovieProviderByName(pid.Provider), url,
		R.ThumbImageRatio, defaultMovieThumbImagePosition, false,
	)
}

func (e *ImageEngine) GetMovieBackdropImage(pid providerid.ProviderID) (image.Image, error) {
	url, _, err := e.getPreferredMovieImageURLAndInfo(pid, false)
	if err != nil {
		return nil, err
	}
	return e.GetImageByURL(
		e.MustGetMovieProviderByName(pid.Provider), url,
		R.BackdropImageRatio, defaultMovieBackdropImagePosition, false,
	)
}

func (e *ImageEngine) GetImageByURL(provider mt.Provider, url string, ratio, pos float64, auto bool) (img image.Image, err error) {
	if img, err = e.getImageByURL(provider, url); err != nil {
		return
	}
	if auto {
		// only turn on advanced for movie providers.
		advancedMode := e.IsMovieProvider(provider.Name())
		axisR, found := detector.FindPrimaryFaceAxisRatio(img, ratio, advancedMode)
		if found {
			pos = axisR // override the default position with detected position.
		}
	}
	return imageutil.CropImagePosition(img, ratio, pos), nil
}

func (e *ImageEngine) getImageByURL(provider mt.Provider, url string) (img image.Image, err error) {
	if err = e.ctx.Err(); err != nil {
		return
	}
	resp, err := e.Fetch(url, provider)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	img, _, err = imageutil.DecodeLimitedContext(e.ctx, resp.Body)
	return
}

func (e *Engine) getPreferredMovieImageURLAndInfo(pid providerid.ProviderID, thumb bool) (url string, info *model.MovieInfo, err error) {
	info, err = e.GetMovieInfoByProviderID(pid, true)
	if err != nil {
		return
	}
	url = info.CoverURL
	if thumb && info.BigThumbURL != "" /* big thumb > cover */ {
		url = info.BigThumbURL
	} else if !thumb && info.BigCoverURL != "" /* big cover > cover */ {
		url = info.BigCoverURL
	}
	return
}

func (e *Engine) GetActorPrimaryImage(pid providerid.ProviderID) (image.Image, error) {
	return e.Images(context.Background()).GetActorPrimaryImage(pid)
}

func (e *Engine) GetMoviePrimaryImage(pid providerid.ProviderID, ratio, pos float64) (image.Image, error) {
	return e.Images(context.Background()).GetMoviePrimaryImage(pid, ratio, pos)
}

func (e *Engine) GetMovieThumbImage(pid providerid.ProviderID) (image.Image, error) {
	return e.Images(context.Background()).GetMovieThumbImage(pid)
}

func (e *Engine) GetMovieBackdropImage(pid providerid.ProviderID) (image.Image, error) {
	return e.Images(context.Background()).GetMovieBackdropImage(pid)
}

func (e *Engine) GetImageByURL(provider mt.Provider, url string, ratio, pos float64, auto bool) (image.Image, error) {
	return e.Images(context.Background()).GetImageByURL(provider, url, ratio, pos, auto)
}
