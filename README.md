# 🔌 TCP Chat Application (GoLang)

A full-featured, modular TCP chat system written in Go, supporting real-time room-based messaging, direct user communication, encrypted messages, and file transfers. Built for simplicity, security, and scalability.

---

## ✨ Features

* 🔐 **User Authentication** (Register / Login)
* 👥 **Room-Based Group Chat**
* 💬 **Direct Messaging (DMs)**
* 🔒 **End-to-End Message Encryption**
* 📁 **File Sharing (with chunked uploads)**
* 🧾 **Message History & Persistence**
* 🟢 **User Presence Status (Online/Away/Busy/Offline)**

---

## 🛠️ Getting Started

### ✅ Prerequisites

* Go 1.20+
* Windows (for `build.bat`) or equivalent build setup on Linux/Mac

### ⚙️ Building

Run the build script:

```bash
build.bat
```

This generates:

* `server/chat-server.exe`
* `client/chat-client.exe`

### 🚀 Running

**Start the server:**

```bash
cd server
./chat-server.exe
```

**Start a client instance:**

```bash
cd client
./chat-client.exe
```

Run multiple clients for multi-user simulation.

---

## 📖 Command Reference

### 👤 Authentication

* `/register <username> <password>` – Register a new user
* `/login <username> <password>` – Log in as a registered user

### 🧩 Room Management

* `/rooms` – List available rooms
* `/create <room-name>` – Create a new chat room
* `/join <room-name>` – Join an existing room
* `/leave` – Leave the current room

### 💬 Messaging

* *(Default)* – Send a message to the current room
* `/msg <username> <message>` – Send a private message
* `/encrypt <username> <message>` – Send an AES-encrypted message

### 📁 File Sharing

* `/file <filepath>` – Send a file to the room
* Server stores to: `uploads/<room-name>/`
* Client receives into: `appData/`

### 🟢 User Presence

* `/status <online|away|busy|offline>` – Update your availability

### 🕘 Message History

* `/history` – Show current room's message history

### 🔚 Exit

* `/exit` – Exit the chat client

---

## 🎬 Example Workflow

### 1. Boot Sequence

```bash
# Server
cd server
./chat-server.exe

# Client 1
cd client
./chat-client.exe

# Client 2
cd client
./chat-client.exe
```

### 2. Basic Use

```bash
# Client 1
/register alice pass123
/create room1
Hello everyone!

# Client 2
/register bob pass123
/join room1
/file greetings.txt
/history
```

### 3. Advanced Usage

```bash
# Client 1
/login alice pass123
/msg bob Hey bob!
/encrypt bob Here's the secret message

# Client 2
[DM from alice]: Hey bob!
[Encrypted from alice]: Here's the secret message
```

---

## 🗂️ Project Structure

```
proj/
├── server/
│   ├── main.go            # Entry point
│   ├── server.go          # TCP server logic
│   ├── client.go          # Client session handler
│   ├── room.go            # Room lifecycle & broadcasting
│   ├── auth.go            # User auth logic
│   └── message_store.go   # Persistent storage handling
├── client/
│   └── main.go            # Client CLI implementation
├── shared/
│   ├── message.go         # Message struct & types
│   ├── file.go            # File chunking & assembly
│   └── events.go          # Event definitions
├── build.bat              # Windows build script
└── README.md              # You’re reading it 😉
```

---

## 💾 Data Storage

* Message logs: `message_history/*.json`
* Server-side uploads: `uploads/<room-name>/`
* Client-side downloads: `appData/`

---

## 🔐 Security Notes

* Passwords stored as **SHA-256 hashes**
* Encrypted DMs use **AES-128** (with static demo key)
* Production-grade version should use **proper key exchange (Diffie-Hellman or TLS)**

---

## 🛡️ Robust Error Handling

Includes protections for:

* Network interruptions
* Malformed or unauthorized commands
* File I/O issues
* Authentication or room access failures

---

## 📌 Final Notes

This project is a proof of concept for secure TCP-based chat systems in Go. Easily extensible to support:

* WebSocket frontend
* Token-based auth (JWT)
* Group policies & moderation
* Mobile client integrations
