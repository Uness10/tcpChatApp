# TCP Chat Application

A feature-rich TCP chat application written in Go that supports room-based chat, direct messaging, file sharing, and message encryption.

## Features

- 🔐 User Authentication (Register/Login)
- 👥 Room-based Chat System
- 📝 Direct Messaging between Users
- 🔒 End-to-End Encrypted Messages
- 📁 File Sharing
- 📜 Message History
- 🟢 User Status (Online/Away/Busy/Offline)
- 💾 Persistent Message Storage

## Building and Running

### Prerequisites
- Go 1.20 or later
- Windows (for build.bat) or similar commands for other OS

### Building
Run the build script:
```bash
build.bat
```

This will create:
- `server/chat-server.exe`
- `client/chat-client.exe`

### Running
1. Start the server:
```bash
cd server
./chat-server.exe
```

2. Start one or more clients:
```bash
cd client
./chat-client.exe
```

## Command Reference

### Authentication
- Register: `/register <username> <password>`
- Login: `/login <username> <password>`

### Room Management
- List rooms: `/rooms`
- Create room: `/create <room-name>`
- Join room: `/join <room-name>`
- Leave room: `/leave`

### Messaging
- Regular chat: Just type your message
- Direct message: `/msg <username> <message>`
- Encrypted message: `/encrypt <username> <message>`

### File Sharing
- Send file: `/file <filepath>`
Files are automatically saved in:
- Server: `uploads/<room-name>/`
- Client: `appData/`

### User Status
- Set status: `/status <online|away|busy|offline>`

### History
- Room history: `/history`

### Exit
- Close client: `/exit`

## Example Workflow

1. **Starting Up**
```bash
# Terminal 1 - Start Server
cd server
./chat-server.exe

# Terminal 2 - Start Client 1
cd client
./chat-client.exe

# Terminal 3 - Start Client 2
cd client
./chat-client.exe
```

2. **Basic Usage Example**
```
# Client 1
/register alice pass123
SUCCESS: Registered and logged in successfully
/create room1
[02:54:10] alice has joined the room
[02:54:11] bob has joined the room
SUCCESS: Room created and joined: room1
Hello everyone!
[02:54:18] [room1] alice: Hello everyone!
[03:16:08] bob is sending file: greetings.txt (chunk 1/1)
All chunks received for greetings.txt. Assembling file...
File greetings.txt saved successfully to appData directory.
[03:16:08] File greetings.txt uploaded by bob is available

# Client 2
/register bob pass123
SUCCESS: Registered and logged in successfully
/join room1
[02:54:11] bob has joined the room
SUCCESS: Joined room: room1
[02:54:18] [room1] alice: Hello everyone!
/file greetings.txt
[03:16:08] bob is sending file: greetings.txt
[03:16:08] File greetings.txt uploaded by bob is available
/history
Message history for room general:
[03:24:18] [general] alice: Hello everyone!

3. **Advanced Features Example**
```
# Client 1 (alice)
/login alice alice123
/msg bob Hey bob!
[DM to bob]: Hey bob!
/encrypt bob Here's the secret message
[Encrypted to bob]: Here's the secret message

# Client 2 (bob) - Still connected
[12:34:56] [DM from alice]: Hey bob!
[12:35:01] [Encrypted from alice]: Here's the secret message
```

## Project Structure

```
proj/
├── server/
│   ├── main.go          # Server entry point
│   ├── server.go        # Core server implementation
│   ├── client.go        # Client handler
│   ├── room.go          # Room management
│   ├── auth.go          # Authentication
│   └── message_store.go # Message persistence
├── client/
│   └── main.go          # Client implementation
├── shared/
│   ├── message.go       # Message types
│   ├── file.go         # File handling
│   └── events.go       # Event system
├── build.bat           # Build script
└── README.md          # This file
```

## Data Storage

- Message history: `message_history/*.json`
- Uploaded files: `uploads/<room-name>/`
- Downloaded files: `appData/`

## Security Notes

- Passwords are hashed using SHA-256
- Direct messages can be encrypted using AES-128
- For demonstration purposes, a fixed encryption key is used
- In production, implement proper key exchange mechanisms

## Error Handling

The application includes robust error handling for:
- Network disconnections
- Invalid commands
- Missing permissions
- File operations
- Authentication failures

