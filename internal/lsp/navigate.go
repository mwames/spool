package lsp

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mwames/Spool/internal/index"
	"github.com/mwames/Spool/internal/spoolid"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

var reqIDPattern = regexp.MustCompile(`id:\s+(\S+)`)

// commentPrefixes mirrors the scanner's supported comment styles.
var commentPrefixes = []string{"//", "#"}

// resolve returns definition locations for the given file and 0-based line.
func resolve(idx *index.Index, filePath string, line int) []protocol.Location {
	if strings.HasSuffix(filePath, ".req") {
		return resolveFromReq(idx, filePath, line)
	}
	return resolveFromTest(idx, filePath, line)
}

// resolveFromTest handles go-to-definition from a test annotation to the AC in the .req file.
func resolveFromTest(idx *index.Index, filePath string, line int) []protocol.Location {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	if line >= len(lines) {
		return nil
	}

	id := extractAnnotationID(lines[line])
	if id == "" {
		return nil
	}

	if iv := idx.Invariant(id); iv != nil {
		return []protocol.Location{
			{
				URI: uri.File(iv.SourceFile),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(iv.Line - 1)},
					End:   protocol.Position{Line: uint32(iv.Line - 1)},
				},
			},
		}
	}

	ac := idx.AC(id)
	if ac == nil {
		return nil
	}

	// AC.Line is 1-based from the parser; LSP positions are 0-based.
	return []protocol.Location{
		{
			URI: uri.File(ac.SourceFile),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(ac.Line - 1)},
				End:   protocol.Position{Line: uint32(ac.Line - 1)},
			},
		},
	}
}

// resolveFromReq handles go-to-definition from an AC id in a .req file to covering tests.
func resolveFromReq(idx *index.Index, filePath string, line int) []protocol.Location {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	if line >= len(lines) {
		return nil
	}

	id := extractReqID(lines[line])
	if id == "" {
		return nil
	}

	ac := idx.AC(id)
	if ac != nil && len(ac.Tests) > 0 {
		var locs []protocol.Location
		for _, t := range ac.Tests {
			locs = append(locs, protocol.Location{
				URI: uri.File(t.File),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(t.Function.Line - 1)},
					End:   protocol.Position{Line: uint32(t.Function.Line - 1)},
				},
			})
		}
		return locs
	}

	if iv := idx.Invariant(id); iv != nil {
		var locs []protocol.Location
		for _, t := range iv.Tests {
			locs = append(locs, testLocation(t))
		}
		return locs
	}

	// Also try as a requirement ID — navigate to its ACs' tests, then its invariants' tests.
	req := idx.Requirement(id)
	if req != nil {
		var locs []protocol.Location
		for _, iac := range req.ACs {
			for _, t := range iac.Tests {
				locs = append(locs, testLocation(t))
			}
		}
		for _, iv := range req.Invariants {
			for _, t := range iv.Tests {
				locs = append(locs, testLocation(t))
			}
		}
		if len(locs) > 0 {
			return locs
		}
	}

	return nil
}

// testLocation returns the location of a linked test function.
func testLocation(t index.LinkedTest) protocol.Location {
	return protocol.Location{
		URI: uri.File(t.File),
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(t.Function.Line - 1)},
			End:   protocol.Position{Line: uint32(t.Function.Line - 1)},
		},
	}
}

// collectLinks returns document links for all Spool IDs in the given file.
func collectLinks(idx *index.Index, filePath string) []protocol.DocumentLink {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")

	if strings.HasSuffix(filePath, ".req") {
		return collectReqLinks(idx, lines)
	}
	return collectTestLinks(idx, lines)
}

// collectReqLinks returns links for Spool IDs in a .req file, pointing to covering tests.
func collectReqLinks(idx *index.Index, lines []string) []protocol.DocumentLink {
	var links []protocol.DocumentLink
	for i, line := range lines {
		loc := reqIDPattern.FindStringIndex(line)
		if loc == nil {
			continue
		}
		m := reqIDPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		candidate := m[1]
		if !spoolid.IsValid(candidate) {
			continue
		}
		normalized, _ := spoolid.NormalizeID(candidate)

		// Find the column range of the ID value within the line.
		idStart := strings.Index(line[loc[0]:], candidate) + loc[0]
		idEnd := idStart + len(candidate)

		var target string
		var tooltip string

		var first *index.LinkedTest
		if ac := idx.AC(normalized); ac != nil {
			if len(ac.Tests) > 0 {
				first = &ac.Tests[0]
			}
		} else if iv := idx.Invariant(normalized); iv != nil {
			if len(iv.Tests) > 0 {
				first = &iv.Tests[0]
			}
		} else if req := idx.Requirement(normalized); req != nil {
			for _, iac := range req.ACs {
				if len(iac.Tests) > 0 {
					first = &iac.Tests[0]
					break
				}
			}
			if first == nil {
				for _, iv := range req.Invariants {
					if len(iv.Tests) > 0 {
						first = &iv.Tests[0]
						break
					}
				}
			}
		}
		if first != nil {
			target = string(uri.File(first.File)) + "#" + fmt.Sprintf("%d", first.Function.Line)
			tooltip = "Go to test: " + first.Function.Name
		}

		if target == "" {
			continue
		}

		links = append(links, protocol.DocumentLink{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(i), Character: uint32(idStart)},
				End:   protocol.Position{Line: uint32(i), Character: uint32(idEnd)},
			},
			Target:  protocol.DocumentURI(target),
			Tooltip: tooltip,
		})
	}
	return links
}

// collectTestLinks returns links for Spool ID annotations in a test file, pointing to AC definitions.
func collectTestLinks(idx *index.Index, lines []string) []protocol.DocumentLink {
	var links []protocol.DocumentLink
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		var body string
		var prefixEnd int
		found := false
		for _, prefix := range commentPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				body = strings.TrimSpace(trimmed[len(prefix):])
				// Find where the ID starts in the original line.
				prefixEnd = strings.Index(line, prefix) + len(prefix)
				found = true
				break
			}
		}
		if !found {
			continue
		}

		candidate := body
		if idx := strings.Index(body, ":"); idx > 0 {
			candidate = strings.TrimSpace(body[:idx])
		}
		if !spoolid.IsValid(candidate) {
			continue
		}
		normalized, _ := spoolid.NormalizeID(candidate)

		var defFile, title string
		if ac := idx.AC(normalized); ac != nil {
			defFile, title = ac.SourceFile, ac.Title
		} else if iv := idx.Invariant(normalized); iv != nil {
			defFile, title = iv.SourceFile, iv.Title
		} else {
			continue
		}

		// Find the column range of the ID in the original line.
		idStart := prefixEnd + strings.Index(line[prefixEnd:], candidate)
		idEnd := idStart + len(candidate)

		links = append(links, protocol.DocumentLink{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(i), Character: uint32(idStart)},
				End:   protocol.Position{Line: uint32(i), Character: uint32(idEnd)},
			},
			Target:  uri.File(defFile),
			Tooltip: title,
		})
	}
	return links
}

// extractAnnotationID extracts a normalized Spool ID from a test file comment line.
func extractAnnotationID(line string) string {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range commentPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			body := strings.TrimSpace(trimmed[len(prefix):])
			candidate := body
			if idx := strings.Index(body, ":"); idx > 0 {
				candidate = strings.TrimSpace(body[:idx])
			}
			if spoolid.IsValid(candidate) {
				normalized, _ := spoolid.NormalizeID(candidate)
				return normalized
			}
			return ""
		}
	}
	return ""
}

// extractReqID extracts a normalized Spool ID from a .req file line like "id: CONFIG-1-1".
func extractReqID(line string) string {
	m := reqIDPattern.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	candidate := m[1]
	if !spoolid.IsValid(candidate) {
		return ""
	}
	normalized, _ := spoolid.NormalizeID(candidate)
	return normalized
}
