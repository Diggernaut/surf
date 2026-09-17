package browser

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/diggernaut/goquery"
	"github.com/diggernaut/mahonia"
	"github.com/diggernaut/surf/errors"
	"github.com/diggernaut/surf/jar"
	"github.com/enetx/g"
	grand "github.com/enetx/g/rand"
	esurf "github.com/enetx/surf"
	"github.com/enetx/surf/profiles"
	"github.com/enetx/surf/profiles/chrome"
	"github.com/enetx/surf/profiles/firefox"
	"golang.org/x/net/html/charset"
)

// Attribute represents a Browser capability
type Attribute int

// AttributeMap represents a map of Attribute values.
type AttributeMap map[Attribute]bool

// Engine represents the HTTP mechanics the browser serves requests with.
type Engine int

const (
	// EngineClassic serves requests with a plain net/http transport, exactly
	// like the browser did before the enetx/surf integration. Impersonation
	// settings are stored but have no effect in this engine.
	EngineClassic Engine = iota

	// EngineEnetx serves requests through the enetx/surf client with browser
	// TLS fingerprinting, HTTP/2 and HTTP/3 support.
	EngineEnetx
)

const (
	// SendRefererAttribute instructs a Browser to send the Referer header.
	SendReferer Attribute = iota

	// MetaRefreshHandlingAttribute instructs a Browser to handle the refresh meta tag.
	MetaRefreshHandling

	// FollowRedirectsAttribute instructs a Browser to follow Location headers.
	FollowRedirects
)

// InitialAssetsArraySize is the initial size when allocating a slice of page
// assets. Increasing this size may lead to a very small performance increase
// when downloading assets from a page with a lot of assets.
var InitialAssetsSliceSize = 20

// Impersonation configures how the underlying HTTP client mimics a real
// browser. The zero value disables fingerprinting and falls back to the
// standard Go TLS client.
type Impersonation struct {
	// Browser to impersonate: "chrome" or "firefox". Empty disables
	// fingerprinting.
	Browser string

	// OS to impersonate: "windows", "macos", "linux", "android", "ios" or
	// "random" (resolved once, when SetImpersonation is called). Android and
	// iOS select the mobile fingerprint variant. Empty defaults to a desktop
	// OS.
	OS string

	// Headers enables the browser profile's default header set and header
	// ordering (sec-ch-ua, Accept, Accept-Encoding with brotli/zstd etc), as
	// well as its HTTP/2 and HTTP/3 settings. The browser's own User-Agent,
	// Referer, cookies and custom headers always take precedence over the
	// profile defaults.
	Headers bool

	// UserAgent replaces the browser's User-Agent with the modern one matching
	// Browser and OS, so the claimed user agent matches the TLS fingerprint.
	UserAgent bool
}

// Browsable represents an HTTP web browser.
type Browsable interface {
	// GetUserAgent sets the user agent.
	GetUserAgent() string

	// SetUserAgent sets the user agent.
	SetUserAgent(ua string)

	// SetAttribute sets a browser instruction attribute.
	SetAttribute(a Attribute, v bool)

	// SetAttributes is used to set all the browser attributes.
	SetAttributes(a AttributeMap)

	// SetState sets the init browser state.
	SetState(sj *jar.State)

	// GetState gets the init browser state.
	GetState() *jar.State

	// SetBookmarksJar sets the bookmarks jar the browser uses.
	SetBookmarksJar(bj jar.BookmarksJar)

	// SetCookieJar is used to set the cookie jar the browser uses.
	SetCookieJar(cj http.CookieJar)

	// GetCookieJar is used to get the cookie jar the browser uses.
	GetCookieJar() http.CookieJar

	// SetHistoryJar is used to set the history jar the browser uses.
	SetHistoryJar(hj jar.History)

	// SetHistoryCapacity is used to set the capacity for history queue
	SetHistoryCapacity(capacity int)

	// SetHeadersJar sets the headers the browser sends with each request.
	SetHeadersJar(h http.Header)

	// SetProxy routes all requests through the proxy at the given URL.
	SetProxy(proxyURL string)

	// ClearProxy disables proxying for requests.
	ClearProxy()

	// SetTLSConfig sets a custom TLS configuration for requests.
	SetTLSConfig(config *tls.Config)

	// SetEngine switches the HTTP mechanics used for requests
	// (EngineClassic or EngineEnetx).
	SetEngine(e Engine)

	// GetEngine returns the HTTP mechanics used for requests.
	GetEngine() Engine

	// SetProfile switches the impersonated browser family ("chrome" or
	// "firefox") of the underlying HTTP client. No effect in the classic
	// engine.
	SetProfile(profile string)

	// SetImpersonation switches the browser impersonation settings of the
	// underlying HTTP client.
	SetImpersonation(imp Impersonation)

	// GetImpersonation returns the active browser impersonation settings.
	GetImpersonation() Impersonation

	// DisableKeepAlives disables HTTP keep-alive connections.
	DisableKeepAlives()

	// CloseIdleConnections closes idle connections of the underlying HTTP client.
	CloseIdleConnections()

	// AddRequestHeader adds a header the browser sends with each request.
	AddRequestHeader(name, value string)

	// GetRequestHeader gets a header the browser sends with each request.
	GetRequestHeader(name string) string

	// GetAllRequestHeaders gets all headers the browser sends with each request.
	GetAllRequestHeaders() string

	// Open requests the given URL using the GET method.
	Open(url string) error

	// Open requests the given URL using the HEAD method.
	Head(url string) error

	// OpenForm appends the data values to the given URL and sends a GET request.
	OpenForm(url string, data url.Values) error

	// OpenBookmark calls Get() with the URL for the bookmark with the given name.
	OpenBookmark(name string) error

	// Post requests the given URL using the POST method.
	Post(url string, contentType string, body io.Reader, ref *url.URL) error

	// PostForm requests the given URL using the POST method with the given data.
	PostForm(url string, data url.Values, ref *url.URL) error

	// PostMultipart requests the given URL using the POST method with the given data using multipart/form-data format.
	PostMultipart(u string, data url.Values, ref *url.URL) error

	// Back loads the previously requested page.
	Back() bool

	// Reload duplicates the last successful request.
	Reload() error

	// Bookmark saves the page URL in the bookmarks with the given name.
	Bookmark(name string) error

	// Click clicks on the page element matched by the given expression.
	Click(expr string) error

	// Form returns the form in the current page that matches the given expr.
	Form(expr string) (Submittable, error)

	// Forms returns an array of every form in the page.
	Forms() []Submittable

	// Links returns an array of every link found in the page.
	Links() []*Link

	// Images returns an array of every image found in the page.
	Images() []*Image

	// Stylesheets returns an array of every stylesheet linked to the document.
	Stylesheets() []*Stylesheet

	// Scripts returns an array of every script linked to the document.
	Scripts() []*Script

	// SiteCookies returns the cookies for the current site.
	SiteCookies() []*http.Cookie

	// ResolveUrl returns an absolute URL for a possibly relative URL.
	ResolveUrl(u *url.URL) *url.URL

	// ResolveStringUrl works just like ResolveUrl, but the argument and return value are strings.
	ResolveStringUrl(u string) (string, error)

	// Download writes the contents of the document to the given writer.
	Download(o io.Writer) (int64, error)

	// Url returns the page URL as a string.
	Url() *url.URL

	// StatusCode returns the response status code.
	StatusCode() int

	// Title returns the page title.
	Title() string

	// ResponseHeaders returns the page headers.
	ResponseHeaders() http.Header

	// Body returns the page body as a string of html.
	Body() string

	// Dom returns the inner *goquery.Selection.
	Dom() *goquery.Selection

	// RequestSize returns the number of bytes for the request.
	RequestSize() int

	// ResponseSize returns the number of bytes for the response.
	ResponseSize() int

	// Find returns the dom selections matching the given expression.
	Find(expr string) *goquery.Selection

	// Register pluggable converter
	SetConverter(content_type string, f func([]byte, string, string) []byte)

	// Unregister pluggable converter
	ClearConverter(content_type string)

	// Set cookie usage settings
	UseCookie(setting bool)
}

// Default is the default Browser implementation.
type Browser struct {
	//timeout
	timeout int
	// AsyncStore
	astore *jar.AsyncStore
	// state is the current browser state.
	state *jar.State

	// userAgent is the User-Agent header value sent with requests.
	userAgent string

	// cookies stores cookies for every site visited by the browser.
	cookies http.CookieJar

	// bookmarks stores the saved bookmarks.
	bookmarks jar.BookmarksJar

	// history stores the visited pages.
	history jar.History

	// engine selects the HTTP mechanics used for requests.
	engine Engine

	// transport is the net/http transport used in the classic engine.
	transport *http.Transport

	// surfClient is the underlying enetx/surf HTTP client used for requests
	// in the enetx engine.
	surfClient *esurf.Client

	// stdClient is the *http.Client adapter around surfClient.
	stdClient *http.Client

	// impersonation is the active browser impersonation setting.
	impersonation Impersonation

	// lastReferer is the Referer header value used by the request currently
	// in flight, needed to re-assert it when profile headers are enabled.
	lastReferer string

	// proxyURL is the proxy used for requests, or empty for a direct connection.
	proxyURL string

	// tlsConfig is an optional custom TLS configuration.
	tlsConfig *tls.Config

	// disableKeepalive disables HTTP keep-alive connections when set.
	disableKeepalive bool

	// headers are additional headers to send with each request.
	headers http.Header

	// attributes is the set browser attributes.
	attributes AttributeMap

	// refresh is a timer used to meta refresh pages.
	refresh *time.Timer

	// body of the current page.
	body []byte

	// pluggable converters
	pluggable_converters map[string]func([]byte, string, string) []byte

	// pluggable_content_type_checker
	pluggableContentTypeChecker []string

	// use cookie flag
	useCookie bool

	// reload counter
	reloadCounter int
	maxReloads    int

	// request/response size
	requestSize  int
	responseSize int
}

// Init pluggable map
func (bow *Browser) InitConverters() {
	bow.pluggable_converters = make(map[string]func([]byte, string, string) []byte)
	bow.pluggableContentTypeChecker = []string{}
}
func (bow *Browser) SetAsyncStore(a *jar.AsyncStore) {
	bow.astore = a
}
func (bow *Browser) GetAsyncStore() *jar.AsyncStore {
	return bow.astore
}
func (bow *Browser) SetContentFixer(content_type string) {
	bow.pluggableContentTypeChecker = append(bow.pluggableContentTypeChecker, content_type)
}
func (bow *Browser) ClearContentFixer(content_type string) {
	i := isInSlice(content_type, bow.pluggableContentTypeChecker)
	if i != -1 {
		bow.pluggableContentTypeChecker = append(bow.pluggableContentTypeChecker[:i], bow.pluggableContentTypeChecker[i+1:]...)
	}

}
func isInSlice(str string, sl []string) int {
	for p, v := range sl {
		if v == str {
			return p
		}
	}
	return -1

}

// Register pluggable converter
func (bow *Browser) SetConverter(content_type string, f func([]byte, string, string) []byte) {
	bow.pluggable_converters[content_type] = f
}

// Unregister pluggable converter
func (bow *Browser) ClearConverter(content_type string) {
	bow.pluggable_converters[content_type] = nil
}

// Open requests the given URL using the GET method.
func (bow *Browser) Open(u string) error {
	ur, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpGET(ur, nil)
}

// Open requests the given URL using the HEAD method.
func (bow *Browser) Head(u string) error {
	ur, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpHEAD(ur, nil)
}

// OpenForm appends the data values to the given URL and sends a GET request.
func (bow *Browser) OpenForm(u string, data url.Values) error {
	ul, err := url.Parse(u)
	if err != nil {
		return err
	}
	ul.RawQuery = data.Encode()

	return bow.Open(ul.String())
}

// OpenBookmark calls Open() with the URL for the bookmark with the given name.
func (bow *Browser) OpenBookmark(name string) error {
	url, err := bow.bookmarks.Read(name)
	if err != nil {
		return err
	}
	return bow.Open(url)
}

// Post requests the given URL using the POST method.
func (bow *Browser) Post(u string, contentType string, body io.Reader, ref *url.URL) error {
	ur, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpPOST(ur, ref, contentType, body)
}

// Put requests the given URL using the PUT method.
func (bow *Browser) Put(u string, contentType string, body io.Reader, ref *url.URL) error {
	ur, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpPUT(ur, ref, contentType, body)
}

// Patch requests the given URL using the PATCH method.
func (bow *Browser) Patch(u string, contentType string, body io.Reader, ref *url.URL) error {
	ur, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpPATCH(ur, ref, contentType, body)
}

// PostForm requests the given URL using the POST method with the given data.
func (bow *Browser) PostForm(u string, data url.Values, ref *url.URL) error {
	return bow.Post(u, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()), ref)
}

// PutForm requests the given URL using the PUT method with the given data.
func (bow *Browser) PutForm(u string, data url.Values, ref *url.URL) error {
	return bow.Put(u, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()), ref)
}

// PatchForm requests the given URL using the PATCH method with the given data.
func (bow *Browser) PatchForm(u string, data url.Values, ref *url.URL) error {
	return bow.Patch(u, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()), ref)
}

// PostMultipart requests the given URL using the POST method with the given data using multipart/form-data format.
func (bow *Browser) PostMultipart(u string, data url.Values, ref *url.URL) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, vs := range data {
		for _, v := range vs {
			writer.WriteField(k, v)
		}
	}
	err := writer.Close()
	if err != nil {
		return err

	}
	return bow.Post(u, writer.FormDataContentType(), body, ref)
}

// Back loads the previously requested page.
//
// Returns a boolean value indicating whether a previous page existed, and was
// successfully loaded.
func (bow *Browser) Back() bool {
	if bow.history.Len() > 1 {
		bow.state = bow.history.Pop()
		return true
	}
	return false
}

// Reload duplicates the last successful request.
func (bow *Browser) Reload() error {
	if bow.state.Request != nil {
		return bow.httpRequest(bow.state.Request)
	}
	return errors.NewPageNotLoaded("Cannot reload, the previous request failed.")
}

// Bookmark saves the page URL in the bookmarks with the given name.
func (bow *Browser) Bookmark(name string) error {
	return bow.bookmarks.Save(name, bow.ResolveUrl(bow.Url()).String())
}

// Click clicks on the page element matched by the given expression.
//
// Currently this is only useful for click on links, which will cause the browser
// to load the page pointed at by the link. Future versions of Surf may support
// JavaScript and clicking on elements will fire the click event.
func (bow *Browser) Click(expr string) error {
	sel := bow.Find(expr)
	if sel.Length() == 0 {
		return errors.NewElementNotFound(
			"Element not found matching expr '%s'.", expr)
	}
	if !sel.Is("a") {
		return errors.NewElementNotFound(
			"Expr '%s' must match an anchor tag.", expr)
	}

	href, err := bow.attrToResolvedUrl("href", sel)
	if err != nil {
		return err
	}

	return bow.httpGET(href, bow.Url())
}

// Form returns the form in the current page that matches the given expr.
func (bow *Browser) Form(expr string) (Submittable, error) {
	sel := bow.Find(expr)
	if sel.Length() == 0 {
		return nil, errors.NewElementNotFound(
			"Form not found matching expr '%s'.", expr)
	}
	if !sel.Is("form") {
		return nil, errors.NewElementNotFound(
			"Expr '%s' does not match a form tag.", expr)
	}

	return NewForm(bow, sel), nil
}

// Forms returns an array of every form in the page.
func (bow *Browser) Forms() []Submittable {
	sel := bow.Find("form")
	len := sel.Length()
	if len == 0 {
		return nil
	}

	forms := make([]Submittable, len)
	sel.Each(func(_ int, s *goquery.Selection) {
		forms = append(forms, NewForm(bow, s))
	})
	return forms
}

// Links returns an array of every link found in the page.
func (bow *Browser) Links() []*Link {
	links := make([]*Link, 0, InitialAssetsSliceSize)
	bow.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, err := bow.attrToResolvedUrl("href", s)
		if err == nil {
			links = append(links, NewLinkAsset(
				href,
				bow.attrOrDefault("id", "", s),
				s.Text(),
			))
		}
	})

	return links
}

// Images returns an array of every image found in the page.
func (bow *Browser) Images() []*Image {
	images := make([]*Image, 0, InitialAssetsSliceSize)
	bow.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, err := bow.attrToResolvedUrl("src", s)
		if err == nil {
			images = append(images, NewImageAsset(
				src,
				bow.attrOrDefault("id", "", s),
				bow.attrOrDefault("alt", "", s),
				bow.attrOrDefault("title", "", s),
			))
		}
	})

	return images
}

// Stylesheets returns an array of every stylesheet linked to the document.
func (bow *Browser) Stylesheets() []*Stylesheet {
	stylesheets := make([]*Stylesheet, 0, InitialAssetsSliceSize)
	bow.Find("link").Each(func(_ int, s *goquery.Selection) {
		rel, ok := s.Attr("rel")
		if ok && rel == "stylesheet" {
			href, err := bow.attrToResolvedUrl("href", s)
			if err == nil {
				stylesheets = append(stylesheets, NewStylesheetAsset(
					href,
					bow.attrOrDefault("id", "", s),
					bow.attrOrDefault("media", "all", s),
					bow.attrOrDefault("type", "text/css", s),
				))
			}
		}
	})

	return stylesheets
}

// Scripts returns an array of every script linked to the document.
func (bow *Browser) Scripts() []*Script {
	scripts := make([]*Script, 0, InitialAssetsSliceSize)
	bow.Find("script").Each(func(_ int, s *goquery.Selection) {
		src, err := bow.attrToResolvedUrl("src", s)
		if err == nil {
			scripts = append(scripts, NewScriptAsset(
				src,
				bow.attrOrDefault("id", "", s),
				bow.attrOrDefault("type", "text/javascript", s),
			))
		}
	})

	return scripts
}

// SiteCookies returns the cookies for the current site.
func (bow *Browser) SiteCookies() []*http.Cookie {
	return bow.cookies.Cookies(bow.Url())
}

// SetState sets the browser state.
func (bow *Browser) SetState(sj *jar.State) {
	bow.state = sj
}

// GetState gets the browser state.
func (bow *Browser) GetState() *jar.State {
	return bow.state
}

// SetCookieJar is used to set the cookie jar the browser uses.
func (bow *Browser) SetCookieJar(cj http.CookieJar) {
	bow.cookies = cj
}

// GetCookieJar is used to get the cookie jar the browser uses.
func (bow *Browser) GetCookieJar() http.CookieJar {
	return bow.cookies
}

// SetUserAgent sets the user agent.
func (bow *Browser) SetUserAgent(userAgent string) {
	bow.userAgent = userAgent
}

// GetUserAgent gets the user agent.
func (bow *Browser) GetUserAgent() string {
	return bow.userAgent
}

// SetAttribute sets a browser instruction attribute.
func (bow *Browser) SetAttribute(a Attribute, v bool) {
	bow.attributes[a] = v
}

// SetAttributes is used to set all the browser attributes.
func (bow *Browser) SetAttributes(a AttributeMap) {
	bow.attributes = a
}

// SetBookmarksJar sets the bookmarks jar the browser uses.
func (bow *Browser) SetBookmarksJar(bj jar.BookmarksJar) {
	bow.bookmarks = bj
}

// SetHistoryJar is used to set the history jar the browser uses.
func (bow *Browser) SetHistoryJar(hj jar.History) {
	bow.history = hj
}

// SetHistoryCapacity is used to set the capacity for history queue
func (bow *Browser) SetHistoryCapacity(capacity int) {
	bow.history.SetCapacity(capacity)
}

// SetHeadersJar sets the headers the browser sends with each request.
func (bow *Browser) SetHeadersJar(h http.Header) {
	bow.headers = h
}

// SetProxy routes all requests through the proxy at the given URL. An empty
// URL disables proxying. URLs without a scheme are treated as HTTP proxies,
// so "host:port" and "//host:port" both mean "http://host:port". Idle
// connections of the replaced HTTP client are closed either way.
func (bow *Browser) SetProxy(proxyURL string) {
	if proxyURL != "" {
		if !strings.Contains(proxyURL, "//") {
			proxyURL = "//" + proxyURL
		}
		if !strings.Contains(proxyURL, "://") {
			proxyURL = "http:" + proxyURL
		}
	}
	bow.proxyURL = proxyURL
	if bow.engine == EngineEnetx {
		bow.reconfigureHTTPClient()
		return
	}
	if bow.transport != nil {
		bow.transport.CloseIdleConnections()
	}
	bow.applyClassicTransport()
}

// ClearProxy disables proxying for requests.
func (bow *Browser) ClearProxy() {
	bow.SetProxy("")
}

// SetTLSConfig sets a custom TLS configuration for requests.
func (bow *Browser) SetTLSConfig(config *tls.Config) {
	bow.tlsConfig = config
	if bow.engine == EngineEnetx {
		bow.reconfigureHTTPClient()
		return
	}
	bow.applyClassicTransport()
}

// SetImpersonation switches the browser impersonation settings of the
// underlying enetx/surf client. See the Impersonation type for the available
// options.
func (bow *Browser) SetImpersonation(imp Impersonation) {
	imp.Browser = strings.ToLower(strings.TrimSpace(imp.Browser))
	imp.OS = strings.ToLower(strings.TrimSpace(imp.OS))
	if imp.OS == "random" {
		imp.OS = grand.Choice(g.SliceOf("windows", "macos", "linux", "android", "ios")).Some()
	}
	bow.impersonation = imp
	if imp.UserAgent {
		if _, osKey, ok := bow.resolveImpersonation(); ok {
			if ua := profileUserAgent(imp.Browser, osKey); ua != "" {
				bow.userAgent = ua
			}
		}
	}
	if bow.engine == EngineEnetx {
		bow.reconfigureHTTPClient()
	}
}

// GetImpersonation returns the active browser impersonation settings. When a
// random OS was requested, the resolved concrete OS is returned.
func (bow *Browser) GetImpersonation() Impersonation {
	return bow.impersonation
}

// SetProfile switches the impersonated browser family while keeping the other
// impersonation options intact. Supported values are "chrome" and "firefox";
// any other value falls back to the standard Go TLS client hello.
func (bow *Browser) SetProfile(profile string) {
	imp := bow.impersonation
	imp.Browser = strings.ToLower(strings.TrimSpace(profile))
	bow.SetImpersonation(imp)
}

// DisableKeepAlives disables HTTP keep-alive connections.
func (bow *Browser) DisableKeepAlives() {
	bow.disableKeepalive = true
	if bow.engine == EngineEnetx {
		bow.reconfigureHTTPClient()
		return
	}
	bow.applyClassicTransport()
}

// CloseIdleConnections closes idle connections of the underlying HTTP client.
func (bow *Browser) CloseIdleConnections() {
	if bow.engine == EngineEnetx {
		if bow.surfClient != nil {
			bow.surfClient.CloseIdleConnections()
		}
		return
	}
	if bow.transport != nil {
		bow.transport.CloseIdleConnections()
	}
}

// SetEngine switches the HTTP mechanics used for requests. The cookie jar,
// headers and history are shared between engines, so switching keeps the
// browsing state; only the underlying connections are re-established.
func (bow *Browser) SetEngine(e Engine) {
	if bow.engine == e {
		return
	}
	bow.engine = e
	if e == EngineEnetx {
		bow.transport = nil
		bow.reconfigureHTTPClient()
		return
	}
	if bow.surfClient != nil {
		bow.surfClient.CloseIdleConnections()
	}
	bow.surfClient, bow.stdClient = nil, nil
	bow.applyClassicTransport()
}

// GetEngine returns the HTTP mechanics used for requests.
func (bow *Browser) GetEngine() Engine {
	return bow.engine
}

// applyClassicTransport (re)creates the classic net/http transport with the
// proxy, TLS and keep-alive settings currently configured on the browser.
func (bow *Browser) applyClassicTransport() {
	if bow.transport == nil {
		bow.transport = &http.Transport{}
	}
	bow.transport.DisableKeepAlives = bow.disableKeepalive
	if bow.tlsConfig != nil {
		bow.transport.TLSClientConfig = bow.tlsConfig
	}
	if bow.proxyURL != "" {
		if u, err := url.Parse(bow.proxyURL); err == nil {
			bow.transport.Proxy = http.ProxyURL(u)
		}
	} else {
		bow.transport.Proxy = nil
	}
}

// resolveImpersonation maps the impersonation settings to an enetx/surf
// profile variant and OS key. ok is false when fingerprinting is disabled.
func (bow *Browser) resolveImpersonation() (variant profiles.Variant, osKey profiles.OSKey, ok bool) {
	osKey = profiles.Windows
	switch bow.impersonation.OS {
	case "macos", "mac":
		osKey = profiles.MacOS
	case "linux":
		osKey = profiles.Linux
	case "android":
		osKey = profiles.Android
	case "ios":
		osKey = profiles.IOS
	}
	switch bow.impersonation.Browser {
	case "chrome":
		if osKey.IsMobile() {
			return chrome.Mobile, osKey, true
		}
		return chrome.Desktop, osKey, true
	case "firefox":
		if osKey.IsMobile() {
			return firefox.Mobile, osKey, true
		}
		return firefox.Desktop, osKey, true
	}
	return profiles.Variant{}, osKey, false
}

// profileUserAgent returns the modern user agent matching the impersonated
// browser family and OS, or an empty string when it is unknown.
func profileUserAgent(browser string, osKey profiles.OSKey) string {
	switch browser {
	case "chrome":
		return chrome.UserAgent.Get(osKey).UnwrapOrDefault().Std()
	case "firefox":
		return firefox.UserAgent.Get(osKey).UnwrapOrDefault().Std()
	}
	return ""
}

// reassertHeaders restores the browser's own headers after the impersonation
// profile applied its defaults, so the configured User-Agent, Referer, cookies
// and custom headers always win over the profile. Used as a low-priority
// (late) request middleware when Impersonation.Headers is enabled.
func (bow *Browser) reassertHeaders(req *esurf.Request) {
	h := req.GetRequest().Header
	if bow.userAgent != "" {
		h.Set("User-Agent", bow.userAgent)
	}
	for key, values := range bow.headers {
		if len(values) > 0 {
			h[key] = append([]string(nil), values...)
		}
	}
	if bow.lastReferer != "" {
		h.Set("Referer", bow.lastReferer)
	} else {
		h.Del("Referer")
	}
	if bow.useCookie && bow.cookies != nil {
		if cookies := bow.cookies.Cookies(req.GetRequest().URL); len(cookies) > 0 {
			parts := make([]string, 0, len(cookies))
			for _, c := range cookies {
				parts = append(parts, c.Name+"="+c.Value)
			}
			h.Set("Cookie", strings.Join(parts, "; "))
		} else {
			h.Del("Cookie")
		}
	} else {
		h.Del("Cookie")
	}
}

// reconfigureHTTPClient rebuilds the underlying enetx/surf client from
// scratch with the current impersonation, proxy, TLS and keep-alive settings,
// and refreshes the *http.Client adapter used to send requests. A fresh
// client is built every time: enetx/surf cannot apply the Impersonate
// profile to a transport that has already served requests ("protocol https
// already registered"), and a fresh transport also guarantees that a cleared
// proxy leaves no stale dialer behind. Idle connections of the replaced
// client are closed.
func (bow *Browser) reconfigureHTTPClient() {
	if bow.engine != EngineEnetx {
		return
	}
	old := bow.surfClient
	bow.surfClient = esurf.NewClient()
	b := bow.surfClient.Builder()
	variant, osKey, ok := bow.resolveImpersonation()
	if ok {
		if bow.impersonation.Headers {
			// Full impersonation: TLS fingerprint, HTTP/2 and HTTP/3
			// settings, header set and header ordering. The browser's own
			// headers are re-asserted afterwards by a late middleware.
			im := b.Impersonate()
			switch osKey {
			case profiles.MacOS:
				im.MacOS()
			case profiles.Linux:
				im.Linux()
			case profiles.Android:
				im.Android()
			case profiles.IOS:
				im.IOS()
			default:
				im.Windows()
			}
			switch bow.impersonation.Browser {
			case "chrome":
				im.Chrome()
			case "firefox":
				im.Firefox()
			}
			b.With(func(req *esurf.Request) error {
				bow.reassertHeaders(req)
				return nil
			}, 100)
		} else {
			// TLS-fingerprint-only mode: the profile headers are not applied,
			// so the browser's own headers are sent untouched.
			ja := b.JA()
			if variant.ShuffleExtensions {
				ja = ja.ShuffleExtensions()
			}
			if variant.HelloSpec != nil {
				ja.SetHelloSpec(*variant.HelloSpec)
			} else {
				ja.SetHelloID(variant.HelloID)
			}
		}
	}
	// enetx/surf defaults to ProxyFromEnvironment and to skipping TLS
	// certificate verification; the browser manages both explicitly.
	b.Proxy(g.String(bow.proxyURL))
	if bow.tlsConfig != nil {
		b.TLSConfig(bow.tlsConfig)
	} else {
		b.SecureTLS()
	}
	if bow.disableKeepalive {
		b.DisableKeepAlive()
	}
	if res := b.Build(); res.IsErr() {
		panic(fmt.Sprintf("surf: cannot configure HTTP client: %v", res.Err()))
	}
	bow.stdClient = bow.surfClient.Std()
	if old != nil {
		old.CloseIdleConnections()
	}
}

// AddRequestHeader sets a header the browser sends with each request.
func (bow *Browser) AddRequestHeader(name, value string) {
	bow.headers.Set(name, value)
}

// GetRequestHeader gets a header the browser sends with each request.
func (bow *Browser) GetRequestHeader(name string) string {
	return bow.headers.Get(name)
}

// GetAllRequestHeaders gets a all headers the browser sends with each request.
func (bow *Browser) GetAllRequestHeaders() string {
	var header string
	for key, val := range bow.headers {
		header += key + ": " + strings.Join(val, ";") + "\n"
	}
	return header
}

// DelRequestHeader deletes a header so the browser will not send it with future requests.
func (bow *Browser) DelRequestHeader(name string) {
	bow.headers.Del(name)
}

// ResolveUrl returns an absolute URL for a possibly relative URL.
func (bow *Browser) ResolveUrl(u *url.URL) *url.URL {
	return bow.Url().ResolveReference(u)
}

// ResolveStringUrl works just like ResolveUrl, but the argument and return value are strings.
func (bow *Browser) ResolveStringUrl(u string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	pu = bow.Url().ResolveReference(pu)
	return pu.String(), nil
}

// Download writes the contents of the document to the given writer.
func (bow *Browser) Download(o io.Writer) (int64, error) {
	buff := bytes.NewBuffer(bow.body)
	return io.Copy(o, buff)
}

// DownloadRaw fetches the given URL with the browser's own client (session
// cookies, proxy, impersonated TLS fingerprint) and writes the raw response
// body to the writer, bypassing the charset conversion pipeline that
// corrupts binary content such as images. Does not touch the browser state
// (the current page, history and bookmarks stay as they are).
func (bow *Browser) DownloadRaw(u *url.URL, ref *url.URL, o io.Writer) (int64, error) {
	req, err := bow.buildRequest("GET", u.String(), ref, nil)
	if err != nil {
		return 0, err
	}
	resp, err := bow.buildClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return io.Copy(o, resp.Body)
}

// DownloadBase64 fetches the given URL like DownloadRaw and returns the raw
// response body base64-encoded — a ready-to-send payload for captcha and
// OCR services.
func (bow *Browser) DownloadBase64(u *url.URL, ref *url.URL) (string, error) {
	var buf bytes.Buffer
	if _, err := bow.DownloadRaw(u, ref, &buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Url returns the page URL as a string.
func (bow *Browser) Url() *url.URL {
	if bow.state.Response == nil {
		// there is a possibility that we issued a request, but for
		// whatever reason the request failed.
		if bow.state.Request != nil {
			return bow.state.Request.URL
		}
		return nil
	}

	return bow.state.Response.Request.URL
}

// StatusCode returns the response status code.
func (bow *Browser) StatusCode() int {
	if bow.state.Response == nil {
		// there is a possibility that we issued a request, but for
		// whatever reason the request failed.
		return 503
	}
	return bow.state.Response.StatusCode
}

// Title returns the page title.
func (bow *Browser) Title() string {
	return bow.state.Dom.Find("title").Text()
}

// ResponseHeaders returns the page headers.
func (bow *Browser) ResponseHeaders() http.Header {
	if bow.state.Response != nil {
		return bow.state.Response.Header
	}
	return http.Header{}
}

// Body returns the page body as a string of html.
func (bow *Browser) Body() string {
	body, _ := bow.state.Dom.First().Html()
	return body
}

// Dom returns the inner *goquery.Selection.
func (bow *Browser) Dom() *goquery.Selection {
	return bow.state.Dom.First()
}

// Find returns the dom selections matching the given expression.
func (bow *Browser) Find(expr string) *goquery.Selection {
	return bow.state.Dom.Find(expr)
}

// SetTimeout set max timeout for build request
func (bow *Browser) SetTimeout(t int) {
	if t == 0 {
		t = 180
	}
	bow.timeout = t
}

// ClearTimeout set max timeout == 180 for build requst
func (bow *Browser) ClearTimeout() {
	bow.timeout = 180
}

// RequestSize returns HTTP request size in bytes
func (bow *Browser) RequestSize() int {
	return bow.requestSize
}

// ResponseSize returns HTTP response size in bytes
func (bow *Browser) ResponseSize() int {
	return bow.responseSize
}

// -- Unexported methods --

// buildClient returns a *http.Client for making the next request. It wraps a
// shallow copy of the enetx/surf std adapter so the transport, connection pool
// and TLS fingerprint are shared, while timeout, cookies and the redirect
// policy stay under browser control.
func (bow *Browser) buildClient() *http.Client {
	if bow.engine == EngineEnetx {
		client := *bow.stdClient
		client.Timeout = time.Duration(time.Duration(bow.timeout) * time.Second)
		if bow.useCookie {
			client.Jar = bow.cookies
		}
		client.CheckRedirect = bow.shouldRedirect

		return &client
	}
	client := &http.Client{}
	client.Timeout = time.Duration(time.Duration(bow.timeout) * time.Second)
	if bow.useCookie {
		client.Jar = bow.cookies
	}
	client.CheckRedirect = bow.shouldRedirect
	if bow.transport != nil {
		client.Transport = bow.transport
	}

	return client
}

// buildRequest creates and returns a *http.Request type.
// Sets any headers that need to be sent with the request.
func (bow *Browser) buildRequest(method, url string, ref *url.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header = copyHeaders(bow.headers)
	req.Header.Set("User-Agent", bow.userAgent)
	if bow.attributes[SendReferer] && ref != nil {
		req.Header.Set("Referer", ref.String())
		bow.lastReferer = ref.String()
	} else {
		bow.lastReferer = ""
	}

	return req, nil
}

// httpGET makes an HTTP GET request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpGET(u *url.URL, ref *url.URL) error {
	req, err := bow.buildRequest("GET", u.String(), ref, nil)
	if err != nil {
		return err
	}
	return bow.httpRequest(req)
}

// httpHEAD makes an HTTP HEAD request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpHEAD(u *url.URL, ref *url.URL) error {
	req, err := bow.buildRequest("HEAD", u.String(), ref, nil)
	if err != nil {
		return err
	}
	return bow.httpRequest(req)
}

// httpPOST makes an HTTP POST request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpPOST(u *url.URL, ref *url.URL, contentType string, body io.Reader) error {
	req, err := bow.buildRequest("POST", u.String(), ref, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	return bow.httpRequest(req)
}

// httpPUT makes an HTTP PUT request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpPUT(u *url.URL, ref *url.URL, contentType string, body io.Reader) error {
	req, err := bow.buildRequest("PUT", u.String(), ref, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	return bow.httpRequest(req)
}

// httpPATCH makes an HTTP PATCH request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpPATCH(u *url.URL, ref *url.URL, contentType string, body io.Reader) error {
	req, err := bow.buildRequest("PATCH", u.String(), ref, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	return bow.httpRequest(req)
}

// send uses the given *http.Request to make an HTTP request.
func (bow *Browser) httpRequest(req *http.Request) error {
	bow.requestSize = 0
	requestDump, err := httputil.DumpRequest(req, true)
	if err == nil {
		bow.requestSize = len(requestDump)
	}
	bow.preSend()
	resp, err := bow.buildClient().Do(req)
	if e, ok := err.(net.Error); ok && e.Timeout() {
		bow.body = []byte(`<html></html>`)
	} else if err != nil {
		if strings.HasSuffix(err.Error(), "Service Unavailable") {
			resp = &http.Response{StatusCode: 503, Request: req}
		}
		bow.body = []byte(`<html></html>`)
		return bow.httpRequestComplete(req, resp, err)
	}
	if resp != nil {
		defer resp.Body.Close()
		if os.Getenv("SURF_DEBUG_HEADERS") != "" {
			d, _ := httputil.DumpRequest(req, false)
			fmt.Fprintln(os.Stderr, "===== [DUMP] =====\n", string(d))
		}
		if os.Getenv("SURF_DEBUG_HEADERS") != "" {
			d, _ := httputil.DumpResponse(resp, false)
			fmt.Fprintln(os.Stderr, "===== [DUMP] =====\n", string(d))
		}
		responseDump, err := httputil.DumpResponse(resp, true)
		if err == nil {
			bow.responseSize = len(responseDump)
		}

		contentType := resp.Header.Get("Content-Type")
		if resp.StatusCode != 403 {
			if contentType == "text/html; charset=GBK" {
				enc := mahonia.NewDecoder("gbk")
				e := enc.NewReader(resp.Body)
				bow.body, err = ioutil.ReadAll(e)
				if err != nil {
					return bow.httpRequestComplete(req, resp, err)
				}
			} else if !bow.contentFix(contentType) {
				fixedBody, err := charset.NewReader(resp.Body, contentType)
				if err == nil {
					bow.body, err = ioutil.ReadAll(fixedBody)
					if err != nil {
						return bow.httpRequestComplete(req, resp, err)
					}

				} else {
					bow.body, err = ioutil.ReadAll(resp.Body)
					if err != nil {
						return bow.httpRequestComplete(req, resp, err)
					}

				}
			} else {
				bow.body, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					return bow.httpRequestComplete(req, resp, err)
				}
			}
			bow.contentConversion(contentType, req.URL.String())
		} else {
			if resp.Body != nil {
				bow.body, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					return bow.httpRequestComplete(req, resp, err)
				}
			} else {
				bow.body = []byte(`<html></html>`)
			}
		}
	} else {
		resp = &http.Response{StatusCode: 503, Request: req}
		bow.body = []byte(`<html></html>`)
	}
	return bow.httpRequestComplete(req, resp, nil)
}

func (bow *Browser) httpRequestComplete(req *http.Request, resp *http.Response, err error) error {
	buff := bytes.NewBuffer(bow.body)
	dom, erro := goquery.NewDocumentFromReader(buff)
	if erro != nil {
		err = erro
	}
	bow.history.Push(bow.state)
	bow.state = jar.NewHistoryState(req, resp, dom)
	bow.postSend()
	bow.reloadCounter = 0
	return err
}

// preSend sets browser state before sending a request.
func (bow *Browser) preSend() {
	if bow.refresh != nil {
		bow.refresh.Stop()

	}
}

// postSend sets browser state after sending a request.
func (bow *Browser) postSend() {
	if isContentTypeHtml(bow.state.Response) && bow.attributes[MetaRefreshHandling] {
		sel := bow.Find("meta[http-equiv='refresh']")
		if sel.Length() > 0 {
			attr, ok := sel.Attr("content")
			if ok {
				dur, err := time.ParseDuration(attr + "s")
				if err == nil {
					if bow.reloadCounter < bow.maxReloads {
						time.Sleep(dur)
						bow.reloadCounter += 1
						bow.Reload()
					}
				}
			}
		}
	}
}

// shouldRedirect is used as the value to http.Client.CheckRedirect.
func (bow *Browser) shouldRedirect(req *http.Request, via []*http.Request) error {
	if bow.attributes[FollowRedirects] {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if len(via) == 0 {
			return nil
		}
		for attr, val := range via[0].Header {
			if _, ok := req.Header[attr]; !ok {
				req.Header[attr] = val
			}
		}
		return nil
	}
	return errors.NewLocation(
		"Redirects are disabled. Cannot follow '%s'.", req.URL.String())
}

// attributeToUrl reads an attribute from an element and returns a url.
func (bow *Browser) attrToResolvedUrl(name string, sel *goquery.Selection) (*url.URL, error) {
	src, ok := sel.Attr(name)
	if !ok {
		return nil, errors.NewAttributeNotFound(
			"Attribute '%s' not found.", name)
	}
	ur, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	return bow.ResolveUrl(ur), nil
}

// attributeOrDefault reads an attribute and returns it or the default value when it's empty.
func (bow *Browser) attrOrDefault(name, def string, sel *goquery.Selection) string {
	a, ok := sel.Attr(name)
	if ok {
		return a
	}
	return def
}

// isContentTypeHtml returns true when the given response sent the "text/html" content type.
func isContentTypeHtml(res *http.Response) bool {
	if res != nil {
		ct := res.Header.Get("Content-Type")
		return ct == "" || strings.Contains(ct, "text/html")
	}
	return false
}

// Manipulate contents with specific content-type
func (bow *Browser) contentConversion(content_type string, url string) {
	if bow.pluggable_converters["*"] != nil {
		bow.body = bow.pluggable_converters["*"](bow.body, content_type, url)
		return
	}
	re := regexp.MustCompile("^([A-z\\/\\.\\+\\-]+)")
	matches := re.FindAllStringSubmatch(content_type, -1)
	if len(matches) > 0 {
		match := matches[0][1]
		if bow.pluggable_converters[match] != nil {
			bow.body = bow.pluggable_converters[match](bow.body, content_type, url)
		}
	}
}

// Manipulate contents with specific content-type
func (bow *Browser) contentAsyncConversion(content_type string, url string, bb []byte) []byte {
	re := regexp.MustCompile("^([A-z\\/\\.\\+\\-]+)")
	matches := re.FindAllStringSubmatch(content_type, -1)
	if len(matches) > 0 {
		match := matches[0][1]
		if bow.pluggable_converters[match] != nil {
			return bow.pluggable_converters[match](bb, content_type, url)
		}
	}
	return bb
}

// Check content before fix Body with specific content-type
func (bow *Browser) contentFix(content_type string) bool {
	re := regexp.MustCompile("^([A-z\\/\\.\\+\\-]+)")
	matches := re.FindAllStringSubmatch(content_type, -1)
	if len(matches) > 0 {
		match := matches[0][1]
		for _, v := range bow.pluggableContentTypeChecker {
			if v == match || v == "*" {
				return true
			}
		}
	}
	return false
}

// copyHeaders makes a copy of headers to avoid mixup between requests
func copyHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	h2 := make(http.Header, len(h))
	for k, v := range h {
		h2[k] = v
	}
	return h2
}

// UseCookie sets mode for using cookies in specific calls
func (bow *Browser) UseCookie(setting bool) {
	bow.useCookie = setting
}

// SetMaxReloads sets max reloads via meta-equip=refresh
func (bow *Browser) SetMaxReloads(max int) {
	bow.maxReloads = max
}

func (bow *Browser) OpenAsync(u, name string) error {
	ur, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpAsyncGET(ur, nil, name)
}

func (bow *Browser) httpAsyncRequest(req *http.Request, name string) error {
	bow.preSend()
	var bb []byte
	bb = []byte(`<html></html>`)
	resp, err := bow.buildClient().Do(req)
	if e, ok := err.(net.Error); ok && e.Timeout() {
		bb = []byte(`<html></html>`)
	}
	if resp != nil {
		defer resp.Body.Close()
		contentType := resp.Header.Get("Content-Type")
		if resp.StatusCode != 403 {
			if contentType == "text/html; charset=GBK" {
				enc := mahonia.NewDecoder("gbk")
				e := enc.NewReader(resp.Body)
				bb, err = ioutil.ReadAll(e)
				if err != nil {
					bb = []byte(`<html></html>`)
				}
			} else if !bow.contentFix(contentType) {
				fixedBody, err := charset.NewReader(resp.Body, contentType)
				if err == nil {
					bb, err = ioutil.ReadAll(fixedBody)
					if err != nil {
						bb = []byte(`<html></html>`)
					}

				} else {
					bb, err = ioutil.ReadAll(resp.Body)
					if err != nil {
						bb = []byte(`<html></html>`)
					}

				}
			} else {
				bb, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					bb = []byte(`<html></html>`)
				}
			}
			bb = bow.contentAsyncConversion(contentType, req.URL.String(), bb)
		} else {
			bb = []byte(`<html></html>`)
		}
	}
	buff := bytes.NewBuffer(bb)
	dom, err := goquery.NewDocumentFromReader(buff)
	if err != nil {
		dom, _ = goquery.NewDocumentFromReader(bytes.NewBuffer([]byte(`<html></html>`)))
	}
	bow.astore.Set(name, dom)
	bow.history.Push(bow.state)
	bow.state = jar.NewHistoryState(req, resp, dom)
	bow.postSend()
	bow.reloadCounter = 0
	return nil
}
func (bow *Browser) httpAsyncGET(u *url.URL, ref *url.URL, name string) error {
	req, err := bow.buildRequest("GET", u.String(), ref, nil)
	if err != nil {
		return err
	}
	return bow.httpAsyncRequest(req, name)
}
