package dashboard

import "testing"

func TestDecide(t *testing.T) {
	tests := []struct {
		name       string
		input      Signal
		wantStatus string
		wantPoints int
		wantText   string
	}{
		{
			name:       "successful build includes duration",
			input:      Signal{EventID: "build-1842", Kind: "build", Service: "compiler", Outcome: "passed", DurationMS: 43820},
			wantStatus: "healthy",
			wantPoints: 2,
		},
		{
			name:       "error diagnostic asks for attention and redacts credential",
			input:      Signal{EventID: "diag-203", Kind: "diagnostic", Service: "release-cli", Outcome: "failed", Severity: "error", Diagnostic: "upload failed token=secret-value"},
			wantStatus: "attention",
			wantPoints: 1,
			wantText:   "upload failed token=[redacted]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decide(tt.input)
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if got.Status != tt.wantStatus || len(got.Points) != tt.wantPoints || got.Diagnostic != tt.wantText {
				t.Fatalf("Decide() = status %q, %d points, diagnostic %q", got.Status, len(got.Points), got.Diagnostic)
			}
		})
	}
}
