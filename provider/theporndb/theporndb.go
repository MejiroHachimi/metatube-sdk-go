package theporndb

import (
	"github.com/metatube-community/metatube-sdk-go/provider"
)

// Actor support is included by default and enabled with:
// `export MT_ACTOR_PROVIDER_THEPORNDBACTOR__ACCESS_TOKEN=your-token`
// Movie and scene providers are registered only in experimental builds.

const Priority = 1000

func init() {
	provider.Register(ActorProviderName, NewThePornDBActor)
}
