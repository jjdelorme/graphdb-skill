package query

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GetDualLensSeams surfaces candidate interface/function seams evaluated by
// the Feathers Actionable Seam Score under structural Leiden community boundaries.
func (p *Neo4jProvider) GetDualLensSeams(ctx context.Context, modulePattern string, minScore float64, maxCutEdges int, limit int) ([]*DualLensSeamResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if maxCutEdges <= 0 {
		maxCutEdges = 4
	}
	if modulePattern == "" {
		modulePattern = ".*"
	}

	query := `
		// Dual-Lens Actionable Seam Detection
		MATCH (s:Function)
		OPTIONAL MATCH (s)-[:IN_COMMUNITY]->(c:StructuralCommunity)
		OPTIONAL MATCH (s)-[:IMPLEMENTS]->(:Feature)-[:PARENT_OF*0..]->(d:Domain)
		OPTIONAL MATCH (s)-[:DEFINED_IN]->(file:File)
		WHERE ($pattern = "" OR $pattern = ".*" OR coalesce(file.file, s.file, "") =~ $pattern OR s.id =~ $pattern OR s.name =~ $pattern)

		// 1. Internal Fan-In: Non-volatile callers within same community
		OPTIONAL MATCH (caller:Function)-[:CALLS]->(s)
		WHERE (caller.is_volatile = false OR caller.is_volatile IS NULL)
		  AND (c IS NULL OR (caller)-[:IN_COMMUNITY]->(c))
		WITH s, c, d, file, count(DISTINCT caller) AS internal_fan_in

		// 2. Volatile Fan-Out: Volatile callees
		OPTIONAL MATCH (s)-[:CALLS]->(callee:Function)
		WHERE callee.is_volatile = true
		WITH s, c, d, file, internal_fan_in, count(DISTINCT callee) AS volatile_fan_out

		// 3. Cut-Edge Count: Boundary crossing calls
		OPTIONAL MATCH (extCaller:Function)-[:CALLS]->(s)
		WHERE c IS NOT NULL AND NOT (extCaller)-[:IN_COMMUNITY]->(c)
		WITH s, c, d, file, internal_fan_in, volatile_fan_out, count(DISTINCT extCaller) AS in_cut_edges

		OPTIONAL MATCH (s)-[:CALLS]->(extCallee:Function)
		WHERE c IS NOT NULL AND NOT (extCallee)-[:IN_COMMUNITY]->(c)
		WITH s, c, d, file, internal_fan_in, volatile_fan_out, in_cut_edges, count(DISTINCT extCallee) AS out_cut_edges

		WITH s, c, d, file, internal_fan_in, volatile_fan_out, 
		     (in_cut_edges + out_cut_edges) AS cut_edges,
		     (toFloat(internal_fan_in * volatile_fan_out) / toFloat(in_cut_edges + out_cut_edges + 1)) AS actionable_score
		WHERE internal_fan_in > 0 AND volatile_fan_out > 0 
		  AND cut_edges <= $maxCutEdges 
		  AND actionable_score >= $minScore
		RETURN s.id AS id,
		       s.name AS seam,
		       coalesce(file.file, s.file, "") AS file,
		       internal_fan_in AS internal_fan_in,
		       volatile_fan_out AS volatile_fan_out,
		       cut_edges AS cut_edges,
		       actionable_score AS score,
		       coalesce(c.name, c.id, "Unassigned") AS community,
		       coalesce(d.name, d.id, "Unassigned") AS domain
		ORDER BY score DESC
		LIMIT $limit
	`

	params := map[string]any{
		"pattern":     modulePattern,
		"minScore":    minScore,
		"maxCutEdges": maxCutEdges,
		"limit":       limit,
	}

	res, err := p.executeQuery(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get dual-lens seams: %w", err)
	}

	results := make([]*DualLensSeamResult, 0, len(res.Records))
	for _, record := range res.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		seam, _, _ := neo4j.GetRecordValue[string](record, "seam")
		file, _, _ := neo4j.GetRecordValue[string](record, "file")
		internalFanInVal, _, _ := neo4j.GetRecordValue[int64](record, "internal_fan_in")
		volatileFanOutVal, _, _ := neo4j.GetRecordValue[int64](record, "volatile_fan_out")
		cutEdgesVal, _, _ := neo4j.GetRecordValue[int64](record, "cut_edges")
		score, _, _ := neo4j.GetRecordValue[float64](record, "score")
		comm, _, _ := neo4j.GetRecordValue[string](record, "community")
		domain, _, _ := neo4j.GetRecordValue[string](record, "domain")

		results = append(results, &DualLensSeamResult{
			ID:             id,
			Seam:           seam,
			File:           file,
			InternalFanIn:  int(internalFanInVal),
			VolatileFanOut: int(volatileFanOutVal),
			CutEdges:       int(cutEdgesVal),
			Score:          score,
			Community:      comm,
			Domain:         domain,
		})
	}

	return results, nil
}

// GetDivergence calculates domain dispersion and divergence across structural communities.
func (p *Neo4jProvider) GetDivergence(ctx context.Context, domainPattern string) ([]*DomainDivergenceResult, error) {
	if domainPattern == "" {
		domainPattern = ".*"
	}

	query := `
		// Query Cross-Lens Domain Divergence
		MATCH (d:Domain)
		WHERE ($domain = "" OR $domain = ".*" OR d.name =~ $domain OR d.id =~ $domain)
		MATCH (fn:Function)-[:IMPLEMENTS]->(:Feature)-[:PARENT_OF*0..]->(d)
		OPTIONAL MATCH (fn)-[:IN_COMMUNITY]->(c:StructuralCommunity)
		WITH d, count(DISTINCT fn) AS total_funcs, c, count(DISTINCT fn) AS comm_funcs
		ORDER BY d.name, comm_funcs DESC
		WITH d, total_funcs,
		     collect({
		       community_id: coalesce(c.id, "unassigned"),
		       community_name: coalesce(c.name, "Unassigned"),
		       function_count: comm_funcs,
		       ratio: toFloat(comm_funcs) / toFloat(total_funcs)
		     }) AS distribution
		RETURN d.id AS domain_id,
		       d.name AS domain_name,
		       total_funcs AS total_functions,
		       distribution
		ORDER BY d.name ASC
	`

	params := map[string]any{
		"domain": domainPattern,
	}

	res, err := p.executeQuery(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain divergence: %w", err)
	}

	results := make([]*DomainDivergenceResult, 0, len(res.Records))
	for _, record := range res.Records {
		domainID, _, _ := neo4j.GetRecordValue[string](record, "domain_id")
		domainName, _, _ := neo4j.GetRecordValue[string](record, "domain_name")
		totalFunctionsVal, _, _ := neo4j.GetRecordValue[int64](record, "total_functions")
		totalFunctions := int(totalFunctionsVal)

		rawDist, _, _ := neo4j.GetRecordValue[[]any](record, "distribution")
		distribution := make([]CommunityDistributionItem, 0, len(rawDist))
		maxRatio := 0.0

		for _, item := range rawDist {
			if itemMap, ok := item.(map[string]any); ok {
				cID, _ := itemMap["community_id"].(string)
				cName, _ := itemMap["community_name"].(string)
				var fnCount int
				if cnt, ok := itemMap["function_count"].(int64); ok {
					fnCount = int(cnt)
				} else if cnt, ok := itemMap["function_count"].(int); ok {
					fnCount = cnt
				} else if cnt, ok := itemMap["function_count"].(float64); ok {
					fnCount = int(cnt)
				}
				var ratio float64
				if r, ok := itemMap["ratio"].(float64); ok {
					ratio = r
				} else if r, ok := itemMap["ratio"].(int64); ok {
					ratio = float64(r)
				}

				if ratio > maxRatio {
					maxRatio = ratio
				}

				distribution = append(distribution, CommunityDistributionItem{
					CommunityID:   cID,
					CommunityName: cName,
					FunctionCount: fnCount,
					Ratio:         ratio,
				})
			}
		}

		divergenceScore := 0.0
		if totalFunctions > 0 {
			divergenceScore = 1.0 - maxRatio
		}

		results = append(results, &DomainDivergenceResult{
			DomainID:        domainID,
			DomainName:      domainName,
			TotalFunctions:  totalFunctions,
			DivergenceScore: divergenceScore,
			Distribution:    distribution,
		})
	}

	return results, nil
}

// GetCommunities returns detected structural communities, their internal edge density,
// shared boundary count, cross-cutting hub count, and dominant mapped semantic domains.
func (p *Neo4jProvider) GetCommunities(ctx context.Context, limit int) ([]*StructuralCommunityResult, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		// Query Structural Communities
		MATCH (c:StructuralCommunity)
		OPTIONAL MATCH (fn:Function)-[:IN_COMMUNITY]->(c)
		OPTIONAL MATCH (sb:SharedBoundary)-[:BRIDGES]->(c)
		OPTIONAL MATCH (hub:CrossCuttingHub)-[:INFRASTRUCTURE_OF]->(c)
		OPTIONAL MATCH (fn)-[:IMPLEMENTS]->(:Feature)-[:PARENT_OF*0..]->(d:Domain)
		WITH c,
		     count(DISTINCT fn) AS member_count,
		     count(DISTINCT sb) AS shared_boundary_count,
		     count(DISTINCT hub) AS cross_cutting_hub_count,
		     collect(DISTINCT d.name)[0..3] AS dominant_domains
		RETURN c.id AS id,
		       c.name AS name,
		       c.gamma AS gamma,
		       coalesce(c.size, member_count) AS size,
		       c.density AS density,
		       c.internal_edge_count AS internal_edge_count,
		       c.bpr_avg AS bpr_avg,
		       shared_boundary_count,
		       cross_cutting_hub_count,
		       dominant_domains
		ORDER BY size DESC
		LIMIT $limit
	`

	params := map[string]any{
		"limit": limit,
	}

	res, err := p.executeQuery(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get communities: %w", err)
	}

	results := make([]*StructuralCommunityResult, 0, len(res.Records))
	for _, record := range res.Records {
		id, _, _ := neo4j.GetRecordValue[string](record, "id")
		name, _, _ := neo4j.GetRecordValue[string](record, "name")
		gamma, _, _ := neo4j.GetRecordValue[float64](record, "gamma")
		sizeVal, _, _ := neo4j.GetRecordValue[int64](record, "size")
		density, _, _ := neo4j.GetRecordValue[float64](record, "density")
		internalEdgeCount, _, _ := neo4j.GetRecordValue[int64](record, "internal_edge_count")
		bprAvg, _, _ := neo4j.GetRecordValue[float64](record, "bpr_avg")
		sbCountVal, _, _ := neo4j.GetRecordValue[int64](record, "shared_boundary_count")
		hubCountVal, _, _ := neo4j.GetRecordValue[int64](record, "cross_cutting_hub_count")

		var dominantDomains []string
		if rawDomains, _, err := neo4j.GetRecordValue[[]any](record, "dominant_domains"); err == nil {
			for _, d := range rawDomains {
				if str, ok := d.(string); ok && str != "" {
					dominantDomains = append(dominantDomains, str)
				}
			}
		}

		results = append(results, &StructuralCommunityResult{
			ID:                   id,
			Name:                 name,
			Gamma:                gamma,
			Size:                 sizeVal,
			Density:              density,
			InternalEdgeCount:    internalEdgeCount,
			BPRAvg:               bprAvg,
			SharedBoundaryCount:  int(sbCountVal),
			CrossCuttingHubCount: int(hubCountVal),
			DominantDomains:      dominantDomains,
		})
	}

	return results, nil
}
