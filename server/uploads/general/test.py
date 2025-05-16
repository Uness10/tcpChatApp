from flask import Flask, request

app = Flask(__name__)

@app.route('/upload', methods=['POST'])
def upload():
    payload = request.get_json()  # If you're sending JSON
    print("Received payload:", payload)
    return "Payload received", 200

if __name__ == '__main__':
    app.run(debug=True)
