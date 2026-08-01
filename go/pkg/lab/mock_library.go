// SPDX-Licence-Identifier: EUPL-1.2

package lab

// MockModelLibrary returns a deterministic set of fixture .model
// rows that mirror the kind of artefacts a fresh install sees
// after the first few packs land. Same shape as the real walk-the-
// filesystem implementation will return; fingerprints are well-
// formed sha256 hex but synthetic — not derived from real packs.
type MockModelLibrary struct{}

// NewMockLibrary returns a MockModelLibrary. Default wiring when
// no real library is plugged into NewService.
//
// Usage example:
//
//	lib := lab.NewMockLibrary()
//	rows, _ := lib.ListModels()
//	// rows[0].Filename == "lemma-12b-v4-p6.model"
func NewMockLibrary() *MockModelLibrary {
	return &MockModelLibrary{}
}

// ListModels returns the fixture rows.
func (m *MockModelLibrary) ListModels() ([]ModelRow, error) {
	return []ModelRow{
		{
			Path:        "/Users/agent/Lethean/data/models/lemma-12b-v4-p6.model",
			Filename:    "lemma-12b-v4-p6.model",
			Arch:        "gemma-4",
			QuantBits:   4,
			Ctx:         131072,
			SizeBytes:   7_834_124_288,
			Fingerprint: "a3d9c812f4567b8e92d145a6c70be8b1d23f4a5b6c7d8e9f0a1b2c3d4e5f6a7b",
			Created:     "2026-04-12T18:34:21Z",
			Producer:    "go-mlx",
		},
		{
			Path:        "/Users/agent/Lethean/data/models/lemma-12b-v4-p6.train",
			Filename:    "lemma-12b-v4-p6.train",
			Arch:        "gemma-4",
			QuantBits:   0,
			Ctx:         131072,
			SizeBytes:   18_212_491_264,
			Fingerprint: "b4e2d934f4567b8e92d145a6c70be8b1d23f4a5b6c7d8e9f0a1b2c3d4e5f6a8c",
			Created:     "2026-04-12T14:08:11Z",
			Producer:    "go-mlx",
		},
		{
			Path:        "/Users/agent/Lethean/data/models/lemma-3-1b-cl-bpl.model",
			Filename:    "lemma-3-1b-cl-bpl.model",
			Arch:        "gemma-3",
			QuantBits:   8,
			Ctx:         8192,
			SizeBytes:   1_073_741_824,
			Fingerprint: "c5f3ea4505678c9fa3e256b7d81cf9c2e34a5b6c7d8e9f0a1b2c3d4e5f6a8b9d",
			Created:     "2026-03-28T09:21:44Z",
			Producer:    "go-mlx",
		},
		{
			Path:        "/Users/agent/Lethean/data/models/qwen-3-32b-think.model",
			Filename:    "qwen-3-32b-think.model",
			Arch:        "qwen-3",
			QuantBits:   4,
			Ctx:         32768,
			SizeBytes:   18_874_368_000,
			Fingerprint: "d6a4fb5616789dab4f367c8e92dafac3f45b6c7d8e9f0a1b2c3d4e5f6a7b8c9e",
			Created:     "2026-02-14T16:55:02Z",
			Producer:    "go-mlx",
		},
		{
			Path:        "/Users/agent/Lethean/data/models/lemma-12b-v4-p6.vindex",
			Filename:    "lemma-12b-v4-p6.vindex",
			Arch:        "gemma-4",
			QuantBits:   0,
			Ctx:         131072,
			SizeBytes:   16_777_216,
			Fingerprint: "e7b50c6727890ebc5048c8f9a3ebbcb4056c7d8e9f0a1b2c3d4e5f6a7b8c9daf",
			Created:     "2026-04-12T19:02:08Z",
			Producer:    "go-mlx",
		},
	}, nil
}
