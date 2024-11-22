#!/bin/bash

# https://gist.github.com/NovikovRoman/30302d68a661526dfeddd503da39e8f9

IP=127.0.0.1
DNS=DNS:localhost # DNS:*.tls,DNS:0.0.0.0
CA_DAYS=365
SERVER_CERT_DAYS=365
CERTS_DIR="certs"

# Check if openssl is installed
if ! command -v openssl &> /dev/null; then
    echo "Error: OpenSSL is not installed."
    exit 1
fi

mkdir -p ${CERTS_DIR} && cd ${CERTS_DIR}

rm *.pem
rm *.cnf

echo "Generate CA's private key"
openssl req -x509 -newkey rsa:4096 -days ${CA_DAYS} -nodes \
    -keyout ca-key.pem -out ca-cert.pem -subj "/CN=Certificate authority/" 2>/dev/null

echo "CA's self-signed certificate"
openssl x509 -in ca-cert.pem -noout # -text

echo "Generate web server's private key and certificate signing request (CSR)"
openssl req -newkey rsa:4096 -nodes -keyout server-key.pem -out server-req.pem \
    -subj "/CN=Certificate authority/" 2>/dev/null

# Remember that when we develop on localhost,
# It’s important to add the IP:0.0.0.0 as an Subject Alternative Name (SAN) extension to the certificate.
echo "subjectAltName=${DNS},IP:${IP}" > server-ext.cnf
# Or you can use localhost DNS and grpc.ssl_target_name_override variable
# echo "subjectAltName=DNS:localhost" > server-ext.cnf

echo "Use CA's private key to sign web server's CSR and get back the signed certificate"
openssl x509 -req -in server-req.pem -days ${SERVER_CERT_DAYS} \
    -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extfile server-ext.cnf
