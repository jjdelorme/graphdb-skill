package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"graphdb/internal/config"
	"graphdb/internal/embedding"
	"graphdb/internal/graph"
	"graphdb/internal/query"
)

func TestHealthCheck(t *testing.T) {
	// Initialize Server with nil dependencies.
	s := NewServer(nil, nil, config.Config{}, "test")

	// Issue GET /api/health via httptest.NewRecorder().
	req, err := http.NewRequest("GET", "/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	// Assert status is 200 OK.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Assert body contains {"status":"ok"}.
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("handler returned invalid json: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("handler returned unexpected body: got %v want %v", response["status"], "ok")
	}
}

type mockGraphProvider struct {
	query.GraphProvider
	getNeighborsFunc           func(target string, depth int) (*query.NeighborResult, error)
	SearchSimilarFunctionsFunc func(queryStr string, embedding []float32, limit int) ([]*query.FeatureResult, error)
	WhatIfFunc                 func(targets []string) (*query.WhatIfResult, error)
	SemanticTraceFunc          func(target string) ([]*graph.Path, error)
	GetCommunitiesFunc         func(ctx context.Context, limit int) ([]*query.StructuralCommunityResult, error)
	GetDivergenceFunc          func(ctx context.Context, domainPattern string) ([]*query.DomainDivergenceResult, error)
	GetDualLensSeamsFunc       func(ctx context.Context, modulePattern string, minScore float64, maxCutEdges int, limit int) ([]*query.DualLensSeamResult, error)
}

func (m *mockGraphProvider) GetNeighbors(target string, depth int, limit int) (*query.NeighborResult, error) {
	if m.getNeighborsFunc != nil {
		return m.getNeighborsFunc(target, depth)
	}
	return nil, nil
}

func (m *mockGraphProvider) SearchSimilarFunctions(queryStr string, embedding []float32, limit int) ([]*query.FeatureResult, error) {
	if m.SearchSimilarFunctionsFunc != nil {
		return m.SearchSimilarFunctionsFunc(queryStr, embedding, limit)
	}
	return nil, nil
}

func (m *mockGraphProvider) WhatIf(targets []string) (*query.WhatIfResult, error) {
	if m.WhatIfFunc != nil {
		return m.WhatIfFunc(targets)
	}
	return nil, nil
}

func (m *mockGraphProvider) SemanticTrace(target string) ([]*graph.Path, error) {
	if m.SemanticTraceFunc != nil {
		return m.SemanticTraceFunc(target)
	}
	return nil, nil
}

func (m *mockGraphProvider) GetCommunities(ctx context.Context, limit int) ([]*query.StructuralCommunityResult, error) {
	if m.GetCommunitiesFunc != nil {
		return m.GetCommunitiesFunc(ctx, limit)
	}
	return nil, nil
}

func (m *mockGraphProvider) GetDivergence(ctx context.Context, domainPattern string) ([]*query.DomainDivergenceResult, error) {
	if m.GetDivergenceFunc != nil {
		return m.GetDivergenceFunc(ctx, domainPattern)
	}
	return nil, nil
}

func (m *mockGraphProvider) GetDualLensSeams(ctx context.Context, modulePattern string, minScore float64, maxCutEdges int, limit int) ([]*query.DualLensSeamResult, error) {
	if m.GetDualLensSeamsFunc != nil {
		return m.GetDualLensSeamsFunc(ctx, modulePattern, minScore, maxCutEdges, limit)
	}
	return nil, nil
}

func TestQueryNeighbors(t *testing.T) {
	mockProvider := &mockGraphProvider{
		getNeighborsFunc: func(target string, depth int) (*query.NeighborResult, error) {
			if target != "main" {
				t.Errorf("expected target 'main', got %s", target)
			}
			if depth != 2 {
				t.Errorf("expected depth 2, got %d", depth)
			}
			return &query.NeighborResult{}, nil
		},
	}
	s := NewServer(mockProvider, nil, config.Config{}, "test")

	req, err := http.NewRequest("GET", "/api/query?type=neighbors&target=main&depth=2", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response query.NeighborResult
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("handler returned invalid json: %v", err)
	}
}

type mockEmbedder struct {
	embedding.Embedder
	embedBatchFunc func(texts []string) ([][]float32, error)
}

func (m *mockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	if m.embedBatchFunc != nil {
		return m.embedBatchFunc(texts)
	}
	return nil, nil
}

func TestQuerySearchSimilar(t *testing.T) {
	mockEmbedder := &mockEmbedder{
		embedBatchFunc: func(texts []string) ([][]float32, error) {
			if len(texts) != 1 || texts[0] != "find the database" {
				t.Errorf("unexpected texts to embed: %v", texts)
			}
			return [][]float32{{0.1, 0.2, 0.3}}, nil
		},
	}

	mockProvider := &mockGraphProvider{
		SearchSimilarFunctionsFunc: func(queryStr string, embedding []float32, limit int) ([]*query.FeatureResult, error) {
			if queryStr != "find the database" {
				t.Errorf("unexpected query: %s", queryStr)
			}
			if len(embedding) != 3 || embedding[0] != 0.1 {
				t.Errorf("unexpected embedding: %v", embedding)
			}
			if limit != 5 {
				t.Errorf("expected limit 5, got %d", limit)
			}
			return []*query.FeatureResult{{Score: 0.9}}, nil
		},
	}

	s := NewServer(mockProvider, mockEmbedder, config.Config{}, "test")

	body, _ := json.Marshal(QueryRequest{
		Type:   "search-similar",
		Target: "find the database",
		Limit:  5,
	})
	req, err := http.NewRequest("POST", "/api/query", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response []*query.FeatureResult
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("handler returned invalid json: %v", err)
	}

	if len(response) != 1 || response[0].Score != 0.9 {
		t.Errorf("unexpected response: %v", response)
	}
}

func TestQueryWhatIf(t *testing.T) {
	mockProvider := &mockGraphProvider{
		WhatIfFunc: func(targets []string) (*query.WhatIfResult, error) {
			if len(targets) != 2 || targets[0] != "funcA" || targets[1] != "funcB" {
				t.Errorf("unexpected targets for what-if: %v", targets)
			}
			return &query.WhatIfResult{}, nil
		},
	}
	s := NewServer(mockProvider, nil, config.Config{}, "test")

	req, err := http.NewRequest("GET", "/api/query?type=what-if&target=funcA,funcB", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response query.WhatIfResult
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("handler returned invalid json: %v", err)
	}
}

func TestQuerySemanticTrace(t *testing.T) {
	mockProvider := &mockGraphProvider{
		SemanticTraceFunc: func(target string) ([]*graph.Path, error) {
			if target != "func1" {
				t.Errorf("expected target 'func1', got %s", target)
			}
			return []*graph.Path{}, nil
		},
	}
	s := NewServer(mockProvider, nil, config.Config{}, "test")

	req, err := http.NewRequest("GET", "/api/query?type=semantic-trace&target=func1", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestQueryCommunities(t *testing.T) {
	mockProvider := &mockGraphProvider{
		GetCommunitiesFunc: func(ctx context.Context, limit int) ([]*query.StructuralCommunityResult, error) {
			if limit != 100 {
				t.Errorf("expected limit 100, got %d", limit)
			}
			return []*query.StructuralCommunityResult{
				{
					ID:      "1",
					Name:    "AuthCommunity",
					Size:    42,
					Density: 0.85,
				},
			}, nil
		},
	}
	s := NewServer(mockProvider, nil, config.Config{}, "test")

	req, err := http.NewRequest("GET", "/api/query?type=communities&limit=100", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response []*query.StructuralCommunityResult
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("handler returned invalid json: %v", err)
	}
	if len(response) != 1 || response[0].ID != "1" {
		t.Errorf("unexpected communities response: %v", response)
	}
}

func TestQueryDivergence(t *testing.T) {
	mockProvider := &mockGraphProvider{
		GetDivergenceFunc: func(ctx context.Context, domainPattern string) ([]*query.DomainDivergenceResult, error) {
			if domainPattern != "Billing" {
				t.Errorf("expected domainPattern 'Billing', got %s", domainPattern)
			}
			return []*query.DomainDivergenceResult{
				{
					DomainID:        "dom-1",
					DomainName:      "Billing",
					TotalFunctions:  15,
					DivergenceScore: 0.65,
				},
			}, nil
		},
	}
	s := NewServer(mockProvider, nil, config.Config{}, "test")

	req, err := http.NewRequest("GET", "/api/query?type=divergence&target=Billing", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response []*query.DomainDivergenceResult
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("handler returned invalid json: %v", err)
	}
	if len(response) != 1 || response[0].DomainName != "Billing" {
		t.Errorf("unexpected divergence response: %v", response)
	}
}

func TestQueryDualLensSeams(t *testing.T) {
	mockProvider := &mockGraphProvider{
		GetDualLensSeamsFunc: func(ctx context.Context, modulePattern string, minScore float64, maxCutEdges int, limit int) ([]*query.DualLensSeamResult, error) {
			if maxCutEdges != 4 {
				t.Errorf("expected maxCutEdges 4, got %d", maxCutEdges)
			}
			return []*query.DualLensSeamResult{
				{
					ID:             "seam-1",
					Seam:           "ProcessPayment",
					InternalFanIn:  5,
					VolatileFanOut: 3,
					CutEdges:       2,
					Score:          15.0,
				},
			}, nil
		},
	}
	s := NewServer(mockProvider, nil, config.Config{}, "test")

	req, err := http.NewRequest("GET", "/api/query?type=dual-lens-seams", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response []*query.DualLensSeamResult
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("handler returned invalid json: %v", err)
	}
	if len(response) != 1 || response[0].Score != 15.0 {
		t.Errorf("unexpected dual-lens seams response: %v", response)
	}
}
