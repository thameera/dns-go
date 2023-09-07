package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

var verbose bool

func debug(format string, v ...interface{}) {
	if verbose {
		fmt.Printf("DEBUG: "+format+"\n", v...)
	}
}

type inputData struct {
	Domain string
	Type   string
}

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

func typeToStr(recordType uint16) (string, bool) {
	dnsTypes := map[uint16]string{
		1:  "A",
		28: "AAAA",
		5:  "CNAME",
		16: "TXT",
	}
	str, found := dnsTypes[recordType]
	return str, found
}

func strToType(str string) (uint16, bool) {
	dnsTypes := map[string]uint16{
		"A":     1,
		"AAAA":  28,
		"CNAME": 5,
		"TXT":   16,
	}
	recordType, found := dnsTypes[str]
	return recordType, found
}

func createHeader() []byte {
	header := DNSHeader{
		ID:             uint16(rand.Intn(65536)),
		Flags:          1 << 8, // Recursion desired
		NumQuestions:   1,
		NumAnswers:     0,
		NumAuthorities: 0,
		NumAdditionals: 0,
	}
	debug("%#v", header)

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

func createQuestion(domain []byte, recordType string) []byte {
	typeStr, found := strToType(recordType)
	if !found {
		panic("Unsupported DNS type: "+recordType)
	}

	question := DNSQuestion{
		Name:  string(domain),
		Type:  typeStr,
		Class: 1, // CLASS_IN
	}
	debug("%#v", question)

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
	debug("Connected to DNS server")

	_, err = conn.Write(msg)
	if err != nil {
		fmt.Println("Error sending msg to DNS server", err)
		return nil, err
	}
	debug("DNS query sent")

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buffer := make([]byte, 512)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading response", err)
		return nil, err
	}
	debug("Read DNS response")
	debug("%v", buffer[:n])

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

	debug("Data len: %d", dataLen)

	typeStr, found := typeToStr(recordType)
	if !found {
		errMsg := fmt.Sprintf("Unsupported DNS type: %d", recordType)
		return record, errors.New(errMsg)
	}

	// Parse the data based on the DNS type
	if typeStr == "A" || typeStr == "AAAA" {
		// In this case, the data is an IP

		data := make([]byte, dataLen)
		err = binary.Read(reader, binary.BigEndian, &data)
		if err != nil {
			return record, err
		}
		record.Data = net.IP(data).String()

	} else if typeStr == "CNAME" {
		// In this case, the data is a domain

		data, err := decodeName(reader)
		if err != nil {
			return record, err
		}
		record.Data = string(data)

	} else if typeStr == "TXT" {
		// The data is just a string in this case

		data := make([]byte, dataLen)
		err = binary.Read(reader, binary.BigEndian, &data)
		if err != nil {
			return record, err
		}
		record.Data = string(data)

	}

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
	debug("%#v", decodedHeader)

	// Parse question
	// We assume only one question was sent - otherwise we need to loop this
	question, err := parseQuestion(reader)
	if err != nil {
		panic(err)
	}
	debug("%#v", question)

	// Parse answers
	for i := 0; i < int(decodedHeader.NumAnswers); i++ {
		record, err := parseRecord(reader)
		if err != nil {
			panic(err)
		}
		debug("%#v", record)
		fmt.Println("Answer:")
		fmt.Println(record)
	}

	// Parse authorities
	for i := 0; i < int(decodedHeader.NumAuthorities); i++ {
		record, err := parseRecord(reader)
		if err != nil {
			panic(err)
		}
		debug("%#v", record)
		fmt.Println("Authority:")
		fmt.Println(record)
	}

	// Parse additionals
	for i := 0; i < int(decodedHeader.NumAdditionals); i++ {
		record, err := parseRecord(reader)
		if err != nil {
			panic(err)
		}
		debug("%#v", record)
		fmt.Println("Additional:")
		fmt.Println(record)
	}
}

func showUsage() {
	fmt.Println("USAGE:")
	fmt.Println("\tgo run . [options...] <name> [type]")
	fmt.Println("where:")
	fmt.Println("\tname: \tname of the resource record to be looked up. Eg: the domain.")
	fmt.Println("\ttype: \tType of the query. Supported options: A, AAAA, CNAME. \n\t\tDefaults to A if not specified.")
	fmt.Println("Options:")
	fmt.Println("\t-v: \tVerbose logging")
}

func parseArgs() (inputData, error) {
	var data inputData

	if len(os.Args) < 2 {
		return data, errors.New("Insufficient arguments")
	}

	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.Parse()

	// Loop through each non-flag argument and identify them
	for _, arg := range flag.Args() {
		upperArg := strings.ToUpper(arg)

		if strings.Index(arg, ".") > 0 {
			if data.Domain != "" {
				return data, errors.New("Two names specified")
			}
			data.Domain = arg
		} else if upperArg == "A" || upperArg == "AAAA" || upperArg == "CNAME" || upperArg == "TXT" {
			if data.Type != "" {
				return data, errors.New("Type duplicated")
			}
			data.Type = strings.ToUpper(arg)
		} else {
			return data, errors.New("Invalid argument")
		}
	}

	// If no type specified, default to A record
	if data.Type == "" {
		data.Type = "A"
	}

	return data, nil
}

func main() {
	input, err := parseArgs()
	if err != nil {
		fmt.Println("Error: ", err)
		showUsage()
		os.Exit(1)
	}
	debug("%#v", input)

	rand.Seed(time.Now().UnixNano())

	header := createHeader()
	encDomain, err := encodeDomain(input.Domain)
	if err != nil {
		panic(err)
	}
	question := createQuestion(encDomain, input.Type)

	query := append(header, question...)

	res, err := callDNSServer(query)
	if err != nil {
		panic("Failed to call DNS server")
	}

	processResponse(res)
}
