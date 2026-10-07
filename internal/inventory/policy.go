package inventory

import (
	_ "embed"
	"encoding/json"
	"strings"

	"golang.org/x/mod/semver"
)

const xcrypto = "golang.org/x/crypto/"

type policyRule struct {
	Packages             []string `json:"packages"`
	Classification       string   `json:"classification"`
	Candidates           []string `json:"candidates"`
	Reason               string   `json:"reason"`
	Boundary             string   `json:"boundary"`
	MinimumModuleVersion string   `json:"minimumModuleVersion"`
}

type policy struct {
	Version string       `json:"version"`
	Rules   []policyRule `json:"rules"`
}

//go:embed policy.json
var policyJSON []byte

var defaultPolicy = func() policy {
	var p policy
	if err := json.Unmarshal(policyJSON, &p); err != nil {
		panic(err)
	}
	return p
}()

// classify intentionally keeps context-sensitive packages unresolved.
// The policy is provisional until reconciled with the authoritative document.
func classify(pkg, symbol, version string, chain []string) Finding {
	f := Finding{
		Package: pkg, Symbol: symbol, ModuleVersion: version,
		Origin:         "module",
		Classification: "unresolved", Guard: "unresolved", PQCBoundary: "unresolved",
		Reason: "No classification rule; requires policy review.",
	}
	name := strings.TrimPrefix(pkg, xcrypto)
	if strings.Contains(symbol, "vendor/"+xcrypto) {
		f.Origin = "stdlib-vendor"
	}
	if strings.HasSuffix(symbol, ".init") || strings.Contains(symbol, ".init#") || strings.Contains(symbol, ".init.") {
		f.Candidates = []string{"F3"}
		f.Reason = "Package initialization entry; this name does not establish primitive execution. Primitive callees are inventoried separately."
		return f
	}
	for _, rule := range defaultPolicy.Rules {
		for _, candidate := range rule.Packages {
			if name != candidate {
				continue
			}
			f.Classification, f.Reason, f.Candidates = rule.Classification, rule.Reason, rule.Candidates
			if rule.Boundary != "" {
				f.PQCBoundary = rule.Boundary
			}
			if rule.MinimumModuleVersion != "" && (!semver.IsValid(version) || semver.Compare(version, rule.MinimumModuleVersion) < 0) {
				f.Classification = "unresolved"
				f.Candidates = []string{rule.Classification}
				f.Reason = "Cannot establish modern stdlib delegation for this dependency version or replacement."
			}
		}
	}
	if name == "sha3" && strings.Contains(symbol, "NewLegacyKeccak") {
		f.Classification, f.Reason = "F1", "Legacy Keccak is F1 when used for a security function."
		f.Candidates = nil
	}
	if name == "blowfish" {
		for _, caller := range chain {
			if strings.HasPrefix(caller, xcrypto+"bcrypt.") || strings.HasPrefix(caller, xcrypto+"ssh/internal/bcrypt_pbkdf.") {
				f.Classification = "unresolved"
				f.Reason = "One source call path includes bcrypt/bcrypt_pbkdf. Other callers may use this primitive directly; substrate-only use and exception scope require review."
				f.Candidates = []string{"F1", "F2"}
				break
			}
		}
	}
	return f
}

func packageFromSymbol(symbol string) string {
	start := strings.Index(symbol, xcrypto)
	if start < 0 {
		return ""
	}
	rest := symbol[start+len(xcrypto):]
	end := strings.IndexByte(rest, '.')
	if end < 0 {
		return ""
	}
	return xcrypto + rest[:end]
}
