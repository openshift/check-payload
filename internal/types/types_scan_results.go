package types

func NewScanResults() *ScanResults {
	return &ScanResults{}
}

func (sr *ScanResults) Append(result *ScanResult) *ScanResults {
	sr.Items = append(sr.Items, result)
	return sr
}
