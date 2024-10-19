package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/google/uuid"
)

type Client struct {
	Address  string
	ID       string
	GameMode byte
}

type GameServers struct {
	Address  string
	GameMode byte
}

type Match struct {
	Client1Address string
	Client2Address string
}

type Header struct {
	Version  string
	GameMode byte
	clientID string
}

const VERSION = "003" // Version as a 3-character string

// Create client token
func createClientToken(clients map[string]Client, clientAddr string, clientID string, GameMode byte, mu *sync.Mutex) {
	mu.Lock()
	clients[clientID] = Client{Address: clientAddr, ID: clientID, GameMode: GameMode}
	mu.Unlock()
	fmt.Printf("[%s] Token created\n", clientID)
}

// Remove client
func deleteClientToken(clients map[string]Client, clientAddr string, mu *sync.Mutex) {
	mu.Lock()
	delete(clients, clientAddr)
	mu.Unlock()
}

// List clients
func ListAllTokens(clients map[string]Client, mu *sync.Mutex) {
	mu.Lock()
	fmt.Println("Current clients:")
	index := 0
	for _, client := range clients {
		fmt.Printf("%d - Address: %s, ID: %s, GameMode: %d\n", index, client.Address, client.ID, client.GameMode)
		index++
	}
	mu.Unlock()
}

// Check if client has a token, by ID
func clientHasToken(clients map[string]Client, clientId string) bool {
	_, ok := clients[clientId]
	return ok
}

// Function to find the address and ID of the client with a specific GameMode
func findClientAddressByGameMode(clients map[string]Client, gameMode byte, clientID string) (string, string) {
	for _, client := range clients {
		if client.GameMode == gameMode && client.ID != clientID {
			return client.Address, client.ID
		}
	}
	return "", ""
}

func findAvailableGameServer(gameMode byte) string {
	// servers := map[string]GameServers{
	// 	"server1": {
	// 		Address:  "192.168.1.1",
	// 		GameMode: 1,
	// 	},
	// 	"server2": {
	// 		Address:  "10.0.0.5",
	// 		GameMode: 2,
	// 	},
	// }
	servers := []GameServers{
		{Address: "192.168.1.1", GameMode: 0},
		{Address: "192.168.2.2", GameMode: 1},
		{Address: "10.0.0.5", GameMode: 2},
	}
	for _, server := range servers {
		if server.GameMode == gameMode {
			return server.Address
		}
	}
	return ""
}

func createMatch(client1, client2 string) Match {
	return Match{
		Client1Address: client1,
		Client2Address: client2,
	}
}

// Function to validate the header
func checkHeaders(conn net.Conn, client1Address string) (*Header, error) {
	// Define a buffer to hold the data (40 bytes)
	// Total size 40 bytes = 3 Version + 1 GameMode + 36 UUID (ClientID)
	buffer := make([]byte, 40)

	n, err := conn.Read(buffer)
	if err != nil {
		if err != io.EOF {
			return nil, fmt.Errorf("error reading from connection: %v", err)
		}
		return nil, err
	}

	if n != len(buffer) {
		return nil, fmt.Errorf("[%s] ERROR: Received invalid header data", client1Address)
	}

	var header Header
	header.Version = string(buffer[0:3])

	if header.Version != VERSION {
		return nil, fmt.Errorf("version mismatch: received version %s, expected version %s", header.Version, VERSION)
	}

	// TODO: Validate GameMode
	header.GameMode = buffer[3]
	header.clientID = string(buffer[4:40])

	return &header, nil
}

func handleConnection(conn net.Conn, clients map[string]Client, mu *sync.Mutex) {
	defer conn.Close()
	client1Address := conn.RemoteAddr().String()
	fmt.Printf("[%s] Client connected\n", client1Address)
	var client1 string

	for {
		// Read the first 4 bytes to get the message length
		var messageLength int32
		err := binary.Read(conn, binary.BigEndian, &messageLength)

		if err == io.EOF {
			fmt.Printf("[%s] Connection closed by client\n", client1Address)
			return
		} else if err != nil {
			fmt.Println("Error reading message length:", err)
			return
		}

		// Read the actual message of the given length
		message := make([]byte, messageLength)
		_, err = io.ReadFull(conn, message)
		if err != nil {
			fmt.Println("Error reading message:", err)
			return
		}

		switch string(message) {
		case "hello":
			fmt.Printf("[%s] Received 'hello' message\n", client1Address)
			// sending the UUID back to the client for future ID
			client1 = uuid.New().String()
			clientIDbuffer := []byte(client1)
			_, err := conn.Write(clientIDbuffer)
			if err != nil {
				fmt.Println("Error writing to connection: ", err)
				return
			}
		case "token":
			fmt.Printf("[%s] Received 'token' message\n", client1Address)
			// Validate the header
			header, err := checkHeaders(conn, client1Address)
			if err != nil {
				fmt.Println(err)
				return
			}

			if header.clientID != client1 {
				fmt.Printf("[%s] UUID mismatch: expected %s, got %s\n", client1, client1, header.clientID)
				return
			}

			if clientHasToken(clients, client1) {
				fmt.Printf("[%s] Client already has a token!\n", client1)
				return
			} else {
				createClientToken(clients, client1Address, client1, header.GameMode, mu)
			}

		case "match":
			fmt.Printf("[%s] Received 'match' message\n", client1Address)
			// Validate the header for the match request
			header, err := checkHeaders(conn, client1Address)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("[%s] Client match request with GameMode=%d and ID=%s\n", client1Address, header.GameMode, header.clientID)

			if clientHasToken(clients, client1) {
				fmt.Printf("[%s] Client already has a token! Let's find a match\n", client1)
				ListAllTokens(clients, mu)
				client2Address, client2 := findClientAddressByGameMode(clients, clients[client1].GameMode, clients[client1].ID)
				if client2 != "" {
					fmt.Printf("[%s] Found client with GameMode=%d: %s\n", client1, clients[client1].GameMode, client2)
					gameserver := findAvailableGameServer(clients[client1].GameMode)
					if gameserver != "" {
						match := createMatch(client1Address, client2Address)
						// this can be another service/DB etc
						fmt.Printf("Match created! Sending gameserver %s to clients %s(%s) and %s(%s)\n", gameserver, client1, match.Client1Address, client2, match.Client2Address)
					}
				} else {
					fmt.Printf("[%s] Cannot find another player with GameMode %d!\n", client1, clients[client1].GameMode)
				}
			} else {
				fmt.Printf("[%s] client doesn't have a token!", client1)
			}
		default:
			fmt.Printf("[%s] Unknown message received: %s\n", client1Address, message)
		}
	}
}

func main() {
	PORT := ":5555"
	clients := make(map[string]Client)
	var mu sync.Mutex

	l, err := net.Listen("tcp4", PORT)
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("Listening on %s...\n", l.Addr().String())
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err)
			return
		}
		go handleConnection(conn, clients, &mu)
	}
}
