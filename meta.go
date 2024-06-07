package main

import "strings"

const fireproofPrefix = "fp"
const metaSeparator = "."

// MetaDataKey looks like this: fp.topics.0.18
type MetaDataKey string

// Valid returns in the string is valid meta data key
//   - must have at least 3 dots (one separates prefix from database, one separates database from version,
//     and the version contains one dot itself
func (m MetaDataKey) Valid() bool {
	sepCount := strings.Count(string(m), metaSeparator)
	return sepCount >= 3 && strings.HasPrefix(string(m), fireproofPrefix)
}

func (m MetaDataKey) Name() string {
	noPrefix := strings.TrimPrefix(string(m), fireproofPrefix+metaSeparator)
	lastDot := strings.LastIndex(noPrefix, metaSeparator)
	if lastDot > 0 {
		secondLastDot := strings.LastIndex(noPrefix[:lastDot], metaSeparator)
		if secondLastDot > 0 {
			return noPrefix[:secondLastDot]
		}
	}
	return ""
}

func (m MetaDataKey) Version() string {
	lastDot := strings.LastIndex(string(m), metaSeparator)
	if lastDot > 0 {
		secondLastDot := strings.LastIndex(string(m), metaSeparator)
		if secondLastDot > 0 {
			return string(m[secondLastDot-1:])
		}
	}
	return ""
}
