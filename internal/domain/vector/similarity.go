package vector

import "math"

// CosineSimilarity keeps behavior aligned with Java RagServiceImpl:
// return -1 for invalid vectors or zero norms.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return -1
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return -1
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

