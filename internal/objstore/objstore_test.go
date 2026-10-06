package objstore_test

import (
	"errors"
	"testing"

	"github.com/mtk14n/obsrv/internal/objstore"
)

func TestValidateKey(t *testing.T) {
	valid := []string{
		"a",
		"v1/logs/date=2026-10-06/hour=13/01J9Z.parquet",
		"metric_series/x-y_z.parquet",
	}
	for _, key := range valid {
		if err := objstore.ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) = %v, want nil", key, err)
		}
	}

	invalid := []string{
		"",
		"/abs",
		"trailing/",
		"a//b",
		"../escape",
		"a/../b",
		"a/./b",
		".hidden",
		"dir/.hidden",
		`back\slash`,
		"nul\x00byte",
	}
	for _, key := range invalid {
		if err := objstore.ValidateKey(key); !errors.Is(err, objstore.ErrInvalidKey) {
			t.Errorf("ValidateKey(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
}
