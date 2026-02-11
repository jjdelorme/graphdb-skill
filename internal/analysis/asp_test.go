package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case uint32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return -1
	}
}

func TestAspParser_Parse_CSharp(t *testing.T) {
	// Construct path to the fixture
	cwd, err := os.Getwd()
	require.NoError(t, err)
	
	// We are running from internal/analysis, so we need to go up to find test/fixtures
	// But usually tests run from the package directory.
	// Let's assume the standard go test behavior.
	projectRoot := filepath.Join(cwd, "../..")
	fixturePath := filepath.Join(projectRoot, "test/fixtures/asp/sample.aspx")

	content, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	parser := &AspParser{}
	nodes, _, err := parser.Parse(fixturePath, content)
	require.NoError(t, err)

	// Verify we found the nodes
	// "MyMethod" and "Calculate" should be present
	
	foundMyMethod := false
	foundCalculate := false

	for _, node := range nodes {
		if node.Properties["name"] == "MyMethod" {
			foundMyMethod = true
			// Check line number - should be 15 in the file
			line := toInt(node.Properties["line"])
			assert.Equal(t, 15, line, "MyMethod should be on line 15")
		}
		if node.Properties["name"] == "Calculate" {
			foundCalculate = true
			// Check line number - should be 20
			line := toInt(node.Properties["line"])
			assert.Equal(t, 20, line, "Calculate should be on line 20")
		}
	}

	assert.True(t, foundMyMethod, "Should have found MyMethod")
	assert.True(t, foundCalculate, "Should have found Calculate")
}

func TestAspParser_Parse_VB(t *testing.T) {
	// Construct path to the fixture
	cwd, err := os.Getwd()
	require.NoError(t, err)
	
	projectRoot := filepath.Join(cwd, "../..")
	fixturePath := filepath.Join(projectRoot, "test/fixtures/asp/sample.asp")

	content, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	parser := &AspParser{}
	nodes, _, err := parser.Parse(fixturePath, content)
	require.NoError(t, err)

	// Verify we found the nodes
	// "MySub" and "Add" should be present
	
	foundMySub := false
	foundAdd := false

	for _, node := range nodes {
		if node.Properties["name"] == "MySub" {
			foundMySub = true
			// Check line number - should be 9
			// 1: <%@ Language="VBScript" %>
			// ...
			// 8: <script runat="server">
			// 9:     Sub MySub()
			line := toInt(node.Properties["line"])
			assert.Equal(t, 9, line, "MySub should be on line 9")
		}
		if node.Properties["name"] == "Add" {
			foundAdd = true
			// Check line number - should be 13
			line := toInt(node.Properties["line"])
			assert.Equal(t, 13, line, "Add should be on line 13")
		}
	}

	assert.True(t, foundMySub, "Should have found MySub")
	assert.True(t, foundAdd, "Should have found Add")
}
