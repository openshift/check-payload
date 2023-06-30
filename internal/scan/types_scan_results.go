package scan

func NewScanResults() *ScanResults {
	return &ScanResults{}
}

func (sr *ScanResults) Append(result *ScanResult) {
	sr.Items = append(sr.Items, result)
}
