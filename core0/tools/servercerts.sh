#!env bash

#1- Generate server key
echo "Generating server key..."
openssl genrsa -out server.key 2048 || exit 1

echo "Generating server certificate signing request..."
openssl req -new -key server.key -out server.csr || exit 1

echo "Self signing the certificate"
openssl x509 -req -days 3650 -in server.csr -signkey server.key -out server.crt

echo "Please manually install the server.key and server.crt in nginx"
