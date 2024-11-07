import http.server
import ssl
import sys
import os

directory_to_serve = sys.argv[1]
cert_file = sys.argv[2]
key_file = sys.argv[3]
port = int(sys.argv[4])

current_dir = os.getcwd()
cert_file_path = os.path.join(current_dir, cert_file)
key_file_path = os.path.join(current_dir, key_file)

# Change to the specified directory
os.chdir(directory_to_serve)

# Configure the HTTPS server
context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
context.load_cert_chain(certfile=cert_file_path, keyfile=key_file_path)
httpd = http.server.HTTPServer(('localhost', port), http.server.SimpleHTTPRequestHandler)
httpd.socket = context.wrap_socket(httpd.socket, server_side=True)

print("Serving on https://localhost:8080 from {directory_to_serve}")
httpd.serve_forever()
