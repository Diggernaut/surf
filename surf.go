// Package surf ensembles other packages into a usable browser.
package surf

import (
	"github.com/diggernaut/surf/agent"
	"github.com/diggernaut/surf/browser"
	"github.com/diggernaut/surf/jar"
)

var (
	// DefaultUserAgent is the global user agent value.
	DefaultUserAgent = agent.Create()

	// DefaultProfile is the default TLS fingerprint profile of the underlying
	// enetx/surf HTTP client.
	DefaultProfile = "chrome"

	// DefaultSendReferer is the global value for the AttributeSendReferer attribute.
	DefaultSendReferer = true

	// DefaultMetaRefreshHandling is the global value for the AttributeHandleRefresh attribute.
	DefaultMetaRefreshHandling = true

	// DefaultFollowRedirects is the global value for the AttributeFollowRedirects attribute.
	DefaultFollowRedirects = true
)

// NewBrowser creates and returns a *browser.Browser type served by the
// classic net/http transport. This keeps the exact pre-enetx behaviour and
// is the safe default for existing consumers. Use NewEnetxBrowser or
// Browser.SetEngine(browser.EngineEnetx) to switch to the enetx/surf HTTP
// client with browser TLS fingerprinting.
func NewBrowser() *browser.Browser {
	bow := &browser.Browser{}
	bow.ClearTimeout()
	bow.SetAsyncStore(jar.NewAsyncStore())
	bow.SetUserAgent(DefaultUserAgent)
	bow.SetState(&jar.State{})
	bow.SetCookieJar(jar.NewMemoryCookies())
	bow.SetBookmarksJar(jar.NewMemoryBookmarks())
	bow.SetHistoryJar(jar.NewMemoryHistory())
	bow.SetHeadersJar(jar.NewMemoryHeaders())
	bow.SetEngine(browser.EngineClassic)
	bow.SetAttributes(browser.AttributeMap{
		browser.SendReferer:         DefaultSendReferer,
		browser.MetaRefreshHandling: DefaultMetaRefreshHandling,
		browser.FollowRedirects:     DefaultFollowRedirects,
	})
	bow.InitConverters()

	return bow
}

// NewEnetxBrowser creates and returns a *browser.Browser type served by the
// enetx/surf HTTP client, impersonating the DefaultProfile browser.
func NewEnetxBrowser() *browser.Browser {
	bow := NewBrowser()
	bow.SetProfile(DefaultProfile)
	bow.SetEngine(browser.EngineEnetx)

	return bow
}
