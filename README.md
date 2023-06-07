# check-payload

This application checks an OpenShift release payload or an operator image for FIPS enabled binaries.

# build

```sh
git clone https://gitlab.cee.redhat.com/rphillip/check-payload.git
cd check-payload
go build
```

# run against an OpenShift release payload

```sh
 sudo ./check-payload \
   -url quay.io/openshift-release-dev/ocp-release:4.11.0-assembly.art6883.4 \
   -output-file report.txt \
   -filter /usr/lib/firmware,/usr/src/plugins,/usr/share/openshift,/usr/libexec/catatonit/catatonit,/usr/bin/pod
```

# run against an operator image

```sh
sudo ./check-payload \
	-operator-image registry.ci.openshift.org/ocp-priv/4.11-art-assembly-art6883-3-priv@sha256:138b1b9ae11b0d3b5faafacd1b469ec8c20a234b387ae33cf007441fa5c5d567 \
   -filter /usr/lib/firmware,/usr/src/plugins,/usr/share/openshift,/usr/libexec/catatonit/catatonit,/usr/bin/pod
```