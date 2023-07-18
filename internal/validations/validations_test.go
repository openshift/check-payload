package validations

import (
	"bytes"
	"testing"
)

const testGoVersionDetailed = `/usr/bin/runc: go1.19.9
	path	github.com/opencontainers/runc
	build	-compiler=gc
	build	-ldflags="-X main.gitCommit= -X main.version=1.1.6 -linkmode=external -compressdwarf=false -B 0x0bfd31e9756ba9e517cb946d2b1c23012b6919ed -extldflags '-Wl,-z,relro  -Wl,-z,now -specs=/usr/lib/rpm/redhat/redhat-hardened-ld'"
	build	-tags=rpm_crashtraceback,libtrust_openssl,selinux,seccomp,strictfipsruntime
	build	CGO_ENABLED=1
	build	CGO_CFLAGS="-O2 -g -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-protector-strong -grecord-gcc-switches -specs=/usr/lib/rpm/redhat/redhat-hardened-cc1 -specs=/usr/lib/rpm/redhat/redhat-annobin-cc1 -m64 -mtune=generic -fasynchronous-unwind-tables -fstack-clash-protection -fcf-protection -D_GNU_SOURCE -D_LARGEFILE_SOURCE -D_LARGEFILE64_SOURCE -D_FILE_OFFSET_BITS=64"
	build	CGO_CPPFLAGS=
	build	CGO_CXXFLAGS=
	build	CGO_LDFLAGS=
	build	GOARCH=amd64
	build	GOOS=linux
	build	GOAMD64=v1`

func BenchmarkValidateGoVersion(b *testing.B) {
	baton := &Baton{}
	out := bytes.NewBuffer([]byte(testGoVersionDetailed))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := doValidateGoVersion(out, baton); err != nil {
			b.Fatal(err)
		}
	}
}
