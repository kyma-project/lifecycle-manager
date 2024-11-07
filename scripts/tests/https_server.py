import http.server
import ssl
import sys
import os

def start_server(directory_to_serve, cert_file, key_file, port):
    if not os.path.exists(directory_to_serve):
        print(f"Error: The directory {directory_to_serve} does not exist.")
        return

    current_dir = os.getcwd()
    cert_file_path = os.path.join(current_dir, cert_file)
    key_file_path = os.path.join(current_dir, key_file)

    # Change to the specified directory to serve
    os.chdir(directory_to_serve)

    # Create an SSL context
    context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
    context.load_cert_chain(certfile=cert_file_path, keyfile=key_file_path)

    # Configure and start the HTTPS server
    httpd = http.server.HTTPServer(('localhost', port), http.server.SimpleHTTPRequestHandler)
    httpd.socket = context.wrap_socket(httpd.socket, server_side=True)

    print(f"Serving on https://localhost:{port} from {directory_to_serve}")
    while True:
        httpd.handle_request()  # Handle one request

directory_to_serve = sys.argv[1]
cert_file = sys.argv[2]
key_file = sys.argv[3]
port = int(sys.argv[4])

start_server(directory_to_serve, cert_file, key_file, port)
