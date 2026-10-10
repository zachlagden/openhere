package api

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/docs"
	"github.com/zachlagden/selfhost/internal/domains"
)

var (
	specPathLine   = regexp.MustCompile(`^  (/\S*):\s*$`)
	specMethodLine = regexp.MustCompile(`^    (get|put|post|patch|delete|head|options):\s*$`)
	paramName      = regexp.MustCompile(`\{[^}]*\}`)
)

func TestEveryRegisteredRouteIsInTheOpenAPISpecAndTheOtherWayRound(t *testing.T) {
	registered := registeredOperations(t)
	documented := documentedOperations(t)

	for operation := range registered {
		if !documented[operation] {
			t.Errorf("route %s is registered but missing from docs/openapi.yaml", operation)
		}
	}
	for operation := range documented {
		if !registered[operation] {
			t.Errorf("operation %s is in docs/openapi.yaml but no route is registered for it", operation)
		}
	}
}

func registeredOperations(t *testing.T) map[string]bool {
	t.Helper()
	_, patterns := New(nil, nil, nil, nil, nil, nil, nil, Options{Domains: &domains.Service{}}).build()
	operations := map[string]bool{}
	for _, pattern := range patterns {
		method, route, found := strings.Cut(pattern, " ")
		if !found {
			continue
		}
		relative, ok := strings.CutPrefix(route, basePath)
		if !ok {
			t.Errorf("route %q is not under %s", pattern, basePath)
			continue
		}
		operations[operationKey(method, relative)] = true
		if method == "GET" {
			operations[operationKey("HEAD", relative)] = true
		}
	}
	if len(operations) == 0 {
		t.Fatal("no routes were registered")
	}
	return operations
}

func documentedOperations(t *testing.T) map[string]bool {
	t.Helper()
	operations := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(docs.OpenAPI))
	inPaths := false
	currentPath := ""
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, " ") && line != "" {
			inPaths = line == "paths:"
			currentPath = ""
			continue
		}
		if !inPaths {
			continue
		}
		if match := specPathLine.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
			continue
		}
		if match := specMethodLine.FindStringSubmatch(line); match != nil && currentPath != "" {
			method := strings.ToUpper(match[1])
			operations[operationKey(method, currentPath)] = true
			if method == "GET" {
				operations[operationKey("HEAD", currentPath)] = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read the spec: %v", err)
	}
	if len(operations) == 0 {
		t.Fatal("the spec lists no operations")
	}
	return operations
}

func operationKey(method, route string) string {
	route = strings.TrimSuffix(paramName.ReplaceAllString(route, "{}"), "/")
	return method + " " + route
}

func TestOperationKeysIgnoreParameterNames(t *testing.T) {
	keys := []string{
		operationKey("GET", "/invites/{token}"),
		operationKey("GET", "/invites/{invite}"),
		operationKey("PUT", "/publish/{slug}/versions/{versionId}/files/{path...}"),
	}
	if keys[0] != keys[1] {
		t.Errorf("parameter names changed the key: %q and %q", keys[0], keys[1])
	}
	if !strings.HasSuffix(keys[2], "/files/{}") {
		t.Errorf("remainder parameter was not normalised: %q", keys[2])
	}
}
