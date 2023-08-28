package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

func createHeader() []byte {
	header := make([]byte, 12)

	var id uint16 = 1234
	binary.BigEndian.PutUint16(header[0:2], id)
	
	var flags uint16 = 0
	binary.BigEndian.PutUint16(header[2:4], flags)

	var numQuestions uint16 = 1
	binary.BigEndian.PutUint16(header[4:6], numQuestions)

	// The rest are all zeros, and we don't need to add them
	// manually due to zero-initialization

	return header
}

func encodeDomain(domain string) []byte {
	// Currently supports second-level domains only
	// example.com works, but not sub.example.com
	parts := strings.Split(domain, ".")

	arrLength := 3 + len(parts[0]) + len(parts[1])
	encoded := make([]byte, arrLength)

	nextLoc := 0
	encoded[0] = byte(len(parts[0]))
	nextLoc += 1
	copy(encoded[nextLoc:], []byte(parts[0]))

	nextLoc += len(parts[0])
	encoded[nextLoc] = byte(len(parts[1]))
	nextLoc += 1
	copy(encoded[nextLoc:], []byte(parts[1]))

	// Last byte is already set to zero.

	return encoded
}

func callDNSServer(msg []byte) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		fmt.Println("Error connecting to DNS server", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(msg)
	if err != nil {
		fmt.Println("Error sending msg to DNS server", err)
		return
	}

	fmt.Println("DNS query sent")

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buffer := make([]byte, 512)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading response", err)
		return
	}

	fmt.Println(buffer)
	fmt.Println(n)
}

func createQuestion(domain []byte) []byte {
	question := make([]byte, len(domain)+4)

	nextLoc := 0
	copy(question[nextLoc:], domain)
	
	nextLoc += len(domain)
	var recordType uint16 = 1 // TYPE_A
	binary.BigEndian.PutUint16(question[nextLoc:nextLoc+2], recordType)

	nextLoc += 2
	var class uint16 = 1 // CLASS_IN
	binary.BigEndian.PutUint16(question[nextLoc:], class)

	return question
}

func main() {
	header := createHeader()
	encDomain := encodeDomain("google.com")
	question := createQuestion(encDomain)

	fmt.Println(header)
	fmt.Println(encDomain)
	fmt.Println(question)

	query := append(header, question...)
	fmt.Println(query)
	fmt.Println(hex.EncodeToString(query))

	callDNSServer(query)
}
