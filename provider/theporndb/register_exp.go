//go:build experimental

package theporndb

import "github.com/metatube-community/metatube-sdk-go/provider"

func init() {
	provider.Register(SceneProviderName, NewThePornDBScene)
	provider.Register(MovieProviderName, NewThePornDBMovie)
}
