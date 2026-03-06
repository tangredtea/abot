package vectordb

import (
	"math"
	"strings"
)

// BM25Scorer implements BM25 keyword scoring algorithm.
type BM25Scorer struct {
	k1        float64
	b         float64
	docCount  int
	avgDocLen float64
	idf       map[string]float64
}

// NewBM25Scorer creates a new BM25 scorer with default parameters.
func NewBM25Scorer() *BM25Scorer {
	return &BM25Scorer{
		k1:  1.2,
		b:   0.75,
		idf: make(map[string]float64),
	}
}

// Score calculates BM25 score for a query against a document.
func (s *BM25Scorer) Score(query, doc string) float64 {
	queryTerms := s.tokenize(query)
	docTerms := s.tokenize(doc)

	if len(queryTerms) == 0 || len(docTerms) == 0 {
		return 0
	}

	tf := make(map[string]int)
	for _, t := range docTerms {
		tf[t]++
	}

	docLen := float64(len(docTerms))
	if s.avgDocLen == 0 {
		s.avgDocLen = docLen
	}

	score := 0.0
	for _, qt := range queryTerms {
		if freq, ok := tf[qt]; ok {
			idf := s.getIDF(qt)
			numerator := float64(freq) * (s.k1 + 1)
			denominator := float64(freq) + s.k1*(1-s.b+s.b*docLen/s.avgDocLen)
			score += idf * (numerator / denominator)
		}
	}

	return score
}

// tokenize splits text into lowercase terms.
func (s *BM25Scorer) tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.Fields(text)
}

// getIDF returns the IDF score for a term.
func (s *BM25Scorer) getIDF(term string) float64 {
	if idf, ok := s.idf[term]; ok {
		return idf
	}
	// Default IDF for unknown terms
	return 1.0
}

// UpdateStats updates document statistics for better BM25 scoring.
func (s *BM25Scorer) UpdateStats(docCount int, avgDocLen float64, termDocFreq map[string]int) {
	s.docCount = docCount
	s.avgDocLen = avgDocLen

	// Calculate IDF for each term
	for term, df := range termDocFreq {
		s.idf[term] = math.Log((float64(docCount)-float64(df)+0.5)/(float64(df)+0.5) + 1.0)
	}
}
