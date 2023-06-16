# check-payload

## Releases

Release can be found on this [page](https://gitlab.cee.redhat.com/rphillip/check-payload/-/releases). There a Linux asset for download.

## About

This application checks an OpenShift release payload or an operator image for FIPS enabled binaries.

## build

```sh
git clone https://gitlab.cee.redhat.com/rphillip/check-payload.git
cd check-payload
go build
```

## run against an OpenShift release payload

```sh
 sudo ./check-payload \
   -url quay.io/openshift-release-dev/ocp-release:4.11.0-assembly.art6883.4 \
   -output-file report.txt \
   -filter /usr/lib/firmware,/usr/src/plugins,/usr/share/openshift,/usr/libexec/catatonit/catatonit,/usr/bin/pod,/usr/bin/tini-static,/usr/bin/cpb,/usr/sbin/build-locale-archive
```

## run against an container or operator image

```sh
sudo ./check-payload \
   -container-image registry.ci.openshift.org/ocp-priv/4.11-art-assembly-art6883-3-priv@sha256:138b1b9ae11b0d3b5faafacd1b469ec8c20a234b387ae33cf007441fa5c5d567 \
   -filter /usr/lib/firmware,/usr/src/plugins,/usr/share/openshift,/usr/libexec/catatonit/catatonit,/usr/bin/pod,/usr/bin/tini-static,/usr/bin/cpb,/usr/sbin/build-locale-archive
```

## node scan

```sh
IMAGE=some.registry.location/check-payload
podman  run --privileged -ti -v /:/myroot $IMAGE -node-scan /myroot
```
