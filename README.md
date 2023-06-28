# check-payload

## Releases

Release can be found on this [page](https://gitlab.cee.redhat.com/rphillip/check-payload/-/releases). There a Linux asset for download.

## About

This application checks an OpenShift release payload or an operator image for FIPS enabled binaries.

## Filters

Filter are now contained within the config.toml file.

## Build

```sh
git clone https://gitlab.cee.redhat.com/rphillip/check-payload.git
cd check-payload
make
```

## Run

### Prerequisities
* podman should be installed on the node.
* podman should be configured with pull secrets for the images to be scanned.

### Scan an OpenShift release payload

```sh
 sudo ./check-payload scan payload \
   --url quay.io/openshift-release-dev/ocp-release:4.11.0-assembly.art6883.4 \
   --output-file report.txt
```

### Scan a container or operator image

```sh
sudo ./check-payload scan operator \
  --spec registry.ci.openshift.org/ocp-priv/4.11-art-assembly-art6883-3-priv@sha256:138b1b9ae11b0d3b5faafacd1b469ec8c20a234b387ae33cf007441fa5c5d567
```

### Scan a node

```sh
IMAGE=some.registry.location/check-payload
podman  run --privileged -ti -v /:/myroot $IMAGE scan node --root /myroot
```
