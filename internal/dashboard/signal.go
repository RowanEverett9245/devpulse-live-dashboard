package dashboard

import (
	"errors"
	"strings"
)

type Signal struct {
	EventID    string `json:"event_id"`
	Kind       string `json:"kind"`
	Service    string `json:"service"`
	Outcome    string `json:"outcome"`
	DurationMS int64  `json:"duration_ms"`
	Severity   string `json:"severity"`
	Diagnostic string `json:"diagnostic"`
}

type Point struct {
	Name  string            `json:"name"`
	Value float64           `json:"value"`
	Type  string            `json:"type"`
	Tags  map[string]string `json:"tags,omitempty"`
}

type Snapshot struct {
	EventID    string  `json:"event_id"`
	Kind       string  `json:"kind"`
	Service    string  `json:"service"`
	Outcome    string  `json:"outcome"`
	Status     string  `json:"status"`
	Diagnostic string  `json:"diagnostic,omitempty"`
	Points     []Point `json:"-"`
}

func Decide(signal Signal) (Snapshot, error) {
	if signal.EventID == "" || signal.Service == "" {
		return Snapshot{}, errors.New("event_id and service are required")
	}
	if signal.Kind != "build" && signal.Kind != "release" && signal.Kind != "diagnostic" {
		return Snapshot{}, errors.New("kind must be build, release, or diagnostic")
	}

	tags := map[string]string{"service": signal.Service, "outcome": signal.Outcome}
	points := []Point{{Name: "developer." + signal.Kind + ".completed", Value: 1, Type: "counter", Tags: tags}}
	if signal.DurationMS > 0 && signal.Kind != "diagnostic" {
		points = append(points, Point{Name: "developer." + signal.Kind + ".duration_ms", Value: float64(signal.DurationMS), Type: "timing", Tags: tags})
	}

	status := "healthy"
	if signal.Outcome != "passed" && signal.Outcome != "succeeded" || signal.Severity == "error" {
		status = "attention"
	}

	return Snapshot{
		EventID:    signal.EventID,
		Kind:       signal.Kind,
		Service:    signal.Service,
		Outcome:    signal.Outcome,
		Status:     status,
		Diagnostic: redact(signal.Diagnostic),
		Points:     points,
	}, nil
}

func redact(message string) string {
	words := strings.Fields(message)
	for i, word := range words {
		lower := strings.ToLower(word)
		if strings.HasPrefix(lower, "token=") || strings.HasPrefix(lower, "password=") {
			words[i] = word[:strings.IndexByte(word, '=')+1] + "[redacted]"
		}
	}
	return strings.Join(words, " ")
}
