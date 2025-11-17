package injectors

import (
	"github.com/lechgu/tichy/internal/chunkers"
	"github.com/lechgu/tichy/internal/config"
	"github.com/lechgu/tichy/internal/conversations"
	"github.com/lechgu/tichy/internal/databases"
	"github.com/lechgu/tichy/internal/embedders"
	"github.com/lechgu/tichy/internal/fetchers"
	"github.com/lechgu/tichy/internal/ingestors"
	"github.com/lechgu/tichy/internal/loggers"
	"github.com/lechgu/tichy/internal/responders"
	"github.com/lechgu/tichy/internal/retrievers"
	"github.com/lechgu/tichy/internal/servers"
	"github.com/samber/do/v2"
)

var Default do.Injector

func init() {
	Default = do.New()
	do.Provide(Default, config.New)
	do.Provide(Default, loggers.New)
	do.Provide(Default, databases.New)
	do.Provide(Default, chunkers.New)
	do.Provide(Default, embedders.New)
	do.Provide(Default, ingestors.New)
	do.Provide(Default, retrievers.New)
	do.Provide(Default, responders.New)
	do.Provide(Default, conversations.New)
	do.Provide(Default, servers.New)
	do.ProvideNamed(Default, "text", fetchers.NewText)
}
