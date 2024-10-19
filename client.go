package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

const (
	HOST    = "localhost"
	PORT    = "5555"
	TYPE    = "tcp"
	VERSION = "0031" // Version as a 3-character string
)

type Header struct {
	Version  string // Version is now a string
	GameMode byte   // GameMode is a 1 byte
	clientID string
}

func handleError(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func connectToServer() (*net.TCPConn, error) {
	tcpServer, err := net.ResolveTCPAddr(TYPE, HOST+":"+PORT)
	handleError(err)

	conn, err := net.DialTCP(TYPE, nil, tcpServer)
	return conn, err
}

func receiveClientID(conn *net.TCPConn) (string, error) {
	clientID := make([]byte, 36) // UUID is 36 bytes
	n, err := conn.Read(clientID)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(clientID[:n]), nil
}

func sendMessageWithLength(conn *net.TCPConn, message string) error {
	messageBytes := []byte(message)
	messageLength := int32(len(messageBytes))

	var buffer bytes.Buffer

	// Write the length of the message as 4 bytes
	err := binary.Write(&buffer, binary.BigEndian, messageLength)
	if err != nil {
		return err
	}

	buffer.Write(messageBytes)

	// Send the buffer (length + message) to the server
	_, err = conn.Write(buffer.Bytes())
	return err
}

func sendHeader(conn *net.TCPConn, header Header) error {
	buffer := make([]byte, 40)
	copy(buffer[0:3], []byte(header.Version))
	buffer[3] = header.GameMode
	clientIDBytes := []byte(header.clientID)

	if len(clientIDBytes) != 36 {
		clientIDBytes = append(clientIDBytes[:0:0], clientIDBytes...) // truncate if necessary
		fmt.Println("Invalid client ID length. Using truncated ID.")
	}
	copy(buffer[4:40], clientIDBytes)

	_, err := conn.Write(buffer)
	return err
}

func main() {
	header := Header{
		Version:  VERSION, // Example version values
		GameMode: 1,       // Example GameMode value as a single byte
	}

	conn, err := connectToServer()
	handleError(err)
	defer conn.Close()

	// sending initial 'hello' message
	message := "hello"
	if err := sendMessageWithLength(conn, message); err != nil {
		handleError(err)
	}
	fmt.Println("'hello' message sent successfully!")

	// Getting back the client ID (36 bytes long UUID) from the server
	clientID, err := receiveClientID(conn)
	handleError(err)
	header.clientID = clientID
	fmt.Println("Received clientID from server:", header.clientID)

	message = "token"
	if err := sendMessageWithLength(conn, message); err != nil {
		handleError(err)
	}
	fmt.Println("'token' message sent successfully!")

	if err := sendHeader(conn, header); err != nil {
		handleError(err)
	}
	fmt.Println("Header sent successfully after 'token' message!")

	message = "match"
	if err := sendMessageWithLength(conn, message); err != nil {
		handleError(err)
	}
	fmt.Println("'match' message sent successfully!")

	if err := sendHeader(conn, header); err != nil {
		handleError(err)
	}
	fmt.Println("Header sent successfully after 'match' message!")

}
