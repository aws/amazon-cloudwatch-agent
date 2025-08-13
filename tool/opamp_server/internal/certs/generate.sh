# First create a certificate authority (CA) that will sign all serve and client certificates.
# The server and the client trust this CA. Typically the public key of the CA certificate
# will be hard-coded in the server and client implementations (and never changes or changes
# only in the catastrophic even of CA private key leak or when predefined CA rotation time
# comes - typically annual or rarer).

# Create CA private key
openssl genrsa -out private/ca.key.pem 4096

# Create CA certificate
openssl req -new -x509 -days 3650 -key private/ca.key.pem -out certs/ca.cert.pem -config openssl.conf

# This section shows how to generate a client-side certificate that the agent can use
# to connect to the OpAMP server for the first time. This is not currently used in
# the example, but we show how it can be done if needed.
#
# Create a private key for client certificate.
# openssl genrsa -out client_certs/client.key.pem 4096
#
# Generate a client CRS
# openssl req -new -key client_certs/client.key.pem -out client_certs/client.csr -config client.conf
#
# Create a client certificate
# openssl ca -config openssl.conf -days 1650 -notext -batch -in client_certs/client.csr -out client_certs/client.cert.pem
# The generated pair of files in client_certs can be now used by TLS connection.

# Create private key for server certificate
openssl genrsa -out server_certs/server.key.pem 4096

# Generate server CRS
openssl req -new -key server_certs/server.key.pem -out server_certs/server.csr -config server.conf

# Create Server certificate
openssl ca -config openssl.conf -extfile server_ext.conf -days 1650 -notext -batch -in server_certs/server.csr -out server_certs/server.cert.pem
# The generated pair of files in server_certs can be now used by TLS connection.
