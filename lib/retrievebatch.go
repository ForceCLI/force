package lib

import (
	"encoding/xml"
	"strings"
)

// Salesforce rejects retrieve calls containing more than 10,000 files with
// LIMIT_EXCEEDED, so queries with more explicitly-listed members are split
// into multiple retrieve calls.
const maxFilesPerRetrieve = 10000

// splitQuery partitions query into batches whose explicitly-listed members
// (each counted as one file) do not exceed limit. Elements with more members
// than the limit are split across batches. Wildcard members expand
// server-side to an unknown number of files, so they count as one.
func splitQuery(query ForceMetadataQuery, limit int) (batches []ForceMetadataQuery) {
	var current ForceMetadataQuery
	count := 0
	for _, element := range query {
		for _, name := range element.Name {
			members := element.Members
			for {
				if count == limit {
					batches = append(batches, current)
					current = nil
					count = 0
				}
				take := min(limit-count, len(members))
				current = append(current, ForceMetadataQueryElement{Name: []string{name}, Members: members[:take]})
				count += take
				members = members[take:]
				if len(members) == 0 {
					break
				}
			}
		}
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return
}

func countQueryMembers(query ForceMetadataQuery) int {
	count := 0
	for _, element := range query {
		count += len(element.Name) * len(element.Members)
	}
	return count
}

// isRetrieveLimitError reports whether err is the server rejecting a
// retrieve for containing too many files. This can happen even when a
// query's explicitly-listed members are within the limit, because wildcard
// members expand server-side.
func isRetrieveLimitError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "LIMIT_EXCEEDED") &&
		strings.Contains(err.Error(), "Too many files in retrieve")
}

// mergePackageXml combines the package.xml manifests returned by multiple
// retrieve batches into one, concatenating the members of types that were
// split across batches.
func mergePackageXml(docs ...[]byte) ([]byte, error) {
	var merged Package
	index := make(map[string]int)
	for _, doc := range docs {
		var p Package
		if err := xml.Unmarshal(doc, &p); err != nil {
			return nil, err
		}
		if merged.Version == "" {
			merged.Version = p.Version
			merged.Xmlns = p.Xmlns
		}
		for _, t := range p.Types {
			if i, ok := index[t.Name]; ok {
				merged.Types[i].Members = append(merged.Types[i].Members, t.Members...)
			} else {
				index[t.Name] = len(merged.Types)
				merged.Types = append(merged.Types, t)
			}
		}
	}
	byteXml, err := xml.MarshalIndent(merged, "", "    ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), byteXml...), nil
}
