package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	elasticsearchgo "github.com/elastic/go-elasticsearch/v9"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/search/domain"
)

const indexVersion = "v1"

type Config struct {
	Addresses []string
	Username  string
	Password  string
	Alias     string
	Timeout   time.Duration
}

type Index struct {
	client *elasticsearchgo.Client
	alias  string
}

func Open(config Config) (*Index, error) {
	if len(config.Addresses) == 0 || strings.TrimSpace(config.Alias) == "" || config.Timeout <= 0 {
		return nil, fmt.Errorf("complete Elasticsearch configuration is required")
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, MaxIdleConns: 50, MaxIdleConnsPerHost: 20,
		IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: config.Timeout,
	}
	options := []elasticsearchgo.Option{
		elasticsearchgo.WithAddresses(config.Addresses...),
		elasticsearchgo.WithTransportOptions(elastictransport.WithTransport(transport)),
		elasticsearchgo.WithRetry(3),
		elasticsearchgo.WithAutoDrainBody(),
	}
	if config.Username != "" {
		options = append(options, elasticsearchgo.WithBasicAuth(config.Username, config.Password))
	}
	client, err := elasticsearchgo.New(options...)
	if err != nil {
		return nil, fmt.Errorf("create Elasticsearch client: %w", err)
	}
	return &Index{client: client, alias: config.Alias}, nil
}

func (i *Index) Ensure(ctx context.Context) (bool, error) {
	response, err := i.perform(ctx, http.MethodHead, "/_alias/"+url.PathEscape(i.alias), nil, nil)
	if err != nil {
		return false, err
	}
	if response.StatusCode == http.StatusOK {
		closeResponse(response)
		return false, nil
	}
	if response.StatusCode != http.StatusNotFound {
		closeResponse(response)
		return false, unavailable("check search alias", fmt.Errorf("status %d", response.StatusCode))
	}
	closeResponse(response)

	physical := i.alias + "-" + indexVersion
	body := map[string]any{
		"settings": map[string]any{
			"number_of_shards": 1, "number_of_replicas": 0,
			"analysis": map[string]any{
				"analyzer": map[string]any{
					"book_text": map[string]any{
						"type": "custom", "tokenizer": "standard",
						"filter": []string{"lowercase", "asciifolding"},
					},
				},
				"normalizer": map[string]any{
					"book_keyword": map[string]any{
						"type": "custom", "filter": []string{"lowercase", "asciifolding"},
					},
				},
			},
		},
		"mappings": map[string]any{
			"dynamic": "strict",
			"properties": map[string]any{
				"id": map[string]any{"type": "keyword"},
				"title": map[string]any{
					"type": "search_as_you_type", "analyzer": "book_text",
					"fields": map[string]any{"raw": map[string]any{"type": "keyword", "normalizer": "book_keyword"}},
				},
				"author": map[string]any{
					"type": "search_as_you_type", "analyzer": "book_text",
					"fields": map[string]any{"raw": map[string]any{"type": "keyword", "normalizer": "book_keyword"}},
				},
				"isbn":             map[string]any{"type": "keyword", "normalizer": "book_keyword"},
				"price_cents":      map[string]any{"type": "long"},
				"stock":            map[string]any{"type": "integer"},
				"seller_id":        map[string]any{"type": "keyword"},
				"popularity_score": map[string]any{"type": "double"},
				"created_at":       map[string]any{"type": "date"},
				"updated_at":       map[string]any{"type": "date"},
			},
		},
		"aliases": map[string]any{i.alias: map[string]any{"is_write_index": true}},
	}
	response, err = i.perform(ctx, http.MethodPut, "/"+url.PathEscape(physical), body, nil)
	if err != nil {
		return false, err
	}
	defer closeResponse(response)
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return true, nil
	}
	if response.StatusCode == http.StatusBadRequest {
		// Another replica may have created the index between HEAD and PUT.
		check, checkErr := i.perform(ctx, http.MethodHead, "/_alias/"+url.PathEscape(i.alias), nil, nil)
		if checkErr == nil {
			defer closeResponse(check)
			if check.StatusCode == http.StatusOK {
				return false, nil
			}
		}
	}
	return false, statusError("create search index", response)
}

func (i *Index) Upsert(ctx context.Context, book domain.BookDocument, version int64) error {
	query := url.Values{"version": {strconv.FormatInt(version, 10)}, "version_type": {"external_gte"}}
	response, err := i.perform(ctx, http.MethodPut,
		"/"+url.PathEscape(i.alias)+"/_doc/"+url.PathEscape(book.ID), book, query)
	if err != nil {
		return err
	}
	defer closeResponse(response)
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return statusError("index book", response)
}

func (i *Index) Delete(ctx context.Context, bookID string, version int64) error {
	query := url.Values{"version": {strconv.FormatInt(version, 10)}, "version_type": {"external_gte"}}
	response, err := i.perform(ctx, http.MethodDelete,
		"/"+url.PathEscape(i.alias)+"/_doc/"+url.PathEscape(bookID), nil, query)
	if err != nil {
		return err
	}
	defer closeResponse(response)
	if response.StatusCode == http.StatusNotFound || (response.StatusCode >= 200 && response.StatusCode < 300) {
		return nil
	}
	return statusError("delete indexed book", response)
}

func (i *Index) BulkUpsert(ctx context.Context, books []domain.BookDocument) error {
	if len(books) == 0 {
		return nil
	}
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	for _, book := range books {
		version := book.UpdatedAt.UTC().UnixNano()
		if version < 1 {
			version = 1
		}
		if err := encoder.Encode(map[string]any{"index": map[string]any{
			"_index": i.alias, "_id": book.ID, "version": version, "version_type": "external_gte",
		}}); err != nil {
			return fmt.Errorf("encode bulk index action: %w", err)
		}
		if err := encoder.Encode(book); err != nil {
			return fmt.Errorf("encode bulk book: %w", err)
		}
	}
	response, err := i.performReader(ctx, http.MethodPost, "/_bulk", &payload, nil, "application/x-ndjson")
	if err != nil {
		return err
	}
	defer closeResponse(response)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return statusError("bulk index books", response)
	}
	var result struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
			Error  any `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return unavailable("decode bulk index response", err)
	}
	if result.Errors {
		for _, item := range result.Items {
			for _, operation := range item {
				if operation.Status >= 300 {
					return unavailable("bulk index contains failed operation", fmt.Errorf("status %d: %v", operation.Status, operation.Error))
				}
			}
		}
	}
	return nil
}

func (i *Index) Search(ctx context.Context, request domain.Request) (domain.Result, error) {
	body := buildSearchBody(request)
	return i.search(ctx, body)
}

func (i *Index) Suggest(ctx context.Context, query string, limit int) (domain.Result, error) {
	body := map[string]any{
		"size":             limit,
		"track_total_hits": false,
		"_source":          []string{"id", "title", "author", "isbn", "price_cents", "stock", "seller_id", "created_at", "updated_at"},
		"query": map[string]any{"bool": map[string]any{
			"minimum_should_match": 1,
			"should": []any{
				map[string]any{"multi_match": map[string]any{
					"query": query, "type": "bool_prefix",
					"fields": []string{"title^5", "title._2gram^4", "title._3gram^3", "author^3", "author._2gram^2", "author._3gram"},
				}},
				map[string]any{"multi_match": map[string]any{
					"query": query, "type": "best_fields", "fields": []string{"title^4", "author^2"},
					"fuzziness": "AUTO", "prefix_length": 1, "max_expansions": 30,
				}},
			},
		}},
		"highlight": highlightConfig(),
	}
	return i.search(ctx, body)
}

func (i *Index) search(ctx context.Context, body map[string]any) (domain.Result, error) {
	response, err := i.perform(ctx, http.MethodPost, "/"+url.PathEscape(i.alias)+"/_search", body, nil)
	if err != nil {
		return domain.Result{}, err
	}
	defer closeResponse(response)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.Result{}, statusError("search books", response)
	}
	var result struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Score     *float64            `json:"_score"`
				Source    domain.BookDocument `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
				Sort      []any               `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.Result{}, unavailable("decode search response", err)
	}
	hits := make([]domain.Hit, 0, len(result.Hits.Hits))
	for _, item := range result.Hits.Hits {
		score := 0.0
		if item.Score != nil {
			score = *item.Score
		}
		highlights := make(map[string]string, len(item.Highlight))
		for field, values := range item.Highlight {
			if len(values) > 0 {
				highlights[field] = values[0]
			}
		}
		hits = append(hits, domain.Hit{Book: item.Source, Score: score, Highlights: highlights, Sort: item.Sort})
	}
	return domain.Result{Hits: hits, Total: result.Hits.Total.Value, TookMS: result.Took}, nil
}

func buildSearchBody(request domain.Request) map[string]any {
	filters := make([]any, 0, 5)
	if request.Filters.MinPriceCents > 0 || request.Filters.MaxPriceCents > 0 {
		rangeQuery := map[string]any{}
		if request.Filters.MinPriceCents > 0 {
			rangeQuery["gte"] = request.Filters.MinPriceCents
		}
		if request.Filters.MaxPriceCents > 0 {
			rangeQuery["lte"] = request.Filters.MaxPriceCents
		}
		filters = append(filters, map[string]any{"range": map[string]any{"price_cents": rangeQuery}})
	}
	if request.Filters.InStock != nil {
		if *request.Filters.InStock {
			filters = append(filters, map[string]any{"range": map[string]any{"stock": map[string]any{"gt": 0}}})
		} else {
			filters = append(filters, map[string]any{"term": map[string]any{"stock": 0}})
		}
	}
	if request.Filters.SellerID != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"seller_id": request.Filters.SellerID}})
	}
	if request.Filters.Author != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"author.raw": request.Filters.Author}})
	}

	var textQuery any = map[string]any{"match_all": map[string]any{}}
	if request.Query != "" {
		textQuery = map[string]any{"bool": map[string]any{
			"minimum_should_match": 1,
			"should": []any{
				map[string]any{"term": map[string]any{"isbn": map[string]any{"value": request.Query, "boost": 15}}},
				map[string]any{"match_phrase": map[string]any{"title": map[string]any{"query": request.Query, "boost": 8}}},
				map[string]any{"multi_match": map[string]any{
					"query": request.Query, "type": "best_fields",
					"fields": []string{"title^5", "author^3", "isbn^8"}, "fuzziness": "AUTO",
					"prefix_length": 1, "max_expansions": 50, "minimum_should_match": "70%",
				}},
				map[string]any{"multi_match": map[string]any{
					"query": request.Query, "type": "bool_prefix",
					"fields": []string{"title^3", "title._2gram^2", "title._3gram", "author^2", "author._2gram", "author._3gram"},
				}},
			},
		}}
	}
	query := map[string]any{"bool": map[string]any{"must": []any{textQuery}, "filter": filters}}
	if request.Query != "" {
		query = map[string]any{"function_score": map[string]any{
			"query": query,
			"functions": []any{
				map[string]any{"filter": map[string]any{"range": map[string]any{"stock": map[string]any{"gt": 0}}}, "weight": 0.25},
				map[string]any{"gauss": map[string]any{"updated_at": map[string]any{"origin": "now", "scale": "365d", "decay": 0.5}}, "weight": 0.15},
			},
			"score_mode": "sum", "boost_mode": "sum",
		}}
	}
	body := map[string]any{
		"size": request.Limit, "track_total_hits": true, "query": query,
		"_source": []string{"id", "title", "author", "isbn", "price_cents", "stock", "seller_id", "created_at", "updated_at"},
		"sort":    searchSort(request.Sort),
	}
	if request.Query != "" {
		body["highlight"] = highlightConfig()
	}
	if len(request.SearchAfter) > 0 {
		body["search_after"] = request.SearchAfter
	}
	return body
}

func searchSort(sort string) []any {
	switch sort {
	case "newest":
		return []any{map[string]any{"created_at": "desc"}, map[string]any{"id": "asc"}}
	case "price_asc":
		return []any{map[string]any{"price_cents": "asc"}, map[string]any{"id": "asc"}}
	case "price_desc":
		return []any{map[string]any{"price_cents": "desc"}, map[string]any{"id": "asc"}}
	default:
		return []any{map[string]any{"_score": "desc"}, map[string]any{"updated_at": "desc"}, map[string]any{"id": "asc"}}
	}
}

func highlightConfig() map[string]any {
	return map[string]any{
		"pre_tags": []string{"<mark>"}, "post_tags": []string{"</mark>"}, "encoder": "html",
		"fields": map[string]any{
			"title":  map[string]any{"number_of_fragments": 0},
			"author": map[string]any{"number_of_fragments": 0},
		},
	}
}

func (i *Index) perform(ctx context.Context, method, path string, body any, query url.Values) (*http.Response, error) {
	if body == nil {
		return i.performReader(ctx, method, path, nil, query, "application/json")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode Elasticsearch request: %w", err)
	}
	return i.performReader(ctx, method, path, bytes.NewReader(payload), query, "application/json")
}

func (i *Index) performReader(ctx context.Context, method, path string, body io.Reader, query url.Values, contentType string) (*http.Response, error) {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("create Elasticsearch request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := i.client.Perform(request)
	if err != nil {
		return nil, unavailable("perform Elasticsearch request", err)
	}
	return response, nil
}

func statusError(operation string, response *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return unavailable(operation, fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(payload))))
}

func unavailable(operation string, err error) error {
	return fmt.Errorf("%w: %s: %w", domain.ErrUnavailable, operation, err)
}

func closeResponse(response *http.Response) {
	if response != nil && response.Body != nil {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
}
