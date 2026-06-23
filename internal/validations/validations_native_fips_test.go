package validations

import (
	"context"
	"debug/buildinfo"
	"errors"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/openshift/check-payload/internal/types"
)

func mustVersion(v string) *semver.Version {
	sv, err := semver.NewVersion(v)
	if err != nil {
		panic(err)
	}
	return sv
}

func makeBaton(goVer string, settings ...debug.BuildSetting) *Baton {
	return &Baton{
		GoVersion:   mustVersion(goVer),
		GoBuildInfo: &debug.BuildInfo{Settings: settings},
	}
}

func TestHasGodebugFIPS140Enabled(t *testing.T) {
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     bool
	}{
		{"no settings", nil, false},
		{"DefaultGODEBUG without fips140", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "http2debug=1"}}, false},
		{"DefaultGODEBUG empty value", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: ""}}, false},

		// auto
		{"DefaultGODEBUG fips140=auto alone", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=auto"}}, true},
		{"DefaultGODEBUG fips140=auto comma-separated", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "http2debug=1,fips140=auto,other=0"}}, true},
		{"DefaultGODEBUG fips140=auto trailing comma", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=auto,"}}, true},
		{"DefaultGODEBUG fips140=auto with spaces", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=auto , other=1"}}, true},
		{"GODEBUG key fips140=auto", []debug.BuildSetting{{Key: "GODEBUG", Value: "fips140=auto"}}, true},

		// on (RH Go 1.26 toolchain default when GOFIPS140 is set)
		{"DefaultGODEBUG fips140=on alone", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=on"}}, true},
		{"DefaultGODEBUG fips140=on comma-separated", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "http2debug=1,fips140=on"}}, true},
		{"GODEBUG key fips140=on", []debug.BuildSetting{{Key: "GODEBUG", Value: "fips140=on"}}, true},

		// only
		{"DefaultGODEBUG fips140=only alone", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=only"}}, true},
		{"DefaultGODEBUG fips140=only comma-separated", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "tlssha1=1,fips140=only"}}, true},
		{"GODEBUG key fips140=only", []debug.BuildSetting{{Key: "GODEBUG", Value: "fips140=only"}}, true},

		// non-activating values
		{"DefaultGODEBUG fips140=off", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=off"}}, false},
		{"DefaultGODEBUG fips140=debug", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "fips140=debug"}}, false},

		// substring must not match
		{"substring nofips140=auto does not match", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "nofips140=auto"}}, false},
		{"substring nofips140=on does not match", []debug.BuildSetting{{Key: "DefaultGODEBUG", Value: "nofips140=on"}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baton := &Baton{GoBuildInfo: &debug.BuildInfo{Settings: tc.settings}}
			if got := hasGodebugFIPS140Enabled(baton); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasGOFIPS140Certified(t *testing.T) {
	tests := []struct {
		name       string
		settings   []debug.BuildSetting
		minVersion string
		want       bool
	}{
		{"no GOFIPS140", nil, "", false},
		{"GOFIPS140=off", []debug.BuildSetting{{Key: "GOFIPS140", Value: "off"}}, "", false},
		{"GOFIPS140 empty", []debug.BuildSetting{{Key: "GOFIPS140", Value: ""}}, "", false},
		{"GOFIPS140=latest", []debug.BuildSetting{{Key: "GOFIPS140", Value: "latest"}}, "", true},
		{"GOFIPS140=v1.0.0", []debug.BuildSetting{{Key: "GOFIPS140", Value: "v1.0.0"}}, "", true},
		{"GOFIPS140=v1.0.0-c2097c7c", []debug.BuildSetting{{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"}}, "", true},
		{"GOFIPS140=certified", []debug.BuildSetting{{Key: "GOFIPS140", Value: "certified"}}, "", true},
		{"GOFIPS140=inprocess", []debug.BuildSetting{{Key: "GOFIPS140", Value: "inprocess"}}, "", true},
		{"version >= min", []debug.BuildSetting{{Key: "GOFIPS140", Value: "v1.0.0"}}, "v1.0.0", true},
		{"version < min", []debug.BuildSetting{{Key: "GOFIPS140", Value: "v0.9.0"}}, "v1.0.0", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := goFIPSMinVersion
			goFIPSMinVersion = tc.minVersion
			defer func() { goFIPSMinVersion = old }()
			baton := &Baton{GoBuildInfo: &debug.BuildInfo{Settings: tc.settings}}
			if got := hasGOFIPS140Certified(baton); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestValidateGoNativeFIPS validates the three-tier version-gated FIPS rules:
//
//	Go <= 1.25: No native FIPS support. Skip, fall through to legacy CGO/OpenSSL checks.
//	Go == 1.26: Dual mode. fips140={auto,on,only} present → enforce native FIPS.
//	            fips140 absent → fall through to legacy checks.
//	Go >= 1.27: OpenSSL backend removed. fips140={auto,on,only} AND GOFIPS140 both required.
func TestValidateGoNativeFIPS(t *testing.T) {
	ctx := context.Background()

	nativeFIPSAuto := []debug.BuildSetting{
		{Key: "DefaultGODEBUG", Value: "fips140=auto,tlssha1=1"},
		{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
	}
	nativeFIPSOn := []debug.BuildSetting{
		{Key: "DefaultGODEBUG", Value: "fips140=on"},
		{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
	}
	nativeFIPSOnly := []debug.BuildSetting{
		{Key: "DefaultGODEBUG", Value: "fips140=only"},
		{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
	}
	// Reproduces the reporter's scenario: RH Go 1.26 toolchain auto-injects
	// fips140=on with CGO_ENABLED=0 and no_openssl tag.
	reporterScenario := []debug.BuildSetting{
		{Key: "DefaultGODEBUG", Value: "fips140=on"},
		{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
		{Key: "CGO_ENABLED", Value: "0"},
		{Key: "-tags", Value: "no_openssl,fips140v1.0"},
	}
	autoOnly := []debug.BuildSetting{
		{Key: "DefaultGODEBUG", Value: "fips140=auto"},
	}
	onOnly := []debug.BuildSetting{
		{Key: "DefaultGODEBUG", Value: "fips140=on"},
	}
	certifiedOnly := []debug.BuildSetting{
		{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
	}

	tests := []struct {
		name       string
		goVer      string
		settings   []debug.BuildSetting
		noCrypto   bool
		wantErr    error
		wantNative bool
		wantModule string
	}{
		// ── Go <= 1.25: always fall through to legacy ──
		{
			name:  "1.25/skip: no native FIPS support",
			goVer: "1.25.0",
		},
		{
			name:     "1.25/skip: native settings present but ignored",
			goVer:    "1.25.0",
			settings: nativeFIPSAuto,
		},

		// ── Go 1.26: dual mode — fips140 GODEBUG selects native path ──
		{
			name:  "1.26/legacy: no fips140 GODEBUG → legacy CGO/OpenSSL path",
			goVer: "1.26.0",
		},
		{
			name:     "1.26/legacy: GOFIPS140 without GODEBUG → legacy path",
			goVer:    "1.26.0",
			settings: certifiedOnly,
		},
		{
			name:       "1.26/native: fips140=auto + GOFIPS140 → pass",
			goVer:      "1.26.0",
			settings:   nativeFIPSAuto,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:       "1.26/native: fips140=on + GOFIPS140 → pass (toolchain default)",
			goVer:      "1.26.0",
			settings:   nativeFIPSOn,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:       "1.26/native: fips140=only + GOFIPS140 → pass",
			goVer:      "1.26.0",
			settings:   nativeFIPSOnly,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:       "1.26/native: reporter scenario (CGO=0, no_openssl, fips140=on) → pass",
			goVer:      "1.26.0",
			settings:   reporterScenario,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:     "1.26/native: fips140=auto without GOFIPS140 → ErrGoFIPSNotCertified",
			goVer:    "1.26.0",
			settings: autoOnly,
			wantErr:  types.ErrGoFIPSNotCertified,
		},
		{
			name:     "1.26/native: fips140=on without GOFIPS140 → ErrGoFIPSNotCertified",
			goVer:    "1.26.0",
			settings: onOnly,
			wantErr:  types.ErrGoFIPSNotCertified,
		},

		// ── Go >= 1.27: OpenSSL backend removed, both settings required ──
		{
			name:       "1.27/native: fips140=auto + GOFIPS140 → pass",
			goVer:      "1.27.0",
			settings:   nativeFIPSAuto,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:       "1.27/native: fips140=on + GOFIPS140 → pass",
			goVer:      "1.27.0",
			settings:   nativeFIPSOn,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:       "1.27/native: fips140=only + GOFIPS140 → pass",
			goVer:      "1.27.0",
			settings:   nativeFIPSOnly,
			wantNative: true,
			wantModule: "go",
		},
		{
			name:    "1.27/fail: no settings → ErrGoFIPSNotEnabled",
			goVer:   "1.27.0",
			wantErr: types.ErrGoFIPSNotEnabled,
		},
		{
			name:     "1.27/fail: GOFIPS140 without GODEBUG → ErrGoFIPSNotEnabled",
			goVer:    "1.27.0",
			settings: certifiedOnly,
			wantErr:  types.ErrGoFIPSNotEnabled,
		},
		{
			name:     "1.27/fail: fips140=auto without GOFIPS140 → ErrGoFIPSNotCertified",
			goVer:    "1.27.0",
			settings: autoOnly,
			wantErr:  types.ErrGoFIPSNotCertified,
		},
		{
			name:     "1.27/fail: fips140=on without GOFIPS140 → ErrGoFIPSNotCertified",
			goVer:    "1.27.0",
			settings: onOnly,
			wantErr:  types.ErrGoFIPSNotCertified,
		},
		{
			name:       "1.28/native: future version with both → pass",
			goVer:      "1.28.0",
			settings:   nativeFIPSAuto,
			wantNative: true,
			wantModule: "go",
		},

		// ── No crypto: always skip regardless of version ──
		{
			name:     "any/skip: no crypto imports",
			goVer:    "1.27.0",
			noCrypto: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baton := makeBaton(tc.goVer, tc.settings...)
			baton.GoNoCrypto = tc.noCrypto

			ve := validateGoNativeFIPS(ctx, "", baton)

			if tc.wantErr != nil {
				if ve == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErr)
				}
				if !errors.Is(ve.Error, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, ve.Error)
				}
			} else if ve != nil {
				t.Fatalf("expected nil, got %v", ve.Error)
			}

			if baton.GoNativeFIPS != tc.wantNative {
				t.Errorf("GoNativeFIPS = %v, want %v", baton.GoNativeFIPS, tc.wantNative)
			}

			if tc.wantModule != "" {
				found := false
				for _, m := range baton.ModulesUsed {
					if m == tc.wantModule {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ModulesUsed %v does not contain %q", baton.ModulesUsed, tc.wantModule)
				}
			}
		})
	}
}

func TestLegacyChecksSkippedForNativeFIPS(t *testing.T) {
	ctx := context.Background()

	// Use fips140=on (RH toolchain default) to verify legacy checks are skipped.
	baton := makeBaton(
		"1.26.0",
		debug.BuildSetting{Key: "DefaultGODEBUG", Value: "fips140=on"},
		debug.BuildSetting{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
		debug.BuildSetting{Key: "CGO_ENABLED", Value: "0"},
		debug.BuildSetting{Key: "-tags", Value: "no_openssl,fips140v1.0"},
	)
	// Simulate validateGoNativeFIPS having run.
	baton.GoNativeFIPS = true

	checks := []struct {
		name string
		fn   ValidationFn
	}{
		{"validateGoCgo", validateGoCgo},
		{"validateGoCGOInit", validateGoCGOInit},
		{"validateGoSymbols", validateGoSymbols},
		{"validateGoStatic", validateGoStatic},
		{"validateGoOpenssl", validateGoOpenssl},
		{"validateGoTagsAndExperiment", validateGoTagsAndExperiment},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if ve := tc.fn(ctx, "", baton); ve != nil {
				t.Errorf("expected nil (skip) for native FIPS binary, got %v", ve.Error)
			}
		})
	}
}

// TestEndToEndReporterScenario simulates the full validation chain for the
// exact build configuration from the issue reporter: Go 1.26, GOFIPS140=v1.0.0,
// CGO_ENABLED=0, -tags no_openssl. The RH toolchain auto-injects fips140=on.
func TestEndToEndReporterScenario(t *testing.T) {
	ctx := context.Background()

	baton := makeBaton(
		"1.26.3",
		debug.BuildSetting{Key: "DefaultGODEBUG", Value: "fips140=on"},
		debug.BuildSetting{Key: "GOFIPS140", Value: "v1.0.0-c2097c7c"},
		debug.BuildSetting{Key: "CGO_ENABLED", Value: "0"},
		debug.BuildSetting{Key: "-tags", Value: "no_openssl,fips140v1.0"},
	)

	ve := validateGoNativeFIPS(ctx, "", baton)
	if ve != nil {
		t.Fatalf("validateGoNativeFIPS: expected pass, got %v", ve.Error)
	}
	if !baton.GoNativeFIPS {
		t.Fatal("expected GoNativeFIPS=true")
	}

	// All legacy checks must be skipped now.
	legacyChecks := []struct {
		name string
		fn   ValidationFn
	}{
		{"validateGoCgo", validateGoCgo},
		{"validateGoSymbols", validateGoSymbols},
		{"validateGoStatic", validateGoStatic},
		{"validateGoOpenssl", validateGoOpenssl},
		{"validateGoTagsAndExperiment", validateGoTagsAndExperiment},
	}
	for _, tc := range legacyChecks {
		if ve := tc.fn(ctx, "", baton); ve != nil {
			t.Errorf("%s: expected skip for native FIPS, got %v", tc.name, ve.Error)
		}
	}

	found := false
	for _, m := range baton.ModulesUsed {
		if m == "go" {
			found = true
		}
	}
	if !found {
		t.Errorf("ModulesUsed %v should contain \"go\"", baton.ModulesUsed)
	}
}

func TestScanRealNativeFIPSBinary(t *testing.T) {
	const testBinary = "../../test/resources/mock_native_fips/usr/bin/go-native-fips-app"
	if _, err := os.Stat(testBinary); err != nil {
		t.Skip("native FIPS test binary not built; run: GOFIPS140=certified GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o test/resources/mock_native_fips/usr/bin/go-native-fips-app test/resources/native_go_fips_binary/main.go")
	}

	t.Run("BuildInfoSanity", func(t *testing.T) {
		bi, err := buildinfo.ReadFile(testBinary)
		if err != nil {
			t.Fatalf("failed to read build info: %v", err)
		}

		settings := make(map[string]string)
		for _, s := range bi.Settings {
			settings[s.Key] = s.Value
		}

		t.Logf("Go version: %s", bi.GoVersion)
		for k, v := range settings {
			t.Logf("  %s = %s", k, v)
		}

		godebug, ok := settings["DefaultGODEBUG"]
		if !ok {
			t.Fatal("expected DefaultGODEBUG in build settings")
		}
		hasFIPS140 := false
		for _, kv := range strings.Split(godebug, ",") {
			v := strings.TrimSpace(kv)
			if v == "fips140=auto" || v == "fips140=on" || v == "fips140=only" {
				hasFIPS140 = true
			}
		}
		if !hasFIPS140 {
			t.Errorf("DefaultGODEBUG=%q does not contain fips140={auto,on,only}", godebug)
		}

		gofips, ok := settings["GOFIPS140"]
		if !ok || gofips == "" || gofips == "off" {
			t.Errorf("expected GOFIPS140 to be a FIPS module version, got %q", gofips)
		}
		t.Logf("GOFIPS140 resolved to: %s", gofips)
	})

	t.Run("ScanBinaryResult", func(t *testing.T) {
		ctx := context.Background()
		res := ScanBinary(ctx, "../../test/resources/mock_native_fips", "/usr/bin/go-native-fips-app", nil)

		if res.Skip {
			t.Fatal("binary was skipped, expected it to be scanned")
		}
		if res.Error != nil {
			t.Fatalf("expected success, got error: %v", res.Error.Error)
		}

		found := false
		for _, m := range res.ModulesUsed {
			if m == "go" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected ModulesUsed to contain \"go\", got %v", res.ModulesUsed)
		}
	})
}
