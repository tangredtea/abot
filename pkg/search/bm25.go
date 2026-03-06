// Package search provides BM25 keyword search.
package search

import (
	"math"
	"strings"
)

// BM25Index implements BM25 keyword search algorithm.
type BM25Index struct {
	documents []Document
	idf       map[string]float64
	avgDocLen float64
	k1        float64 // Term frequency saturation (default 1.5)
	b         float64 // Length normalization (default 0.75)
}

// Document represents an indexed document.
type Document struct {
	ID      string
	Content string
	tokens  []string
	termFreq map[string]int
}

// NewBM25Index creates a BM25 index.
func NewBM25Index() *BM25Index {
	return &BM25Index{
		documents: make([]Document, 0),
		idf:       make(map[string]float64),
		k1:        1.5,
		b:         0.75,
	}
}

// AddDocument adds a document to the index.
func (b *BM25Index) AddDocument(id, content string) {
	tokens := tokenize(content)
	termFreq := calculateTermFreq(tokens)

	doc := Document{
		ID:       id,
		Content:  content,
		tokens:   tokens,
		termFreq: termFreq,
	}

	b.documents = append(b.documents, doc)
	b.updateIDF()
}

// Search performs BM25 search.
func (b *BM25Index) Search(query string, limit int) []SearchResult {
	queryTokens := tokenize(query)

	results := make([]SearchResult, 0, len(b.documents))

	for _, doc := range b.documents {
		score := b.calculateBM25(queryTokens, doc)
		if score > 0 {
			results = append(results, SearchResult{
				ID:      doc.ID,
				Content: doc.Content,
				Score:   score,
			})
		}
	}

	// Sort by score descending
	sortByScore(results)

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// calculateBM25 calculates BM25 score for a document.
func (b *BM25Index) calculateBM25(queryTokens []string, doc Document) float64 {
	score := 0.0
	docLen := float64(len(doc.tokens))

	for _, token := range queryTokens {
		if freq, ok := doc.termFreq[token]; ok {
			idf := b.idf[token]

			// BM25 formula
			numerator := float64(freq) * (b.k1 + 1)
			denominator := float64(freq) + b.k1*(1-b.b+b.b*(docLen/b.avgDocLen))

			score += idf * (numerator / denominator)
		}
	}

	return score
}

// updateIDF updates inverse document frequency for all terms.
func (b *BM25Index) updateIDF() {
	N := float64(len(b.documents))
	docFreq := make(map[string]int)

	// Calculate document frequency
	for _, doc := range b.documents {
		seen := make(map[string]bool)
		for token := range doc.termFreq {
			if !seen[token] {
				docFreq[token]++
				seen[token] = true
			}
		}
	}

	// Calculate IDF
	for term, df := range docFreq {
		b.idf[term] = math.Log((N - float64(df) + 0.5) / (float64(df) + 0.5))
	}

	// Calculate average document length
	totalLen := 0
	for _, doc := range b.documents {
		totalLen += len(doc.tokens)
	}
	if len(b.documents) > 0 {
		b.avgDocLen = float64(totalLen) / float64(len(b.documents))
	}
}

// tokenize splits text into tokens.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	return tokens
}

// calculateTermFreq calculates term frequency map.
func calculateTermFreq(tokens []string) map[string]int {
	freq := make(map[string]int)
	for _, token := range tokens {
		freq[token]++
	}
	return freq
}

// sortByScore sorts results by score descending.
func sortByScore(results []SearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
