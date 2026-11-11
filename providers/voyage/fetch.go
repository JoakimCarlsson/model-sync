package voyage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

const baseURL = "https://docs.voyageai.com/docs"

// refURL is where Voyage publishes the OpenAPI definition of each endpoint,
// which is the only document stating the hard bounds on a request.
const refURL = "https://docs.voyageai.com/reference"

// marketplaceURL is the prefix of an AWS Marketplace listing page.
const marketplaceURL = "https://aws.amazon.com/marketplace/pp/"

// blogURL is the prefix of the posts Voyage's model tables link as the
// announcement of a model, and the only document it dates a model in.
const blogURL = "https://blog.voyageai.com/"

// overviewURL is the model overview MongoDB publishes, Voyage now being part of
// MongoDB. It restates the same tables in the same shape, and it is read
// because it is the only page that still carries the context length and the
// embedding width of a model Voyage's own pages have dropped while continuing
// to serve and to charge for it.
const overviewURL = "https://www.mongodb.com/docs/voyageai/models.md"

// Documents Voyage publishes that this parser reads. Appending .md to a doc
// URL returns its markdown source.
var documentURLs = []string{
	baseURL + "/pricing.md",
	baseURL + "/embeddings.md",
	baseURL + "/multimodal-embeddings.md",
	baseURL + "/contextualized-chunk-embeddings.md",
	baseURL + "/reranker.md",
	baseURL + "/batch-inference.md",
	baseURL + "/rate-limits.md",
	baseURL + "/tokenization.md",
	refURL + "/embeddings-api.md",
	refURL + "/contextualized-embeddings-api.md",
	refURL + "/multimodal-embeddings-api.md",
	refURL + "/reranker-api.md",
	overviewURL,
}

// listingURLs are the AWS Marketplace listings for Voyage models. A listing
// title names one model and is the only display name anyone publishes for it,
// Voyage's own pages and MongoDB's naming a model by its identifier alone.
//
// The pairing of a listing to a model is not configured here: each page states
// its own title, and a title is recorded only against the model whose
// identifier the title reduces to, so a listing that moves or is renamed drops
// out rather than being attached to the wrong model. The listings for the
// models Voyage sold before MongoDB acquired it are not read, because they now
// serve a page with no listing in it.
var listingURLs = []string{
	marketplaceURL + "prodview-7ljacmzvuxxrm",
	marketplaceURL + "prodview-ac6uin3e7r2c4",
	marketplaceURL + "prodview-ambr6owhgdzns",
	marketplaceURL + "prodview-lmjyoeygizdhe",
	marketplaceURL + "prodview-newtixzos6yyc",
	marketplaceURL + "prodview-oezpzvj5usjjk",
	marketplaceURL + "prodview-ohiz4nvrdvuhq",
	marketplaceURL + "prodview-pgzgeftmiyf6y",
	marketplaceURL + "prodview-q7u2e7guotpmk",
	marketplaceURL + "prodview-vyjhj6cisij6a",
	marketplaceURL + "prodview-xf2qrked5snxe",
	marketplaceURL + "prodview-xj76cqxng4wyw",
}

// blogLinkRe matches the announcement post a model table links to. The set of
// posts to read is discovered from the documents rather than listed here, so
// that a model added to a table brings its own date with it.
var blogLinkRe = regexp.MustCompile(
	`https://blog\.voyageai\.com/\d{4}/\d{2}/\d{2}/[a-z0-9-]+`,
)

// announcementURLs returns every announcement post linked from the documents,
// in a fixed order so that a run fetches the same thing twice.
func announcementURLs(docs []catalog.Document) []string {
	seen := map[string]bool{}
	for _, doc := range docs {
		for _, url := range blogLinkRe.FindAllString(string(doc.Body), -1) {
			seen[url] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// Fetch retrieves every document. The pricing page is required; the others
// only add detail, so losing one degrades the run instead of ending it.
//
// The announcement posts are fetched last, because which of them exist is
// stated by the documents fetched first.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	docs, failures := p.getAll(ctx, documentURLs)
	if !slices.ContainsFunc(docs, func(d catalog.Document) bool {
		return isPricing(d.URL)
	}) {
		return nil, errors.Join(failures...)
	}
	extra, more := p.getAll(ctx, listingURLs)
	docs = append(docs, extra...)
	failures = append(failures, more...)
	extra, more = p.getAll(ctx, announcementURLs(docs))
	docs = append(docs, extra...)
	failures = append(failures, more...)
	return docs, errors.Join(failures...)
}

// getAll retrieves a list of documents, returning what it got and what it
// could not get.
func (p *Provider) getAll(
	ctx context.Context,
	urls []string,
) ([]catalog.Document, []error) {
	var (
		docs     []catalog.Document
		failures []error
	)
	for _, url := range urls {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, failures
}

// get retrieves url with at most four attempts within one minute, preserving
// the original document URL. Transient failures use cancellable exponential
// backoff or Retry-After; permanent failures and exhausted retries return errors.
func (p *Provider) get(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	fetchURL := url
	if strings.HasPrefix(url, blogURL) {
		fetchURL = strings.TrimRight(url, "/") + "/"
	}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return catalog.Document{}, fmt.Errorf("fetch %s: %w", url, err)
		}
		doc, retryAfter, retry, err := p.getAttempt(ctx, url, fetchURL)
		if err == nil || !retry || attempt == 3 {
			return doc, err
		}
		timer := time.NewTimer(fetchRetryDelay(retryAfter, attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return catalog.Document{}, fmt.Errorf("fetch %s: %w", url, ctx.Err())
		case <-timer.C:
		}
	}
}

// getAttempt requests fetchURL once and returns a document identified by url,
// the Retry-After header, whether a failure is transient, and any request, HTTP,
// or body-read error. Every response body is closed before returning.
func (p *Provider) getAttempt(
	ctx context.Context,
	url string,
	fetchURL string,
) (catalog.Document, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return catalog.Document{}, "", false, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return catalog.Document{}, "", transientFetchError(err), fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusInternalServerError ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout
		return catalog.Document{}, resp.Header.Get("Retry-After"), retry, fmt.Errorf(
			"fetch %s: %s",
			url,
			resp.Status,
		)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, "", transientFetchError(err), fmt.Errorf("read %s: %w", url, err)
	}
	return catalog.Document{URL: url, Body: body}, "", false, nil
}

// transientFetchError reports whether err represents a temporary network failure,
// connection interruption, or incomplete response rather than cancellation or
// a permanent transport configuration failure.
func transientFetchError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var networkError net.Error
	if errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary()) {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE)
}

// fetchRetryDelay returns the server's Retry-After delay in seconds or HTTP-date
// form, falling back to exponential seconds for the zero-based retry attempt.
// Past dates mean no delay; large numeric values saturate without overflowing.
func fetchRetryDelay(retryAfter string, attempt int) time.Duration {
	value := strings.TrimSpace(retryAfter)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
		const maxDelay = time.Duration(1<<63 - 1)
		if seconds > uint64(maxDelay/time.Second) {
			return maxDelay
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(time.Until(date), 0)
	}
	return time.Second << attempt
}
