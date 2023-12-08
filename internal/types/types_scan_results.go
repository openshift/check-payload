package types

func NewScanResults() *ScanResults {
	return &ScanResults{}
}

func (sr *ScanResults) Append(result *ScanResult) *ScanResults {
	sr.Items = append(sr.Items, result)
	return sr
}

func (sr *ScanResults) AppendResults(results *ScanResults) *ScanResults {
	sr.Items = append(sr.Items, results.Items...)
	return sr
}
