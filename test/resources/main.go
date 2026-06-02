//go:debug fips140=auto

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	_ = &tls.Config{}
	_ = &http.Server{}
	b, _ := json.Marshal(os.Args)
	fmt.Println(string(b))
}
