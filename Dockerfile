FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.20-openshift-4.14 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN make

FROM registry.ci.openshift.org/ocp/4.14:base-rhel9
ARG OC_VERSION=latest
RUN dnf -y update && dnf install -y binutils file go podman runc && dnf clean all
RUN wget -O "openshift-client-linux-${OC_VERSION}.tar.gz" "https://mirror.openshift.com/pub/openshift-v4/amd64/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" \
  && tar -C /usr/local/bin -xzvf "openshift-client-linux-$OC_VERSION.tar.gz" oc
RUN curl --fail --retry 3 -LJO https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/latest-4.14/opm-linux.tar.gz && \
    tar -xzf opm-linux.tar.gz && \
    mv ./opm /usr/local/bin/ && \
    rm -f opm-linux.tar.gz
COPY --from=builder /app/check-payload /check-payload

ENTRYPOINT ["/check-payload"]
LABEL com.redhat.component="check-payload"
