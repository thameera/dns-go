package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

type DNSHeader struct {
	ID             uint16
	Flags          uint16
	NumQuestions   uint16
	NumAnswers     uint16
	NumAuthorities uint16
	NumAdditionals uint16
}

type DNSQuestion struct {
	Name  string
	Type  uint16
	Class uint16
}

type DNSRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   int32
	Data  string
}

func createHeader() []byte {
	header := DNSHeader{
		ID:             uint16(rand.Intn(65536)),
		Flags:          1<<8, // Recursion desired
		NumQuestions:   1,
		NumAnswers:     0,
		NumAuthorities: 0,
		NumAdditionals: 0,
	}

	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, header); err != nil {
		// TODO return err
		panic(err)
	}

	return buf.Bytes()
}

func encodeDomain(domain string) ([]byte, error) {
	buf := new(bytes.Buffer)

	for _, part := range strings.Split(domain, ".") {
		// Length of the part
		if err := binary.Write(buf, binary.BigEndian, byte(len(part))); err != nil {
			return nil, err
		}

		// The part itself
		if _, err := buf.WriteString(part); err != nil {
			return nil, err
		}
	}

	// Add trailing zero
	if err := binary.Write(buf, binary.BigEndian, byte(0)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func createQuestion(domain []byte) []byte {
	question := DNSQuestion{
		Name:  string(domain),
		Type:  1, // TYPE_A
		Class: 1, // CLASS_IN
	}

	buf := new(bytes.Buffer)

	// Write the domain separately, because binary.Write can't handle 'string' type
	_, err := buf.WriteString(question.Name)
	if err != nil {
		// TODO return err
		panic(err)
	}
	if err := binary.Write(buf, binary.BigEndian, question.Type); err != nil {
		panic(err)
	}
	if err := binary.Write(buf, binary.BigEndian, question.Class); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

func callDNSServer(msg []byte) ([]byte, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		fmt.Println("Error connecting to DNS server", err)
		return nil, err
	}
	defer conn.Close()

	_, err = conn.Write(msg)
	if err != nil {
		fmt.Println("Error sending msg to DNS server", err)
		return nil, err
	}

	fmt.Println("DNS query sent")

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buffer := make([]byte, 512)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading response", err)
		return nil, err
	}

	// fmt.Println(buffer)
	// fmt.Println(n)

	return buffer[:n], nil
}

func decodeName(reader *bytes.Reader) (string, error) {
	var parts []string

	for {
		length, err := reader.ReadByte()
		if err != nil {
			return "", err
		}

		if length == 0 {
			break
		}

		if length&0xc0 == 0xc0 { // 0xc0 = 11000000
			// Pointer
			result, err := decodeCompressedName(reader, length)
			if err != nil {
				return "", err
			}
			parts = append(parts, result)
			break
		} else {
			// Normal
			part := make([]byte, length)
			_, err := reader.Read(part)
			if err != nil {
				return "", err
			}
			parts = append(parts, string(part))
		}
	}

	return strings.Join(parts, "."), nil
}

func decodeCompressedName(reader *bytes.Reader, length byte) (string, error) {
	pointerByte := []byte{length & 0x3f} // 0x3f = 00111111
	nextByte := make([]byte, 1)
	_, err := reader.Read(nextByte)
	if err != nil {
		return "", err
	}
	pointerByte = append(pointerByte, nextByte[0])

	var pointer uint16
	err = binary.Read(bytes.NewReader(pointerByte), binary.BigEndian, &pointer)
	if err != nil {
		return "", err
	}

	currentPos, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	_, err = reader.Seek(int64(pointer), io.SeekStart)
	if err != nil {
		return "", err
	}

	result, err := decodeName(reader)
	if err != nil {
		return "", err
	}

	_, err = reader.Seek(currentPos, io.SeekStart)
	if err != nil {
		return "", err
	}

	return result, nil
}

func parseQuestion(reader *bytes.Reader) (DNSQuestion, error) {
	var question DNSQuestion

	name, err := decodeName(reader)
	if err != nil {
		return question, err
	}
	question.Name = name

	var recordType uint16
	err = binary.Read(reader, binary.BigEndian, &recordType)
	if err != nil {
		return question, err
	}
	question.Type = recordType

	var class uint16
	err = binary.Read(reader, binary.BigEndian, &class)
	if err != nil {
		return question, err
	}
	question.Class = class

	return question, nil
}

func parseRecord(reader *bytes.Reader) (DNSRecord, error) {
	var record DNSRecord

	name, err := decodeName(reader)
	if err != nil {
		return record, err
	}
	record.Name = name

	var recordType, class, dataLen uint16
	var ttl int32
	err = binary.Read(reader, binary.BigEndian, &recordType)
	if err != nil {
		return record, err
	}
	record.Type = recordType

	err = binary.Read(reader, binary.BigEndian, &class)
	if err != nil {
		return record, err
	}
	record.Class = class
	err = binary.Read(reader, binary.BigEndian, &ttl)
	if err != nil {
		return record, err
	}
	record.TTL = ttl
	err = binary.Read(reader, binary.BigEndian, &dataLen)
	if err != nil {
		return record, err
	}

	data := make([]byte, dataLen)
	fmt.Println("Data len", dataLen)
	err = binary.Read(reader, binary.BigEndian, &data)
	fmt.Println(data)
	if err != nil {
		return record, err
	}
	// TODO: Handle when data is not an IP
	record.Data = net.IP(data).String()

	return record, nil
}

func processResponse(res []byte) {
	reader := bytes.NewReader(res)

	// Parse header
	decodedHeader := DNSHeader{}
	err := binary.Read(reader, binary.BigEndian, &decodedHeader)
	if err != nil {
		panic(err)
	}

	fmt.Println(decodedHeader)

	// Parse question
	// We assume only one question was sent - otherwise we need to loop this
	question, err := parseQuestion(reader)
	if err != nil {
		panic(err)
	}
	fmt.Println(question)

	// Parse answers
	for i := 0; i < int(decodedHeader.NumAnswers); i++ {
		record, err := parseRecord(reader)
		if err != nil {
			panic(err)
		}
		fmt.Println("Answer:")
		fmt.Println(record)
	}

	// Parse authorities
	for i := 0; i < int(decodedHeader.NumAuthorities); i++ {
		record, err := parseRecord(reader)
		if err != nil {
			panic(err)
		}
		fmt.Println("Authority:")
		fmt.Println(record)
	}

	// Parse additionals
	for i := 0; i < int(decodedHeader.NumAdditionals); i++ {
		record, err := parseRecord(reader)
		if err != nil {
			panic(err)
		}
		fmt.Println("Additional:")
		fmt.Println(record)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage:")
		fmt.Println("\tgo run . DOMAIN")
		os.Exit(1)
	}

	domain := os.Args[1]

	rand.Seed(time.Now().UnixNano())

	header := createHeader()
	encDomain, err := encodeDomain(domain)
	if err != nil {
		panic(err)
	}
	question := createQuestion(encDomain)

	// fmt.Println(header)
	// fmt.Println(encDomain)
	// fmt.Println(question)

	query := append(header, question...)
	// fmt.Println(query)
	// fmt.Println(hex.EncodeToString(query))

	res, err := callDNSServer(query)
	if err != nil {
		panic("Failed to call DNS server")
	}

	fmt.Println(res)

	// readResponseHeader(res[:12])
	processResponse(res)
}
