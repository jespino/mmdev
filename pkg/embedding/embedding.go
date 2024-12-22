package embedding

import (
	"math"
	"strings"
	"unicode"
)

const VectorSize = 256

// Vocabulary stores word frequencies across all documents
type Vocabulary struct {
	words     map[string]int // word -> document frequency
	docCount  int
	wordList  []string
}

func NewVocabulary() *Vocabulary {
	return &Vocabulary{
		words: make(map[string]int),
	}
}

func (v *Vocabulary) AddDocument(text string) {
	// Count unique words in this document
	seenWords := make(map[string]bool)
	for _, word := range tokenize(text) {
		if !seenWords[word] {
			v.words[word]++
			seenWords[word] = true
		}
	}
	v.docCount++
}

func (v *Vocabulary) Finalize() {
	// Create sorted word list for consistent vector positions
	v.wordList = make([]string, 0, len(v.words))
	for word := range v.words {
		v.wordList = append(v.wordList, word)
	}
}

func (v *Vocabulary) CreateVector(text string) []float32 {
	// Count words in this document
	wordFreq := make(map[string]int)
	totalWords := 0
	for _, word := range tokenize(text) {
		wordFreq[word]++
		totalWords++
	}

	// Create TF-IDF vector
	vector := make([]float32, VectorSize)
	for i, word := range v.wordList[:min(len(v.wordList), VectorSize)] {
		tf := float64(wordFreq[word]) / float64(totalWords)
		idf := math.Log(float64(v.docCount) / float64(v.words[word]))
		vector[i] = float32(tf * idf)
	}

	// Normalize vector
	magnitude := float32(0)
	for _, val := range vector {
		magnitude += val * val
	}
	magnitude = float32(math.Sqrt(float64(magnitude)))
	if magnitude > 0 {
		for i := range vector {
			vector[i] /= magnitude
		}
	}

	return vector
}

func tokenize(text string) []string {
	var tokens []string
	var currentToken strings.Builder

	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			currentToken.WriteRune(r)
		} else if currentToken.Len() > 0 {
			tokens = append(tokens, currentToken.String())
			currentToken.Reset()
		}
	}
	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
