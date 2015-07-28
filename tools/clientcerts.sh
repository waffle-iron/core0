#!env bash

EXE="$0"
SERVER_KEY="$1"
SERVER_CRT="$2"
CLIENT_NAME="$3"

function help {
    echo "USAGE: $EXE <server-key-file> <server-certificate-file> <client-name>"
}

if [ ! -f "$SERVER_KEY" ]; then
    help && exit 1
fi

if [ ! -f "$SERVER_CRT" ]; then
    help && exit 1
fi

if [ -z "$CLIENT_NAME" ]; then
    help && exit 1
fi

echo "Generating client key..."
CLIENT_KEY="client-$CLIENT_NAME.key"
CLIENT_CSR="client-$CLIENT_NAME.csr"
CLIENT_CRT="client-$CLIENT_NAME.crt"

openssl genrsa -out $CLIENT_KEY 2048

echo "Gernerating client certificate signing request..."
openssl req -new -key $CLIENT_KEY -out $CLIENT_CSR

echo "Signing client certificate, with server certificate"
openssl x509 -req -days 365 -in $CLIENT_CSR -CA $SERVER_CRT -CAkey $SERVER_KEY -set_serial 01 -out $CLIENT_CRT
