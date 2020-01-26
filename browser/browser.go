package browser

import (
	"bytes"
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
	"strconv"
	"strings"
	"time"

	"github.com/Diggernaut/goquery"
	"github.com/Diggernaut/mahonia"
	"github.com/Diggernaut/surf/errors"
	"github.com/Diggernaut/surf/jar"
	"github.com/robertkrimen/otto"
	"golang.org/x/net/html/charset"
)

// Attribute represents a Browser capability
type Attribute int

// AttributeMap represents a map of Attribute values.
type AttributeMap map[Attribute]bool

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

	// SetTransport sets the http library transport mechanism for each request.
	SetTransport(t *http.Transport)

	// SetTransport sets the http library transport mechanism for each request.
	GetTransport() *http.Transport

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

	// transport specifies the mechanism by which individual HTTP
	// requests are made.
	transport *http.Transport

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

// PostForm requests the given URL using the POST method with the given data.
func (bow *Browser) PostForm(u string, data url.Values, ref *url.URL) error {
	return bow.Post(u, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()), ref)
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

// SetTransport sets the http library transport mechanism for each request.
func (bow *Browser) SetTransport(t *http.Transport) {
	bow.transport = t
}

// GetTransport gets the http library transport mechanism.
func (bow *Browser) GetTransport() *http.Transport {
	return bow.transport
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

// -- Unexported methods --

// buildClient creates, configures, and returns a *http.Client type.
func (bow *Browser) buildClient() *http.Client {
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

// send uses the given *http.Request to make an HTTP request.
func (bow *Browser) httpRequest(req *http.Request) error {
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
		if resp.StatusCode == 503 && (resp.Header.Get("Server") == "cloudflare-nginx" || resp.Header.Get("Server") == "cloudflare") {
			if !bow.solveCF(resp, req.URL) {
				return bow.httpRequestComplete(req, resp, fmt.Errorf("Page protected with cloudflare with unknown algorythm"))
			}
			return nil
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

// Solve CloudFlare
func (bow *Browser) solveCF(resp *http.Response, rurl *url.URL) bool {
	if strings.Contains(rurl.String(), "chk_jschl") {
		// We are in deadloop
		return false
	}

	time.Sleep(time.Duration(4) * time.Second)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	buff := bytes.NewBuffer(body)
	dom, err := goquery.NewDocumentFromReader(buff)
	if err != nil {
		return false
	}
	host := rurl.Host

	js := dom.Find("script:contains(\"s,t,o,p,b,r,e,a,k,i,n,g\")").Text()
	if strings.Contains(js, "parseInt") {
		re1 := regexp.MustCompile("setTimeout\\(function\\(\\){\\s+(var s,t,o,p,b,r,e,a,k,i,n,g,f.+?\\r?\\n[\\s\\S]+?a\\.value =.+?)\\r?\\n")
		re2 := regexp.MustCompile("a\\.value = (parseInt\\(.+?\\)).+")
		re3 := regexp.MustCompile("\\s{3,}[a-z](?: = |\\.).+")
		re4 := regexp.MustCompile("[\\n\\\\']")

		js = re1.FindAllStringSubmatch(js, -1)[0][1]
		js = re2.ReplaceAllString(js, re2.FindAllStringSubmatch(js, -1)[0][1])
		js = re3.ReplaceAllString(js, "")
		js = re4.ReplaceAllString(js, "")
		js = strings.Replace(js, "return", "", -1)

		jsEngine := otto.New()
		data, err := jsEngine.Eval(js)
		if err != nil {
			return false
		}
		checksum, err := data.ToInteger()
		if err != nil {
			return false
		}
		checksum += int64(len(host))
		if err != nil {
			return false
		}

		jschlVc, _ := dom.Find("input[name=\"jschl_vc\"]").Attr("value")
		pass, _ := dom.Find("input[name=\"pass\"]").Attr("value")
		jschlAnswer := strconv.Itoa(int(checksum))

		u := rurl.Scheme + "://" + rurl.Host + "/cdn-cgi/l/chk_jschl"
		ur, err := url.Parse(u)
		q := ur.Query()
		q.Add("jschl_vc", jschlVc)
		q.Add("pass", pass)
		q.Add("jschl_answer", jschlAnswer)
		ur.RawQuery = q.Encode()

		bow.DelRequestHeader("Cookie")
		bow.DelRequestHeader("Referer")
		bow.AddRequestHeader("Referer", rurl.String())

		cjar := bow.GetCookieJar()
		cook := cjar.Cookies(rurl)
		if cook != nil {
			for _, co := range cook {
				bow.AddRequestHeader("Cookie", co.Name+"="+co.Value)
			}
		}
		bow.Open(ur.String())

		if bow.refresh != nil {
			bow.refresh.Stop()
		}
		return true
	}

	re1 := regexp.MustCompile("setTimeout\\(function\\(\\){\\s+(var s,t,o,p,b,r,e,a,k,i,n,g,f.+?\\r?\\n[\\s\\S]+?a\\.value =.+?)\\r?\\n")
	re2 := regexp.MustCompile("\\s{3,}[a-z](?: = |\\.).+")
	re3 := regexp.MustCompile("[\\n\\\\']")
	re4 := regexp.MustCompile(";\\s*\\d+\\s*$")
	re5 := regexp.MustCompile("a\\.value\\s*\\=")

	js = re1.FindAllStringSubmatch(js, -1)[0][1]
	js = strings.Replace(js, "s,t,o,p,b,r,e,a,k,i,n,g,f,", "s,t = \""+host+"\",o,p,b,r,e,a,k,i,n,g,f,", 1)
	js = re2.ReplaceAllString(js, "")
	js = re3.ReplaceAllString(js, "")
	js = re4.ReplaceAllString(js, "")
	js = re5.ReplaceAllString(js, "return ")

	jsEngine := otto.New()
	data, err := jsEngine.Eval("(function () {" + js + "})()")
	if err != nil {
		return false
	}
	checksum, err := data.ToInteger()
	if err != nil {
		return false
	}
	checksum += int64(len(host))
	if err != nil {
		return false
	}

	jschlVc, _ := dom.Find("input[name=\"jschl_vc\"]").Attr("value")
	pass, _ := dom.Find("input[name=\"pass\"]").Attr("value")

	u := rurl.Scheme + "://" + rurl.Host + "/cdn-cgi/l/chk_jschl"
	ur, err := url.Parse(u)
	q := ur.Query()
	q.Add("jschl_vc", jschlVc)
	q.Add("pass", pass)
	ur.RawQuery = q.Encode() + "&jschl_answer=" + data.String()

	bow.DelRequestHeader("Cookie")
	bow.DelRequestHeader("Referer")
	bow.AddRequestHeader("Referer", rurl.String())
	bow.AddRequestHeader("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	bow.AddRequestHeader("accept-language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	bow.AddRequestHeader("upgrade-insecure-requests", "1")

	bow.Open(ur.String())

	if bow.refresh != nil {
		bow.refresh.Stop()
	}
	return true
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
			if v == match {
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
