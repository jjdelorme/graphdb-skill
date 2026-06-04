package query

import (
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// SeedVolatility applies heuristic rules to seed initial is_volatile flags.
func (p *Neo4jProvider) SeedVolatility(modulePattern string, rules []ContaminationRule) error {
	// Cleanup legacy flags first
	cleanupQuery := `
		// Cleanup legacy contamination flags
		MATCH (f:Function)
		REMOVE f.ui_contaminated, f.db_contaminated, f.io_contaminated, f.is_volatile, f.volatility_score
	`
	_, err := p.executeQuery(cleanupQuery, nil)
	if err != nil {
		return fmt.Errorf("failed to cleanup legacy flags: %w", err)
	}

	for _, rule := range rules {
		var query string
		params := map[string]any{
			"pattern":      modulePattern,
			"rule_pattern": rule.Pattern,
		}

		if rule.Type == "file" {
			// Seed based on file path
			query = `
				// Seed volatility by file path
				MATCH (f:Function)-[:DEFINED_IN]->(file:File)
				WHERE file.file =~ $pattern AND file.file =~ $rule_pattern
				SET f.is_volatile = true
				RETURN count(f) as count
			`
		} else if rule.Type == "function" {
			if rule.Heuristic == "path" {
				// Seed based on function name
				query = `
					// Seed volatility by function path
					MATCH (f:Function)-[:DEFINED_IN]->(file:File)
					WHERE file.file =~ $pattern AND f.name =~ $rule_pattern
					SET f.is_volatile = true
					RETURN count(f) as count
				`
			} else if rule.Heuristic == "content" {
				// Seed based on function content/body (if available)
				query = `
					// Seed volatility by function content
					MATCH (f:Function)-[:DEFINED_IN]->(file:File)
					WHERE file.file =~ $pattern AND f.content =~ $rule_pattern
					SET f.is_volatile = true
					RETURN count(f) as count
				`
			}
		}

		if query != "" {
			_, err := p.executeQuery(query, params)
			if err != nil {
				return fmt.Errorf("failed to apply volatility rule (%s): %w", rule.Layer, err)
			}
		}
	}

	return nil
}

// CountVolatileFunctions returns the number of functions flagged as volatile.
func (p *Neo4jProvider) CountVolatileFunctions() (int64, error) {
	query := `
		// Count Volatile Functions
		MATCH (f:Function {is_volatile: true}) RETURN count(f) as count
	`
	res, err := p.executeQuery(query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count volatile functions: %w", err)
	}
	if len(res.Records) == 0 {
		return 0, nil
	}
	count, _, _ := neo4j.GetRecordValue[int64](res.Records[0], "count")
	return count, nil
}

// HasVolatilityData returns true if at least one Function node has a non-null is_volatile property.
func (p *Neo4jProvider) HasVolatilityData() (bool, error) {
	query := `
		// Check if any is_volatile flags exist
		MATCH (f:Function) WHERE f.is_volatile IS NOT NULL RETURN true as exists LIMIT 1
	`
	res, err := p.executeQuery(query, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check volatility data presence: %w", err)
	}
	return len(res.Records) > 0, nil
}

// PropagateVolatility walks the CALLS graph UPWARD to propagate volatility.
// MATCH (caller)-[:CALLS]->(callee {is_volatile: true}) SET caller.is_volatile = true
func (p *Neo4jProvider) PropagateVolatility() error {
	// Cypher query to propagate one level at a time UPWARD.
	// We run this until no more nodes are updated.
	query := `
		// Propagate Volatility Upward
		MATCH (caller:Function)-[:CALLS]->(callee:Function {is_volatile: true})
		WHERE caller.is_volatile IS NULL OR caller.is_volatile = false
		WITH caller LIMIT 5000
		SET caller.is_volatile = true
		RETURN count(caller) as count
	`

	for {
		res, err := p.executeQuery(query, nil)
		if err != nil {
			return fmt.Errorf("failed to propagate volatility: %w", err)
		}

		if len(res.Records) == 0 {
			break
		}

		count, _, _ := neo4j.GetRecordValue[int64](res.Records[0], "count")
		if count == 0 {
			break
		}
		// Continue propagating until no more nodes are updated
	}

	return nil
}

// CalculateRiskScores calculates risk scores for functions based on fan-in, fan-out, volatility, and file churn.
// Formula: risk_score = normalize(fan_in * 0.4 + fan_out * 0.1 + volatility_score * 3.0 + churn * 0.4)
func (p *Neo4jProvider) CalculateRiskScores() error {
	// 1. Calculate volatility scores (degree of contamination)
	// Seeded/Propagated nodes have is_volatile=true.
	// We use the distance to the nearest volatile node.
	volatilityQuery := `
		// Calculate Volatility Scores
		MATCH (f:Function)
		OPTIONAL MATCH p = (f)-[:CALLS*0..2]->(v:Function {is_volatile: true})
		WITH f, min(length(p)) as distance
		SET f.volatility_score = CASE WHEN distance IS NOT NULL THEN 1.0 / (distance + 1.0) ELSE 0.0 END
	`
	_, err := p.executeQuery(volatilityQuery, nil)
	if err != nil {
		return fmt.Errorf("failed to calculate volatility scores: %w", err)
	}

	// 2. Calculate raw scores and store them temporarily
	query := `
		// Calculate Raw Risk Scores
		MATCH (f:Function)
		OPTIONAL MATCH (f)-[:DEFINED_IN]->(file:File)
		WITH f, coalesce(file.change_frequency, 0) as churn
		OPTIONAL MATCH (f)<-[:CALLS]-(caller)
		WITH f, churn, count(caller) as fan_in
		OPTIONAL MATCH (f)-[:CALLS]->(callee)
		WITH f, churn, fan_in, count(callee) as fan_out
		WITH f, churn, fan_in, fan_out,
		     coalesce(f.volatility_score, 0.0) as vol_score
		SET f.raw_risk_score = (fan_in * 0.4 + fan_out * 0.1 + vol_score * 3.0 + churn * 0.4)
		RETURN max(f.raw_risk_score) as max_score
	`
	res, err := p.executeQuery(query, nil)
	if err != nil {
		return fmt.Errorf("failed to calculate raw risk scores: %w", err)
	}

	if len(res.Records) == 0 {
		return nil
	}

	maxScore, _, _ := neo4j.GetRecordValue[float64](res.Records[0], "max_score")
	if maxScore == 0 {
		maxScore = 1.0
	}

	// 2. Normalize scores to [0.0, 1.0]
	normalizeQuery := `
		// Normalize Risk Scores
		MATCH (f:Function)
		WHERE f.raw_risk_score IS NOT NULL
		SET f.risk_score = f.raw_risk_score / $max_score
		REMOVE f.raw_risk_score
	`
	_, err = p.executeQuery(normalizeQuery, map[string]any{"max_score": maxScore})
	if err != nil {
		return fmt.Errorf("failed to normalize risk scores: %w", err)
	}

	return nil
}
