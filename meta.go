package main

import "strings"

const fireproofPrefix = "fp"
const keySeparator = "."

// MetaDataKey looks like this: fp.topics.0.18
type MetaDataKey string

func MetaDataKeyFromDatabaseVersion(db, version string) MetaDataKey {
	return MetaDataKey(strings.Join([]string{fireproofPrefix, db, version}, keySeparator))
}

// Valid returns in the string is valid meta data key
//   - must have at least 3 dots (one separates prefix from database, one separates database from version,
//     and the version contains one dot itself
func (m MetaDataKey) Valid() bool {
	sepCount := strings.Count(string(m), keySeparator)
	return sepCount >= 3 && strings.HasPrefix(string(m), fireproofPrefix)
}

func (m MetaDataKey) Name() string {
	noPrefix := strings.TrimPrefix(string(m), fireproofPrefix+keySeparator)
	lastDot := strings.LastIndex(noPrefix, keySeparator)
	if lastDot > 0 {
		secondLastDot := strings.LastIndex(noPrefix[:lastDot], keySeparator)
		if secondLastDot > 0 {
			return noPrefix[:secondLastDot]
		}
	}
	return ""
}

func (m MetaDataKey) Version() string {
	lastDot := strings.LastIndex(string(m), keySeparator)
	if lastDot > 0 {
		secondLastDot := strings.LastIndex(string(m), keySeparator)
		if secondLastDot > 0 {
			return string(m[secondLastDot-1:])
		}
	}
	return ""
}

// DataKey looks like this: fp.topics
type DataKey string

func DataKeyFromDatabase(db string) MetaDataKey {
	return MetaDataKey(strings.Join([]string{fireproofPrefix, db}, keySeparator))
}

// Valid returns in the string is valid meta data key
//   - must have at least 1 dot (one separates prefix from database
func (m DataKey) Valid() bool {
	sepCount := strings.Count(string(m), keySeparator)
	return sepCount >= 1 && strings.HasPrefix(string(m), fireproofPrefix)
}

func (m DataKey) Name() string {
	noPrefix := strings.TrimPrefix(string(m), fireproofPrefix+keySeparator)
	return noPrefix
}
