# (Re-)Generate certificate files for testing pi service
#
# Dependencies:
#  1. cfssl installed and on path
#     use `go get github.com/cloudflare/cfssl/cmd/...` to install
#  2. GNU Make 4.x (or a port with `file` support)
#
# Rules:
#   1. CA cert and key is special; recreation will break any existing cert-key pairs that are in
#      use.
#     i.  CA is automatically created if missing and a host cert is requested (see below).
#     ii. CA cert can be manually created by:-
#         $ make ca.pem
#         which will generate the files ca.pem and ca-key.pem.
#   2. Default goal creates certs for service.pi.example.com, client.pi.example.com,
#      and otel.pi.example.com.
#   3. Other host certificates can be created by executing make _domain.com_.pem,
#      where _domain.com_ is the domain to use.
#      E.g. to create a cert for foobar.com
#      $ make foobar.com.pem
#      Above command will create foobar.com.pem (cert) and foobar.com-key.pem
#      (private key) files in current dir. Cert will have CN=foobar.com as subject,
#      with SANs for foorbar.com, localhost, and 127.0.0.1

CA_CONFIG := ca-config.json
CA_CSR := ca-csr.json
CA_CERT := ca.pem
DOMAIN := pi.example.com
DEFAULT_SET := service client otel

# When on Windows, force shell to be cmd instead of bash or any other shell
ifeq ($(OS),Windows_NT)
SHELL=cmd.exe
endif

define ca-config-content
{
  "signing": {
    "default": {
      "expiry": "240h"
    },
    "profiles": {
      "host": {
        "usages": [
          "signing",
          "key encipherment",
          "client auth",
          "server auth"
        ],
        "expiry": "240h"
      }
    }
  }
}
endef

define ca-csr-content
{
  "CN": "PiService Testing CA",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "US",
      "L": "Laguna Niguel",
      "O": "Matthew Emes",
      "OU": "Pi v2",
      "ST": "California"
    }
  ]
}
endef

define host-csr-content
{
  "CN": "!!name!!",
  "hosts": [
    "!!name!!",
    "localhost",
    "127.0.0.1"
  ],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "US",
      "L": "Laguna Niguel",
      "O": "Matthew Emes",
      "OU": "Pi v2",
      "ST": "California"
    }
  ]
}
endef

# Default target generates fake certificate for default hosts
all: $(foreach host,$(DEFAULT_SET),$(host).$(DOMAIN).pem)

# Delete all certs, keys and csr's, except for those used by the CA
clean:
	-rm $(filter-out $(CA_CERT) $(subst ca,ca-key,$(CA_CERT)) $(CA_CSR) $(CA_CONFIG),$(wildcard *pem *csr *json))

# Clean-up the CA files too
realclean: clean
	-rm $(CA_CERT) $(subst ca,ca-key,$(CA_CERT)) $(CA_CSR) $(CA_CONFIG)

# Create a CA cert from CSR file
$(CA_CERT): $(CA_CSR)
	cfssl gencert -initca $< | cfssljson -bare $(subst .pem,,$@)

# Recreate CA CSR as needed
$(CA_CSR):
	$(file >$@,$(ca-csr-content))

# Recreate CA config as needed
$(CA_CONFIG):
	$(file >$@,$(ca-config-content))

# Create a CSR json file for a domain cert
%-csr.json:
	$(file >$@, $(subst !!name!!,$*,$(host-csr-content)))

# General rule to generate a domain cert
%.pem: %-csr.json $(CA_CERT) $(CA_CONFIG)
	cfssl gencert -ca $(CA_CERT) -ca-key $(subst ca,ca-key,$(CA_CERT)) -config $(CA_CONFIG) -profile host $< | \
	cfssljson -bare $*

.PHONY: all clean realclean
